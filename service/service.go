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

package service

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/binkynet/LocalWorker/service/bridge"
	"github.com/binkynet/LocalWorker/service/mqtt"
)

type Service interface {
	// Run the worker until the given context is cancelled.
	Run(ctx context.Context) error
}

type ServiceDependencies struct {
	Log      zerolog.Logger
	MqttConn mqtt.API
	Bridge   bridge.API
}

type service struct {
	ServiceDependencies
}

// NewService creates a Service instance and returns it.
func NewService(deps ServiceDependencies) (Service, error) {
	return &service{
		ServiceDependencies: deps,
	}, nil
}

// Run the worker until the given context is cancelled.
func (s *service) Run(ctx context.Context) error {
	// Fetch local slave configuration
	s.Bridge.BlinkGreenLED(time.Millisecond * 250)
	s.Bridge.SetRedLED(true)

	// TODO fetch
	//	time.Sleep(time.Second * 5)

	// Initialize local slaves
	s.Bridge.BlinkGreenLED(time.Millisecond * 100)
	s.Bridge.SetRedLED(true)

	// Open bus
	bus, err := s.Bridge.I2CBus()
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to open I2CBus")
	} else {
		// Detect local slaves
		s.Log.Info().Msg("Detecting local slaves")
		addrs := bus.DetectSlaveAddresses()
		s.Log.Info().Msgf("Detected %d local slaves: %v", len(addrs), addrs)

		// TODO initialize
		s.Log.Info().Msg("Running test on address 0x20")
		bridge.TestI2CBus(bus)
	}

	// Initialization done, run loop
	s.Bridge.SetGreenLED(true)
	s.Bridge.SetRedLED(false)

	// TODO run loop

	<-ctx.Done()
	s.Bridge.SetGreenLED(false)
	s.Bridge.SetRedLED(true)
	return nil
}
