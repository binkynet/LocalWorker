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

	"github.com/pkg/errors"
	terminate "github.com/pulcy/go-terminate"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"

	api "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/BinkyNet/netlog"

	"github.com/binkynet/LocalWorker/pkg/environment"
	"github.com/binkynet/LocalWorker/service"
	"github.com/binkynet/LocalWorker/service/bridge"
)

const (
	projectName          = "BinkyNet Local Worker"
	staticProjectVersion = "1.4.0"
	defaultGrpcPort      = 7129
)

var (
	projectVersion = "dev"
	projectBuild   = "dev"
	maskAny        = errors.WithStack
)

func main() {
	var levelFlag string
	var serverHost string
	var grpcPort int
	var bridgeType string

	logWriter, err := netlog.NewLogger()
	if err != nil {
		Exitf("Failed to create log writer: %s", err)
	}
	logOutput := zerolog.MultiLevelWriter(
		zerolog.ConsoleWriter{Out: os.Stderr},
		logWriter,
	)
	logger := zerolog.New(logOutput).With().Timestamp().Logger()
	defaultBridgeType := environment.AutoDetectBridgeType(logger)

	pflag.StringVarP(&levelFlag, "level", "l", "debug", "Set log level")
	pflag.StringVarP(&bridgeType, "bridge", "b", defaultBridgeType, "Type of bridge to use (rpi|opz|stub)")
	pflag.StringVar(&serverHost, "host", "0.0.0.0", "Host address the GRPC server will listen on")
	pflag.IntVar(&grpcPort, "port", defaultGrpcPort, "Port the GRPC server will listen on")
	pflag.Parse()

	var br bridge.API
	switch bridgeType {
	case "rpi":
		br, err = bridge.NewRaspberryPiBridge()
		if err != nil {
			Exitf("Failed to initialize Raspberry Pi Bridge: %v\n", err)
		}
	case "opz":
		br, err = bridge.NewOrangePIZeroBridge()
		if err != nil {
			Exitf("Failed to initialize Orange Pi Zero Bridge: %v\n", err)
		}
	case "stub":
		br = bridge.NewStub()
	default:
		Exitf("Unknown bridge type '%s' (rpi|opz|stub)\n", bridgeType)
	}
	logger.Debug().Str("bridge-type", bridgeType).Msg("Created bridge")

	version := projectVersion
	if version == "dev" || version == "" {
		version = staticProjectVersion
	}
	svc, err := service.NewService(service.Config{
		ProgramVersion: version,
	}, service.Dependencies{
		Log:    logger,
		Bridge: br,
	})
	if err != nil {
		Exitf("Failed to initialize Service: %v\n", err)
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
	g.Go(func() error { return svc.Run(ctx) })
	if err := g.Wait(); err != nil {
		Exitf("Service run failed: %#v", err)
	}
}

// Print the given error message and exit with code 1
func Exitf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}
