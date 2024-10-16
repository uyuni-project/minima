package storage

import (
	"crypto"
	"errors"
	"io"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/uyuni-project/minima/util"
)

// S3Storage allows to store data in an Amazon S3 bucket
type S3Storage struct {
	region string
	bucket string
	prefix string
	svc    *s3.S3
}

// NewS3Storage returns a new Storage backed by an S3 bucket
func NewS3Storage(accessKeyID string, secretAccessKey string, region string, bucket string) (storage Storage, err error) {
	creds := credentials.NewStaticCredentials(accessKeyID, secretAccessKey, "")
	config := aws.NewConfig().WithRegion(region).WithCredentials(creds)
	svc := s3.New(session.New(), config)

	err = configureBucket(region, bucket, svc)
	if err != nil {
		return
	}

	prefix, err := getCurrentPrefix(region, bucket, svc)
	if err != nil {
		return
	}

	err = configureWebsite(region, bucket, prefix, svc)
	if err != nil {
		return
	}

	storage = &S3Storage{region, bucket, prefix, svc}
	return
}

func configureBucket(region string, bucket string, svc *s3.S3) error {
	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
		CreateBucketConfiguration: &s3.CreateBucketConfiguration{
			LocationConstraint: aws.String(region),
		},
	}
	// HACK: https://docs.aws.amazon.com/AmazonS3/latest/API/RESTBucketPUT.html#RESTBucketPUT-requests-request-elements
	if region == "us-east-1" {
		input.CreateBucketConfiguration = nil
	}
	_, err := svc.CreateBucket(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists:
				return errors.New("bucket name already taken by another AWS user, please use a different name")
			case s3.ErrCodeBucketAlreadyOwnedByYou:
				return nil
			default:
				return err
			}
		} else {
			return err
		}
	}
	log.Printf("Bucket %s created\n", bucket)
	return nil
}

func getCurrentPrefix(region string, bucket string, svc *s3.S3) (result string, err error) {
	website, err := svc.GetBucketWebsite(&s3.GetBucketWebsiteInput{Bucket: aws.String(bucket)})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NoSuchWebsiteConfiguration":
				return "", nil
			}
		}
		return
	}

	if len(website.RoutingRules) != 1 {
		return
	}

	condition := website.RoutingRules[0].Condition
	if condition == nil {
		return
	}

	prefix := condition.KeyPrefixEquals
	if prefix == nil {
		return
	}

	result = *prefix
	return
}

func configureWebsite(region string, bucket string, prefix string, svc *s3.S3) (err error) {
	input := &s3.PutBucketWebsiteInput{
		Bucket: aws.String(bucket),
		WebsiteConfiguration: &s3.WebsiteConfiguration{
			IndexDocument: &s3.IndexDocument{
				Suffix: aws.String("index.html"),
			},
			RoutingRules: []*s3.RoutingRule{
				{
					Condition: &s3.Condition{
						KeyPrefixEquals: aws.String(prefix),
					},
					Redirect: &s3.Redirect{
						ReplaceKeyPrefixWith: aws.String(""),
					},
				},
			},
		},
	}
	_, err = svc.PutBucketWebsite(input)
	if err != nil {
		return
	}
	return
}

func (s *S3Storage) newPrefix() string {
	if s.prefix == "a/" {
		return "b/"
	}
	return "a/"
}

// NewReader returns a Reader for a file in a location, returns ErrFileNotFound
// if the requested path was not found at all
func (s *S3Storage) NewReader(filename string, location Location) (reader io.ReadCloser, err error) {
	var prefix string
	if location == Permanent {
		prefix = s.prefix
	} else {
		prefix = s.newPrefix()
	}
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(prefix + filename),
	}

	info, err := s.svc.GetObject(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NotFound":
				err = ErrFileNotFound
			}
		}
		return
	}

	return info.Body, err
}

// StoringMapper returns a mapper that will store read data to a temporary location specified by filename
func (s *S3Storage) StoringMapper(filename string, checksum string, hash crypto.Hash) (mapper util.ReaderMapper) {
	return func(reader io.ReadCloser) (result io.ReadCloser, err error) {
		uploader := s3manager.NewUploaderWithClient(s.svc)

		pipeReader, pipeWriter := io.Pipe()

		errs := make(chan error)
		go func() {
			_, err := uploader.Upload(&s3manager.UploadInput{
				Bucket: aws.String(s.bucket),
				Key:    aws.String(s.newPrefix() + filename),
				Body:   pipeReader,
			})
			errs <- err
		}()

		result = util.NewTeeReadCloser(reader, &waitingCloser{pipeWriter, errs, filename})
		return
	}
}

type waitingCloser struct {
	io.WriteCloser
	errs     chan error
	filename string
}

func (w *waitingCloser) Close() error {
	err := w.WriteCloser.Close()
	if err != nil {
		return err
	}
	err = <-w.errs
	return err
}

// Recycle will copy a file from the permanent to the temporary location
func (s *S3Storage) Recycle(filename string) (err error) {
	input := &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		CopySource: aws.String(s.bucket + "/" + s.prefix + filename),
		Key:        aws.String(s.newPrefix() + filename),
	}

	_, err = s.svc.CopyObject(input)
	return
}

// Commit moves any temporary file accumulated so far to the permanent location
func (s *S3Storage) Commit() (err error) {
	newPrefix := s.newPrefix()
	err = configureWebsite(s.region, s.bucket, newPrefix, s.svc)
	if err != nil {
		return
	}

	batcher := s3manager.NewBatchDeleteWithClient(s.svc)
	objectsToDelete := true
	for objectsToDelete {
		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(s.bucket),
			Prefix: aws.String(s.prefix),
		}

		objects, err := s.svc.ListObjectsV2(input)
		if err != nil {
			return err
		}

		if len(objects.Contents) > 0 {
			toDelete := []s3manager.BatchDeleteObject{}
			for _, o := range objects.Contents {
				toDelete = append(toDelete, s3manager.BatchDeleteObject{Object: &s3.DeleteObjectInput{
					Key:    o.Key,
					Bucket: aws.String(s.bucket),
				}})
			}

			err := batcher.Delete(nil, &s3manager.DeleteObjectsIterator{
				Objects: toDelete,
			})
			if err != nil {
				return err
			}
		} else {
			objectsToDelete = false
		}
	}

	return
}
