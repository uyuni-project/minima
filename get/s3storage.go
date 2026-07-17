package get

import (
	"context"
	"crypto"
	"errors"
	"io"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
	"github.com/uyuni-project/minima/util"
)

// S3Storage allows to store data in an Amazon S3 bucket
type S3Storage struct {
	region string
	bucket string
	prefix string
	client *s3.Client
}

// NewS3Storage returns a new Storage backed by an S3 bucket
func NewS3Storage(accessKeyID string, secretAccessKey string, region string, bucket string) (storage Storage, err error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return
	}
	client := s3.NewFromConfig(cfg)

	err = configureBucket(region, bucket, client)
	if err != nil {
		return
	}

	prefix, err := getCurrentPrefix(bucket, client)
	if err != nil {
		return
	}

	err = configureWebsite(bucket, prefix, client)
	if err != nil {
		return
	}

	storage = &S3Storage{region, bucket, prefix, client}
	return
}

func configureBucket(region string, bucket string, client *s3.Client) error {
	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(region),
		},
	}
	// HACK: https://docs.aws.amazon.com/AmazonS3/latest/API/RESTBucketPUT.html#RESTBucketPUT-requests-request-elements
	if region == "us-east-1" {
		input.CreateBucketConfiguration = nil
	}
	_, err := client.CreateBucket(context.Background(), input)
	if err != nil {
		var bucketExists *types.BucketAlreadyExists
		var bucketOwnedByYou *types.BucketAlreadyOwnedByYou
		if errors.As(err, &bucketExists) {
			return errors.New("Bucket name already taken by another AWS user, please use a different name")
		}
		if errors.As(err, &bucketOwnedByYou) {
			return nil
		}
		return err
	}
	log.Printf("Bucket %s created\n", bucket)
	return nil
}

func getCurrentPrefix(bucket string, client *s3.Client) (result string, err error) {
	website, err := client.GetBucketWebsite(context.Background(), &s3.GetBucketWebsiteInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchWebsiteConfiguration" {
			return "", nil
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

func configureWebsite(bucket string, prefix string, client *s3.Client) (err error) {
	input := &s3.PutBucketWebsiteInput{
		Bucket: aws.String(bucket),
		WebsiteConfiguration: &types.WebsiteConfiguration{
			IndexDocument: &types.IndexDocument{
				Suffix: aws.String("index.html"),
			},
			RoutingRules: []types.RoutingRule{
				{
					Condition: &types.Condition{
						KeyPrefixEquals: aws.String(prefix),
					},
					Redirect: &types.Redirect{
						ReplaceKeyPrefixWith: aws.String(""),
					},
				},
			},
		},
	}
	_, err = client.PutBucketWebsite(context.Background(), input)
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

	info, err := s.client.GetObject(context.Background(), input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchKey" {
			err = ErrFileNotFound
		}
		return
	}

	return info.Body, err
}

// StoringMapper returns a mapper that will store read data to a temporary location specified by filename
func (s *S3Storage) StoringMapper(filename string, checksum string, hash crypto.Hash) (mapper util.ReaderMapper) {
	return func(reader io.ReadCloser) (result io.ReadCloser, err error) {
		pipeReader, pipeWriter := io.Pipe()

		errs := make(chan error)
		go func() {
			_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
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
	_, err = s.client.CopyObject(context.Background(), &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		CopySource: aws.String(s.bucket + "/" + s.prefix + filename),
		Key:        aws.String(s.newPrefix() + filename),
	})
	return
}

// Commit moves any temporary file accumulated so far to the permanent location
func (s *S3Storage) Commit() (err error) {
	newPrefix := s.newPrefix()
	err = configureWebsite(s.bucket, newPrefix, s.client)
	if err != nil {
		return
	}

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(s.prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return err
		}
		if len(page.Contents) == 0 {
			break
		}

		ids := make([]types.ObjectIdentifier, len(page.Contents))
		for i, o := range page.Contents {
			ids[i] = types.ObjectIdentifier{Key: o.Key}
		}
		_, err = s.client.DeleteObjects(context.Background(), &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &types.Delete{Objects: ids},
		})
		if err != nil {
			return err
		}
	}

	return
}
