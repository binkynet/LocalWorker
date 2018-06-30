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
	"math/rand"
	"sync"
	"time"

	discoveryAPI "github.com/binkynet/BinkyNet/discovery"
	"github.com/pkg/errors"
	restkit "github.com/pulcy/rest-kit"
	"github.com/rs/zerolog"

	"github.com/binkynet/LocalWorker/pkg/netmanager"
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
	MqttBuilder func(env discoveryAPI.WorkerEnvironment, clientID string) (mqtt.Service, error)
}

type service struct {
	Config
	Dependencies

	mutex              sync.Mutex
	hostID             string
	registrationCancel func()
	workerCancel       func()
	mqttService        mqtt.Service
	netManagerClient   *netmanager.Client
	topicPrefix        string
}

// NewService creates a Service instance and returns it.
func NewService(conf Config, deps Dependencies) (Service, error) {
	deps.Log = deps.Log.With().Str("component", "service").Logger()
	// Create host ID
	hostID, err := createHostID()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create host ID")
	}
	return &service{
		Config:       conf,
		Dependencies: deps,
		hostID:       hostID,
	}, nil
}

// Run initialize the local worker and then continues
// to register the worker, followed by running the worker loop
// in a given environment.
func (s *service) Run(ctx context.Context) error {
	defer s.Bridge.Close()

	// Create host ID
	s.Log.Info().Str("id", s.hostID).Msg("Found host ID")

	// Fetch local slave configuration
	s.Bridge.BlinkGreenLED(time.Millisecond * 250)
	s.Bridge.BlinkRedLED(time.Millisecond * 250)

	// Open bus
	var err error
	/*bus, err := s.Bridge.I2CBus()
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to open I2CBus")
	} else {
		// Detect local slaves
		/*s.Log.Info().Msg("Detecting local slaves")
		addrs := bus.DetectSlaveAddresses()
		s.Log.Info().Msgf("Detected %d local slaves: %v", len(addrs), addrs)
	}*/

	for {
		// Register worker
		s.Bridge.BlinkGreenLED(time.Millisecond * 250)
		s.Bridge.SetRedLED(false)

		s.Log.Debug().Msg("registering worker...")
		registrationCtx, registrationCancel := context.WithCancel(ctx)
		s.mutex.Lock()
		s.registrationCancel = registrationCancel
		s.mutex.Unlock()
		err = s.registerWorker(registrationCtx, s.hostID, s.DiscoveryPort, s.ServerPort, s.ServerSecure)
		registrationCancel()
		if err != nil {
			s.Log.Error().Err(err).Msg("registerWorker failed")
			return maskAny(err)
		}

		s.mutex.Lock()
		mqttService := s.mqttService
		netManagerClient := s.netManagerClient
		topicPrefix := s.topicPrefix
		s.mqttService = nil
		s.netManagerClient = nil
		s.topicPrefix = ""
		s.mutex.Unlock()
		if mqttService == nil {
			return maskAny(fmt.Errorf("MQTT service has not been created"))
		}
		if netManagerClient == nil {
			return maskAny(fmt.Errorf("NetManager client has not been created"))
		}
		s.Log.Debug().Msg("worker registration completed")

		// Initialization done, run loop
		workerCtx, workerCancel := context.WithCancel(ctx)
		s.mutex.Lock()
		s.workerCancel = workerCancel
		s.mutex.Unlock()
		err = s.runWorkerInEnvironment(workerCtx, netManagerClient, mqttService, topicPrefix)
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
func (s *service) runWorkerInEnvironment(ctx context.Context, netManagerClient *netmanager.Client,
	mqttService mqtt.Service, topicPrefix string) error {
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
		conf, err := netManagerClient.GetWorkerConfig(ctx, s.hostID)
		if err != nil {
			s.Log.Error().Err(err).Msg("Failed to request configuration")
			// Wait a bit and then retry
		} else {
			s.Log.Debug().Interface("config", conf).Msg("received worker config")
			// Create a new worker using given config
			moduleID := s.hostID
			if conf.Alias != "" {
				moduleID = conf.Alias
			}
			w, err := worker.NewService(worker.Config{
				LocalWorkerConfig: conf,
				TopicPrefix:       topicPrefix,
				ModuleID:          moduleID,
			}, worker.Dependencies{
				Log:         s.Log.With().Str("component", "worker").Logger(),
				Bridge:      s.Bridge,
				MQTTService: mqttService,
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
	log := s.Log.With().
		Str("endpoint", input.Manager.Endpoint).
		Str("mqtt-host", input.Mqtt.Host).
		Int("mqtt-port", input.Mqtt.Port).
		Logger()
	log.Info().Msg("Environment called")

	s.mutex.Lock()
	defer s.mutex.Unlock()

	log.Debug().Msg("creating network manager client")
	netManagerClient, err := netmanager.NewClient(input.Manager.Endpoint)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create MQTT service")
		return maskAny(restkit.InternalServerError(err.Error(), 0))
	}

	log.Debug().Msg("creating random MQTT client ID")
	buf := make([]byte, 8)
	rand.Read(buf)
	clientID := fmt.Sprintf("%s_%x", s.hostID, buf)
	log.Debug().Str("client-id", clientID).Msg("creating MQTT service")
	mqttSvc, err := s.MqttBuilder(input, clientID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create MQTT service")
		return maskAny(restkit.InternalServerError(err.Error(), 0))
	}
	if s.mqttService != nil {
		s.mqttService.Close()
	}
	s.netManagerClient = netManagerClient
	s.mqttService = mqttSvc
	s.topicPrefix = input.Mqtt.TopicPrefix

	if cancel := s.registrationCancel; cancel != nil {
		log.Debug().Msg("Canceling registration")
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
