PROJECT := LocalWorker
ROOTDIR := $(shell pwd)
VERSION := $(shell cat VERSION)
COMMIT := $(shell git rev-parse --short HEAD)

ORGPATH := github.com/binkynet
REPONAME := $(PROJECT)
REPOPATH := $(ORGPATH)/$(REPONAME)
BINNAME := bnLocalWorker

SOURCES := $(shell find . -name '*.go')
BINARIES := ./bin/linux/arm/$(BINNAME) ./bin/linux/amd64/$(BINNAME) ./bin/darwin/amd64/$(BINNAME)

.PHONY: all clean bootstrap binaries test

all: binaries deployment

clean:
	rm -Rf $(ROOTDIR)/bin

bootstrap:
	go get github.com/mitchellh/gox
	docker build -t u-root-builder -f Dockerfile.u-root .
	docker build -t mkimage-builder -f Dockerfile.mkimage .

binaries: $(BINARIES)

$(BINARIES): $(SOURCES)
	CGO_ENABLED=0 gox \
		-osarch="linux/amd64 linux/arm darwin/amd64" \
		-ldflags="-X main.projectVersion=$(VERSION) -X main.projectBuild=$(COMMIT)" \
		-output="bin/{{.OS}}/{{.Arch}}/$(BINNAME)" \
		-tags="netgo" \
		./...

test:
	go test ./...

deployment: ./bin/uInitrd

./bin/linux/arm/uroot.cpio: ./bin/linux/arm/$(BINNAME)
	docker run -t --rm \
		-e GOARCH=arm \
		-v $(ROOTDIR):/project \
		-w /go/src/github.com/u-root/u-root \
		u-root-builder \
		u-root \
		-format=cpio -build=bb -o /project/bin/linux/arm/uroot.cpio \
		-files=/project/bin/linux/arm/$(BINNAME):bin/bnLocalWorker \
		-uinitcmd=/bin/bnLocalWorker \
		-defaultsh="" \
		./cmds/core/init

./bin/uInitrd: ./bin/linux/arm/uroot.cpio
	docker run -t --rm \
		-v $(ROOTDIR):/project \
		mkimage-builder \
		mkimage \
			-A arm \
			-O linux \
			-T ramdisk \
			-d /project/bin/linux/arm/uroot.cpio \
			/project/bin/uInitrd


.PHONY: update-modules
update-modules:
	rm -f go.mod go.sum 
	go mod init github.com/binkynet/LocalWorker
	go mod edit \
		-replace github.com/coreos/go-systemd=github.com/coreos/go-systemd@e64a0ec8b42a61e2a9801dc1d0abe539dea79197
	go get -u \
		github.com/binkynet/BinkyNet@v0.11.1
	go mod tidy

deploy:
	scp bin/uInitrd pi@192.168.77.1:/home/pi/tftp/
