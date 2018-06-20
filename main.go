//    Copyright 2017 Ewout Prangsma
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

	discoveryAPI "github.com/binkynet/BinkyNet/discovery"

	"github.com/binkynet/LocalWorker/service"
	"github.com/binkynet/LocalWorker/service/bridge"
	"github.com/binkynet/LocalWorker/service/mqtt"
	"github.com/binkynet/LocalWorker/service/server"
)

const (
	projectName       = "BinkyNet Local Worker"
	defaultServerPort = 7129
)

var (
	projectVersion = "dev"
	projectBuild   = "dev"
	maskAny        = errors.WithStack
)

func main() {
	var levelFlag string
	var serverHost string
	var serverPort int
	var discoveryPort int
	var bridgeType string

	pflag.StringVarP(&levelFlag, "level", "l", "debug", "Set log level")
	pflag.StringVarP(&bridgeType, "bridge", "b", "rpi", "Type of bridge to use (rpi|opz)")
	pflag.StringVar(&serverHost, "host", "0.0.0.0", "Host address the HTTP server will listen on")
	pflag.IntVar(&serverPort, "port", defaultServerPort, "Port the HTTP server will listen on")
	pflag.IntVar(&discoveryPort, "discovery-port", discoveryAPI.DefaultPort, "Port the NetManager discovery is listening on")
	pflag.Parse()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()

	var br bridge.API
	var err error
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
	default:
		Exitf("Unknown bridge type '%s' (rpi|opz)\n", bridgeType)
	}

	svc, err := service.NewService(service.Config{
		DiscoveryPort: discoveryPort,
		ServerPort:    serverPort,
		ServerSecure:  false,
	}, service.Dependencies{
		Log: logger,
		MqttBuilder: func(env discoveryAPI.WorkerEnvironment, clientID string) (mqtt.Service, error) {
			result, err := mqtt.NewService(mqtt.Config{
				Host:     env.Mqtt.Host,
				Port:     env.Mqtt.Port,
				UserName: env.Mqtt.UserName,
				Password: env.Mqtt.Password,
				ClientID: clientID,
			}, logger)
			if err != nil {
				return nil, maskAny(err)
			}
			return result, nil
		},
		Bridge: br,
	})
	if err != nil {
		Exitf("Failed to initialize Service: %v\n", err)
	}

	httpServer, err := server.NewServer(server.Config{
		Host: serverHost,
		Port: serverPort,
	}, svc, logger)
	if err != nil {
		Exitf("Failed to initialize Server: %v\n", err)
	}

	// Prepare to shutdown in a controlled manor
	ctx, cancel := context.WithCancel(context.Background())
	t := terminate.NewTerminator(func(template string, args ...interface{}) {
		logger.Info().Msgf(template, args...)
	}, cancel)
	go t.ListenSignals()

	fmt.Printf("Starting %s (version %s build %s)\n", projectName, projectVersion, projectBuild)
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return svc.Run(ctx) })
	g.Go(func() error { return httpServer.Run(ctx) })
	if err := g.Wait(); err != nil {
		Exitf("Service run failed: %#v", err)
	}
}

// Print the given error message and exit with code 1
func Exitf(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}
