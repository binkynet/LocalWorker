PROJECT := LocalWorker
ROOTDIR := $(shell pwd)
VERSION := $(shell cat VERSION)
COMMIT := $(shell git rev-parse --short HEAD)

GOBUILDDIR := $(ROOTDIR)/.gobuild
BINDIR := $(ROOTDIR)
VENDORDIR := $(ROOTDIR)/vendor

ORGPATH := github.com/binkynet
ORGDIR := $(GOBUILDDIR)/src/$(ORGPATH)
REPONAME := $(PROJECT)
REPODIR := $(ORGDIR)/$(REPONAME)
REPOPATH := $(ORGPATH)/$(REPONAME)
BINNAME := bnLocalWorker
BIN := $(BINDIR)/$(BINNAME)

GOPATH := $(GOBUILDDIR)
GOVERSION := 1.9.0-alpine

ifndef GOOS
	GOOS := linux
endif
ifndef GOARCH
	GOARCH := arm
endif

SOURCES := $(shell find . -name '*.go')

.PHONY: all clean deps

all: $(BIN)

clean:
	rm -Rf $(BIN) $(GOBUILDDIR)

deps:
	@${MAKE} -B -s .gobuild

local:
	@${MAKE} -B GOOS=$(shell go env GOHOSTOS) GOARCH=$(shell go env GOHOSTARCH) $(BIN)

.gobuild:
	@mkdir -p $(ORGDIR)
	@rm -f $(REPODIR) && ln -s ../../../../ $(REPODIR)
	@GOPATH=$(GOPATH) pulsar go flatten -V $(VENDORDIR)
	@GOPATH=$(GOPATH) pulsar go get $(ORGPATH)/BinkyNet/...

$(BIN): .gobuild $(SOURCES)
	docker run \
		--rm \
		-v $(ROOTDIR):/usr/code \
		-e GOPATH=/usr/code/.gobuild \
		-e GOOS=$(GOOS) \
		-e GOARCH=$(GOARCH) \
		-e CGO_ENABLED=0 \
		-w /usr/code/ \
		golang:$(GOVERSION) \
		go build -a -ldflags "-X main.projectVersion=$(VERSION) -X main.projectBuild=$(COMMIT)" -o $(BINNAME) $(REPOPATH)

test: $(BIN)
	go test ./...

