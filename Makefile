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

.PHONY: update-modules
update-modules:
	rm -f go.mod go.sum 
	go mod init github.com/binkynet/LocalWorker
	go mod edit \
		-replace github.com/coreos/go-systemd=github.com/coreos/go-systemd@e64a0ec8b42a61e2a9801dc1d0abe539dea79197
	go get -u \
		github.com/binkynet/BinkyNet@ad70e4c93fb40a8fd7ef6cbd3592b323dd9bec04
	go mod tidy
