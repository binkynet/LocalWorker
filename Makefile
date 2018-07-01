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
GOVERSION := 1.10.3-alpine
GOCACHEVOL := $(PROJECT)-gocache

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
	@pulsar go path -p $(REPOPATH)
	@GOPATH=$(GOPATH) pulsar go flatten -V $(VENDORDIR)
	@GOPATH=$(GOPATH) pulsar go get $(ORGPATH)/BinkyNet/...

.PHONY: $(GOCACHEVOL)
$(GOCACHEVOL):
	docker volume create $(GOCACHEVOL)

$(BIN): .gobuild $(SOURCES) $(GOCACHEVOL)
	docker run \
		--rm \
		-v $(ROOTDIR):/usr/code \
		-v $(GOCACHEVOL):/usr/cache \
		-e GOCACHE=/usr/cache \
		-e GOPATH=/usr/code/.gobuild \
		-e GOOS=$(GOOS) \
		-e GOARCH=$(GOARCH) \
		-e CGO_ENABLED=0 \
		-w /usr/code/ \
		golang:$(GOVERSION) \
		go build -ldflags "-X main.projectVersion=$(VERSION) -X main.projectBuild=$(COMMIT)" -o $(BINNAME) $(REPOPATH)

test: $(BIN)
	go test ./...

