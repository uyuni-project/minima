# minima
A Simple Linux Repository Manager.

Currently, it's a commandline utility to download RPM repos locally via http (mirroring).

Usage:
```
Downloads a repository given its URL

Usage:
  minima get [URL] [flags]

Flags:
  -a, --archs string       Comma-separated list of archs to include (default "all")
  -d, --directory string   Destination directory to save the repo (default ".")

Global Flags:
      --config string   config file (default is $HOME/.minima.yaml)
```


## How to contribute

 - set up a [Go workspace](https://golang.org/doc/code.html)
   - set the `GOPATH` environment variable (eg. in `~/.bashrc`)
   - set the `PATH` environment variable (eg. in `~/.bashrc`)
 - grab the project sources: `cd $GOPATH; go get github.com/moio/minima`
 - install development utilities: `go get -u github.com/spf13/cobra/cobra github.com/govend/govend`
