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
	"fmt"
	"sync"
	"time"

	discoveryAPI "github.com/binkynet/BinkyNet/discovery"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/binkynet/LocalWorker/service/bridge"
	"github.com/binkynet/LocalWorker/service/mqtt"
	"github.com/binkynet/LocalWorker/service/worker"
)

type Service interface {
	// Run the worker until the given context is cancelled.
	Run(ctx context.Context) error
	// Called to relay environment information.
	Environment(ctx context.Context, input discoveryAPI.WorkerEnvironment) error
	// Called to force a complete reload of the worker.
	Reload(ctx context.Context) error
}

type Config struct {
	DiscoveryPort int
	ServerPort    int
	ServerSecure  bool
}

type Dependencies struct {
	Log         zerolog.Logger
	Bridge      bridge.API
	MqttBuilder func(discoveryAPI.WorkerEnvironment) (mqtt.Service, error)
}

type service struct {
	Config
	Dependencies

	mutex              sync.Mutex
	registrationCancel func()
	workerCancel       func()
	mqttService        mqtt.Service
}

// NewService creates a Service instance and returns it.
func NewService(conf Config, deps Dependencies) (Service, error) {
	deps.Log = deps.Log.With().Str("component", "service").Logger()
	return &service{
		Config:       conf,
		Dependencies: deps,
	}, nil
}

// Run initialize the local worker and then continues
// to register the worker, followed by running the worker loop
// in a given environment.
func (s *service) Run(ctx context.Context) error {
	// Create host ID
	hostID, err := createHostID()
	if err != nil {
		return errors.Wrap(err, "Failed to create host ID")
	}
	s.Log.Info().Str("id", hostID).Msg("Found host ID")

	// Fetch local slave configuration
	s.Bridge.BlinkGreenLED(time.Millisecond * 250)
	s.Bridge.BlinkRedLED(time.Millisecond * 250)

	// Open bus
	bus, err := s.Bridge.I2CBus()
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to open I2CBus")
	} else {
		// Detect local slaves
		s.Log.Info().Msg("Detecting local slaves")
		addrs := bus.DetectSlaveAddresses()
		s.Log.Info().Msgf("Detected %d local slaves: %v", len(addrs), addrs)
	}

	for {
		// Register worker
		s.Bridge.BlinkGreenLED(time.Millisecond * 250)
		s.Bridge.SetRedLED(false)

		s.Log.Debug().Msg("registering worker...")
		registrationCtx, registrationCancel := context.WithCancel(ctx)
		s.mutex.Lock()
		s.registrationCancel = registrationCancel
		s.mutex.Unlock()
		err = s.registerWorker(registrationCtx, hostID, s.DiscoveryPort, s.ServerPort, s.ServerSecure)
		registrationCancel()
		if err != nil {
			s.Log.Error().Err(err).Msg("registerWorker failed")
			return maskAny(err)
		}

		s.mutex.Lock()
		mqttService := s.mqttService
		s.mqttService = nil
		s.mutex.Unlock()
		if mqttService == nil {
			return maskAny(fmt.Errorf("MQTT service has not been created"))
		}
		s.Log.Debug().Msg("worker registration completed")

		// Initialization done, run loop
		workerCtx, workerCancel := context.WithCancel(ctx)
		s.mutex.Lock()
		s.workerCancel = workerCancel
		s.mutex.Unlock()
		err = s.runWorkerInEnvironment(workerCtx, mqttService)
		workerCancel()
		if err != nil {
			s.Log.Error().Err(err).Msg("registerWorker failed")
			return maskAny(err)
		}

		// If context cancelled, return
		if ctx.Err() != nil {
			return nil
		}
	}
}

// Run the worker until the given context is cancelled.
func (s *service) runWorkerInEnvironment(ctx context.Context, mqttService mqtt.Service) error {
	defer mqttService.Close()

	// Initialization done, run loop
	s.Bridge.SetGreenLED(true)
	s.Bridge.SetRedLED(false)

	defer func() {
		s.Bridge.SetGreenLED(false)
		s.Bridge.SetRedLED(true)
	}()

	for {
		delay := time.Second

		// Request configuration
		conf, err := mqttService.RequestConfiguration(ctx)
		if err != nil {
			s.Log.Error().Err(err).Msg("Failed to request configuration")
			// Wait a bit and then retry
		} else {
			// Create a new worker using given config
			w, err := worker.NewService(conf, worker.Dependencies{
				Log:    s.Log.With().Str("component", "worker").Logger(),
				Bridge: s.Bridge,
			})
			if err != nil {
				s.Log.Error().Err(err).Msg("Failed to create worker")
				// Wait a bit and then retry
			} else {
				// Run worker
				if err := w.Run(ctx); err != nil {
					s.Log.Error().Err(err).Msg("Failed to run worker")
				} else {
					s.Log.Info().Err(err).Msg("Worker ended")
				}
			}
		}

		select {
		case <-time.After(delay):
			// Just continue
		case <-ctx.Done():
			// Context cancelled, stop.
			return nil
		}
	}
}

// Called to relay environment information.
func (s *service) Environment(ctx context.Context, input discoveryAPI.WorkerEnvironment) error {
	s.Log.Info().Msg("Environment called")

	s.mutex.Lock()
	defer s.mutex.Unlock()

	mqttSvc, err := s.MqttBuilder(input)
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to create MQTT service")
	} else {
		if s.mqttService != nil {
			s.mqttService.Close()
		}
		s.mqttService = mqttSvc
	}

	if cancel := s.registrationCancel; cancel != nil {
		cancel()
	}
	return nil
}

// Called to force a complete reload of the worker.
func (s *service) Reload(ctx context.Context) error {
	s.Log.Info().Msg("Reload called")

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if cancel := s.workerCancel; cancel != nil {
		cancel()
	}
	return nil
}
