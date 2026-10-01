[![Go Reference](https://pkg.go.dev/badge/github.com/manniwood/iidy.svg)](https://pkg.go.dev/github.com/manniwood/iidy)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

# IIDY - Is It Done Yet?

## Status: For Play Purposes Only

This code is not intended for production use. It is a fun way to explore
some ideas with Go and PostgreSQL. It is very permissively licenced, so
feel free to beg/borrow/steal anything that you like from here.

## Summary

IIDY-proto is a fork of my [iidy-protobuf](https://github.com/manniwood/iidy-protobuf) project
where I add `created_at`, `updated_at` and `deleted_at` fields to the database, to play with
certain date handling ideas that I don't want to forget.

## The Journey

### Install protobuf compiler and Go plugins

As per [protobuf's installation instructions](https://protobuf.dev/installation/):

```
$ cd ~/Downloads

# Fetch the download URL for the latest Linux x86_64 asset from GitHub API
$ URL=$(curl -s https://api.github.com/repos/protocolbuffers/protobuf/releases/latest \
  | jq -r '.assets[] | select(.name | endswith("linux-x86_64.zip")) | .browser_download_url')

$ curl -LO $URL

$ unzip protoc-36.0-linux-x86_64.zip -d $HOME/.local
```


As per [the Go gRPC quickstart](https://grpc.io/docs/languages/go/quickstart/):

Ensure your Go distro has the protobuf and grpc stuff:

```
$ go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
$ go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

Ensure your Go project's go.mod has protobuf and grpc packages:

```
$ go get google.golang.org/protobuf
$ go get google.golang.org/protobuf/reflect/protoreflect
$ go get google.golang.org/protobuf/runtime/protoimpl
$ go get google.golang.org/grpc
$ go get google.golang.org/grpc/codes
$ go get google.golang.org/grpc/status
```

### Make my proto definitions

See proto folder

### TODO: write the rest of this README
