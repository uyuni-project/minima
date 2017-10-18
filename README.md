![Minimal infinitely-growing pattern in Conway's Game of Life](doc/minimal.svg)


# minima
minima is a simple Linux repository manager.

Currently, the only implemented functionality is the smart downloading of RPM repos from an HTTP source for mirroring. Downloaded repos can be saved either in a local filesystem directory or an Amazon S3 bucket.

[![Travis CI build status](https://travis-ci.org/moio/minima.svg?branch=master)](https://travis-ci.org/moio/minima)

## Configuration

You can specify configuration in YAML either in a file (by default `minima.yaml`) or the `MINIMA_CONFIG` environment variable.


An example `minima.yaml` is below:

```yaml
# filesystem directory example
- url: http://download.opensuse.org/repositories/myrepo1/openSUSE_Leap_42.3/
  path: /tmp/minima/repo1

# AWS S3 bucket example
- url: http://download.opensuse.org/repositories/myrepo1/openSUSE_Leap_42.3/
  access_key_id: ACCESS_KEY_ID
  secret_access_key: SECRET_ACCESS_KEY
  region: us-east-1
  bucket: minima-bucket-key
  archs: [x86_64]
```

To sync repositories, use `minima sync`.

## How to contribute

 - set up a [Go workspace](https://golang.org/doc/code.html)
   - set the `GOPATH` environment variable (eg. in `~/.bashrc`)
   - set the `PATH` environment variable (eg. in `~/.bashrc`)
 - grab the project sources: `cd $GOPATH; go get github.com/moio/minima`
 - install development utilities: `go get -u github.com/spf13/cobra/cobra github.com/govend/govend`
