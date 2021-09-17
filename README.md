![Minimal infinitely-growing pattern in Conway's Game of Life](doc/minimal.svg)


# minima
minima is a simple Linux repository manager.

Currently, the only implemented functionality is the smart downloading of RPM and simple DEB repos from an HTTP source for mirroring. Downloaded repos can be saved either in a local filesystem directory or an Amazon S3 bucket.

[![Travis CI build status](https://travis-ci.org/uyuni-project/minima.svg?branch=master)](https://travis-ci.org/uyuni-project/minima)

## Configuration

You can specify configuration in YAML either in a file (by default `minima.yaml`) or the `MINIMA_CONFIG` environment variable.

An example `minima.yaml` is below:
```yaml
storage:
  type: file
  path: /srv/mirror
  # uncomment to save to an AWS S3 bucket instead of the filesystem
  # type: s3
  # access_key_id: ACCESS_KEY_ID
  # secret_access_key: SECRET_ACCESS_KEY
  # region: us-east-1
  # bucket: minima-bucket-key
  #

http:
  - url: http://download.opensuse.org/repositories/myrepo1/openSUSE_Leap_42.3/
    archs: [x86_64]

# optional section to download repos from SCC
# scc:
#   username: UC7
#   password: INSERT_PASSWORD_HERE
#   repo_names:
#     - SLES12-SP2-LTSS-Updates
#   archs: [x86_64]

# OBS credentials:
# obs:
#    username: ""
#    password: ""
```



To sync repositories, use `minima sync`.

To search for new MU repositories, use `minima updates -s`.

To search and sync automatically all the new MU repositories:
use `minima updates`.

To cleanup obsolete repositories, use `--cleanup` option. It should be ran whether with 
sync or updates accordingly:
```
./minima sync --cleanup
```
or
```
./minima updates --cleanup
```
If a repository that is not specified in the minima.yaml config file will be identified,
you will be prompted to keep or delete it. If you want to automatically delete all the
repositories that are not specified in minima.yaml, run:
```
./minima sync --cleanup --auto-approve
```

## How to contribute

 - set up a [Go workspace](https://golang.org/doc/code.html)
   - set the `GOPATH` environment variable (eg. in `~/.bashrc`)
   - set the `PATH` environment variable (eg. in `~/.bashrc`)
 - grab the project sources: `cd $GOPATH; go get github.com/uyuni-project/minima`
 - install development utilities: `go get -u github.com/spf13/cobra/cobra github.com/govend/govend`
