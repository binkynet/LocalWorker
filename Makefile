PROJECT := LocalWorker
ROOTDIR := $(shell pwd)
VERSION := $(shell cat VERSION)
COMMIT := $(shell git rev-parse --short HEAD)

ORGPATH := github.com/binkynet
REPONAME := $(PROJECT)
REPOPATH := $(ORGPATH)/$(REPONAME)
BINNAME := bnLocalWorker

SOURCES := $(shell find . -name '*.go')

.PHONY: all clean bootstrap binaries test

all: binaries

clean:
	rm -Rf $(ROOTDIR)/bin

bootstrap:
	go get github.com/mitchellh/gox

binaries: $(SOURCES)
	CGO_ENABLED=0 gox \
		-osarch="linux/amd64 linux/arm darwin/amd64" \
		-ldflags="-X main.projectVersion=$(VERSION) -X main.projectBuild=$(COMMIT)" \
		-output="bin/{{.OS}}/{{.Arch}}/$(BINNAME)" \
		-tags="netgo" \
		./...

test:
	go test ./...

