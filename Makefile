PROJECT := LocalWorker
ROOTDIR := $(shell pwd)
VERSION := $(shell cat VERSION)
COMMIT := $(shell git rev-parse --short HEAD)

ORGPATH := github.com/binkynet
REPONAME := $(PROJECT)
REPOPATH := $(ORGPATH)/$(REPONAME)
BINNAME := bnLocalWorker

SOURCES := $(shell find . -name '*.go') go.mod go.sum
BINARIES := ./bin/linux/arm/$(BINNAME) ./bin/linux/amd64/$(BINNAME) ./bin/darwin/amd64/$(BINNAME)

.PHONY: all clean bootstrap binaries test

ALLARCHS := arm arm64

ALLARCHBINDIRS := $(addprefix ./bin/,$(ALLARCHS))
ALLINITRDS := $(addsuffix /uInitrd,$(ALLARCHBINDIRS))
ALLBOOTSCRIPTS := $(addsuffix /boot.scr.uimg,$(ALLARCHBINDIRS))
ALLDEPLOYMENTS := $(ALLINITRDS) $(ALLBOOTSCRIPTS)

all: binaries deployment

clean:
	rm -Rf $(ROOTDIR)/bin

bootstrap:
	go get github.com/mitchellh/gox
	docker build -t u-root-builder -f Dockerfile.u-root .
	docker build -t mkimage-builder -f Dockerfile.mkimage .

binaries: 
	CGO_ENABLED=0 gox \
		-osarch="linux/amd64 linux/arm linux/arm64 darwin/amd64 darwin/arm64" \
		-ldflags="-X main.projectVersion=$(VERSION) -X main.projectBuild=$(COMMIT)" \
		-output="bin/{{.OS}}/{{.Arch}}/$(BINNAME)" \
		-tags="netgo" \
		./...

test:
	go test ./...

deployment: $(ALLDEPLOYMENTS)

./bin/linux/%/uroot.cpio: ./bin/linux/%/$(BINNAME)
	@mkdir -p bin/linux/$*
	docker run -t --rm \
		-e GOARCH=$* \
		-v $(ROOTDIR):/project \
		-w /go/u-root \
		u-root-builder \
		u-root \
		-format=cpio -build=bb -o /project/bin/linux/$*/uroot.cpio \
		-files=/project/bin/linux/$*/$(BINNAME):bin/bnLocalWorker \
		-uinitcmd=/bin/bnLocalWorker \
		-defaultsh="" \
		./cmds/core/init

./bin/%/uInitrd: ./bin/linux/%/uroot.cpio
	@mkdir -p bin/$*
	docker run -t --rm \
		-v $(ROOTDIR):/project \
		mkimage-builder \
		mkimage \
			-A $* \
			-O linux \
			-T ramdisk \
			-d /project/bin/linux/$*/uroot.cpio \
			/project/bin/$*/uInitrd

./bin/%/boot.scr.uimg: ./boot/boot.cmd
	@mkdir -p bin/$*
	docker run -t --rm \
		-v $(ROOTDIR):/project \
		mkimage-builder \
		mkimage \
			-C none \
			-A $* \
			-T script \
			-d /project/boot/boot.cmd \
			/project/bin/$*/boot.scr.uimg

.PHONY: update-modules
update-modules:
	rm -f go.mod go.sum 
	go mod init github.com/binkynet/LocalWorker
	go mod edit \
		-replace github.com/coreos/go-systemd=github.com/coreos/go-systemd@e64a0ec8b42a61e2a9801dc1d0abe539dea79197
	go get -u \
		github.com/binkynet/BinkyNet@v1.12.0
	go mod tidy

deploy:
	scp bin/arm/uInitrd pi@192.168.77.1:/home/pi/tftp/
	scp bin/arm/boot.scr.uimg pi@192.168.77.1:/home/pi/tftp/

deploy-local:
	scp bin/linux/arm/bnLocalWorker pi@192.168.140.104:/home/pi/
