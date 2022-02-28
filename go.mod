module github.com/binkynet/LocalWorker

go 1.17

replace github.com/coreos/go-systemd => github.com/coreos/go-systemd v0.0.0-20190620071333-e64a0ec8b42a

require (
	github.com/binkynet/BinkyNet v0.14.5
	github.com/ecc1/gpio v0.0.0-20200212231225-d40e43fcf8f5
	github.com/ewoutp/go-aggregate-error v0.0.0-20141209171456-e0dbde632d55
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/mattn/go-pubsub v0.0.0-20160821075316-7a151c7747cd
	github.com/pkg/errors v0.9.1
	github.com/pulcy/go-terminate v0.0.0-20160630075856-d486fe7ee814
	github.com/rs/zerolog v1.18.0
	github.com/spf13/pflag v1.0.5
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/sys v0.0.0-20200930185726-fdedc70b468f
	google.golang.org/grpc v1.29.1
)

require (
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.3.4 // indirect
	github.com/grandcat/zeroconf v1.0.0 // indirect
	github.com/juju/errgo v0.0.0-20140925100237-08cceb5d0b53 // indirect
	github.com/miekg/dns v1.1.27 // indirect
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9 // indirect
	golang.org/x/net v0.0.0-20201021035429-f5854403a974 // indirect
	golang.org/x/text v0.3.3 // indirect
	google.golang.org/genproto v0.0.0-20200423170343-7949de9c1215 // indirect
)
