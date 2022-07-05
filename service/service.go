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
	"sync"
	"time"

	api "github.com/binkynet/BinkyNet/apis/v1"
	discovery "github.com/binkynet/BinkyNet/discovery"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/binkynet/LocalWorker/service/bridge"
	"github.com/binkynet/LocalWorker/service/objects"
)

type Service interface {
	api.LocalWorkerServiceServer

	// Run the worker until the given context is cancelled.
	Run(ctx context.Context) error
}

type Config struct {
	ProgramVersion string
}

type Dependencies struct {
	Logger     zerolog.Logger
	Bridge     bridge.API
	LokiLogger LokiLogger
}

type service struct {
	Config
	Dependencies

	mutex         sync.Mutex
	hostID        string
	workerCancel  func()
	shutdown      bool
	configChanged chan *api.LocalWorkerConfig
	startedAt     time.Time
	workerRunner  *workerRunner
	actuals       objects.ServiceActuals

	lokiListener      *discovery.ServiceListener
	lokiChanges       chan api.ServiceInfo
	timeOffsetChanges chan int64
}

// NewService creates a Service instance and returns it.
func NewService(conf Config, deps Dependencies) (Service, error) {
	deps.Logger = deps.Logger.With().Str("component", "service").Logger()
	// Create host ID
	hostID, err := createHostID()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create host ID")
	}
	deps.Logger = deps.Logger.With().Str("module-id", hostID).Logger()
	actuals := objects.ServiceActuals{
		OutputActuals: make(chan api.Output, 8),
		PowerActuals:  make(chan api.PowerState, 8),
		SensorActuals: make(chan api.Sensor, 8),
		SwitchActuals: make(chan api.Switch, 8),
	}
	s := &service{
		Config:            conf,
		Dependencies:      deps,
		hostID:            hostID,
		lokiChanges:       make(chan api.ServiceInfo),
		timeOffsetChanges: make(chan int64),
		configChanged:     make(chan *api.LocalWorkerConfig),
		startedAt:         time.Now(),
		workerRunner:      newWorkerRunner(hostID, deps.Bridge, actuals),
		actuals:           actuals,
	}
	s.lokiListener = discovery.NewServiceListener(deps.Logger, api.ServiceTypeLokiProvider, true, s.lokiChanged)
	return s, nil
}

// Run initialize the local worker and then continues
// to register the worker, followed by running the worker loop
// in a given environment.
func (s *service) Run(ctx context.Context) error {
	log := s.Logger.With().Str("host-id", s.hostID).Logger()
	defer s.Bridge.Close()

	// Create host ID
	log.Info().Msg("Found host ID")

	// Fetch local slave configuration
	s.Bridge.BlinkGreenLED(time.Millisecond * 250)
	s.Bridge.BlinkRedLED(time.Millisecond * 250)

	// Start discovery listeners
	go s.lokiListener.Run(ctx)
	go s.LokiLogger.Run(ctx, log, s.hostID, s.lokiChanges, s.timeOffsetChanges)

	return s.workerRunner.runWorkers(ctx, log, s.configChanged)
}

// Loki service has changed
func (s *service) lokiChanged(info api.ServiceInfo) {
	s.lokiChanges <- info
}
