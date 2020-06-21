module github.com/binkynet/LocalWorker

go 1.13

replace github.com/coreos/go-systemd => github.com/coreos/go-systemd v0.0.0-20190620071333-e64a0ec8b42a

require (
	github.com/binkynet/BinkyNet v0.0.0-20200621103600-2b482ddd05ee
	github.com/ecc1/gpio v0.0.0-20200212231225-d40e43fcf8f5
	github.com/ewoutp/go-aggregate-error v0.0.0-20141209171456-e0dbde632d55
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/juju/errgo v0.0.0-20140925100237-08cceb5d0b53
	github.com/julienschmidt/httprouter v1.3.0
	github.com/mattn/go-pubsub v0.0.0-20160821075316-7a151c7747cd
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.0 // indirect
	github.com/pulcy/go-terminate v0.0.0-20160630075856-d486fe7ee814
	github.com/pulcy/rest-kit v0.0.0-20170324092929-348dc5f0fbb6
	github.com/rs/zerolog v1.18.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20200615200032-f1bc736245b1
	google.golang.org/grpc v1.27.1
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
)
