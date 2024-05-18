//    Copyright 2018-2021 Ewout Prangsma
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	_ "net/http/pprof"

	terminate "github.com/pulcy/go-terminate"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"

	api "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/BinkyNet/sshlog"

	"github.com/binkynet/LocalWorker/pkg/environment"
	"github.com/binkynet/LocalWorker/pkg/server"
	"github.com/binkynet/LocalWorker/pkg/service"
	"github.com/binkynet/LocalWorker/pkg/service/bridge"
	"github.com/binkynet/LocalWorker/pkg/ui"
)

const (
	projectName          = "BinkyNet Local Worker"
	staticProjectVersion = "1.5.0"
	defaultGrpcPort      = 7129
	defaultHTTPPort      = 7130
	defaultSSHPort       = 7122
)

var (
	projectVersion = "dev"
	projectBuild   = "dev"
)

func main() {
	var levelFlag string
	var serverHost string
	var grpcPort int
	var bridgeType string
	var httpPort int
	var sshPort int

	lokiLogger := service.NewLokiLogger()
	sshLogger := sshlog.NewSshLogger()
	logOutput := zerolog.MultiLevelWriter(
		zerolog.ConsoleWriter{Out: os.Stderr},
		lokiLogger,
		sshLogger,
	)
	zerolog.TimeFieldFormat = time.RFC3339Nano
	logger := zerolog.New(logOutput).With().Timestamp().Logger()
	logger.Info().Msg("Started logger on console & netlog")
	defaultBridgeType := environment.AutoDetectBridgeType(logger)

	pflag.StringVarP(&levelFlag, "level", "l", "debug", "Set log level")
	pflag.StringVarP(&bridgeType, "bridge", "b", defaultBridgeType, "Type of bridge to use (rpi|opz|stub)")
	pflag.StringVar(&serverHost, "host", "0.0.0.0", "Host address the GRPC server will listen on")
	pflag.IntVar(&grpcPort, "port", defaultGrpcPort, "Port the GRPC server will listen on")
	pflag.IntVar(&httpPort, "http-port", defaultHTTPPort, "Port the HTTP server will listen on")
	pflag.IntVar(&sshPort, "ssh-port", defaultSSHPort, "Port the SSH server will listen on")
	pflag.Parse()

	var br bridge.API
	var err error
	switch bridgeType {
	case "rpi":
		br, err = bridge.NewRaspberryPiBridge()
		if err != nil {
			Exitf(logger, "Failed to initialize Raspberry Pi Bridge: %v\n", err)
		}
	case "opz":
		br, err = bridge.NewOrangePIZeroBridge()
		if err != nil {
			Exitf(logger, "Failed to initialize Orange Pi Zero Bridge: %v\n", err)
		}
	case "stub":
		br = bridge.NewStub()
	default:
		Exitf(logger, "Unknown bridge type '%s' (rpi|opz|stub)\n", bridgeType)
	}
	logger.Debug().Str("bridge-type", bridgeType).Msg("Created bridge")

	bus, err := br.I2CBus()
	if err != nil {
		Exitf(logger, "Failed to open I2C bus: %v\n", err)
	}
	addrs := bus.DetectSlaveAddresses()
	addrsStr := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		addrsStr = append(addrsStr, fmt.Sprintf("0x%x", addr))
	}
	logger.Debug().Strs("addresses", addrsStr).Msg("Detected I2C addresses")

	version := projectVersion
	if version == "dev" || version == "" {
		version = staticProjectVersion
	}
	svc, err := service.NewService(service.Config{
		ProgramVersion: version,
		MetricsPort:    httpPort,
		GRPCPort:       grpcPort,
		SSHPort:        sshPort,
	}, service.Dependencies{
		Logger:     logger,
		Bridge:     br,
		LokiLogger: lokiLogger,
	})
	if err != nil {
		Exitf(logger, "Failed to initialize Service: %v\n", err)
	}
	uiProv := ui.NewUIProvider()
	srv, err := server.New(server.Config{
		Host:     serverHost,
		HTTPPort: httpPort,
		SSHPort:  sshPort,
		GRPCPort: grpcPort,
	}, logger, sshLogger, uiProv, svc)
	if err != nil {
		Exitf(logger, "Failed to initialize Server: %v\n", err)
	}

	// Prepare to shutdown in a controlled manor
	ctx, cancel := context.WithCancel(context.Background())
	ctx = api.WithServiceInfoHost(ctx, serverHost)
	t := terminate.NewTerminator(func(template string, args ...interface{}) {
		logger.Info().Msgf(template, args...)
	}, cancel)
	go t.ListenSignals()

	fmt.Printf("Starting %s (version %s build %s)\n", projectName, projectVersion, projectBuild)
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { svc.Run(ctx); return nil })
	g.Go(func() error { return srv.Run(ctx) })
	if err := g.Wait(); err != nil {
		Exitf(logger, "Service run failed: %#v", err)
	}
	fmt.Printf("Exiting %s (version %s build %s)\n", projectName, projectVersion, projectBuild)
}

// Print the given error message and exit with code 1
func Exitf(log zerolog.Logger, message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message, args...)
	// Sleep before exit to allow logs to be send
	log.Warn().Msg("About to exit process")
	time.Sleep(time.Second * 5)
	// Do actual exit
	log.Warn().Msg("Exiting process")
	os.Exit(1)
}
