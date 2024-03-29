//    Copyright 2017-2022 Ewout Prangsma
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
	"os"
	"sync"
	"sync/atomic"
	"time"

	api "github.com/binkynet/BinkyNet/apis/v1"
	discovery "github.com/binkynet/BinkyNet/discovery"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/binkynet/LocalWorker/pkg/environment"
	"github.com/binkynet/LocalWorker/pkg/service/bridge"
	grpcutil "github.com/binkynet/LocalWorker/pkg/service/util"
)

type Service interface {
	// Run the worker until the given context is cancelled.
	Run(ctx context.Context) error
}

type Config struct {
	ProgramVersion string
	MetricsPort    int
}

type Dependencies struct {
	Logger     zerolog.Logger
	Bridge     bridge.API
	LokiLogger LokiLogger
}

type service struct {
	Config
	Dependencies

	mutex             sync.Mutex
	hostID            string
	workerCancel      func()
	shutdown          bool
	lastEnvironmentID uint32
	lastWorkerID      uint32
	environmentSem    *semaphore.Weighted
	workerSem         *semaphore.Weighted
	startedAt         time.Time

	nwControlListener *discovery.ServiceListener
	lokiListener      *discovery.ServiceListener
	nwControlChanges  chan api.ServiceInfo
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
	s := &service{
		Config:            conf,
		Dependencies:      deps,
		hostID:            hostID,
		nwControlChanges:  make(chan api.ServiceInfo),
		lokiChanges:       make(chan api.ServiceInfo),
		timeOffsetChanges: make(chan int64),
		environmentSem:    semaphore.NewWeighted(1),
		workerSem:         semaphore.NewWeighted(1),
		startedAt:         time.Now(),
	}
	s.nwControlListener = discovery.NewServiceListener(deps.Logger, api.ServiceTypeNetworkControl, true, s.nwControlChanged)
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

	var nwControlInfo *api.ServiceInfo

	// Start discovery listeners
	go s.nwControlListener.Run(ctx)
	go s.lokiListener.Run(ctx)
	go s.LokiLogger.Run(ctx, log, s.hostID, s.lokiChanges, s.timeOffsetChanges)

	for {
		// Register worker
		s.Bridge.BlinkGreenLED(time.Millisecond * 250)
		s.Bridge.SetRedLED(false)

		select {
		case info := <-s.nwControlChanges:
			// NetworkControlService discovery change detected
			nwControlInfo = &info
			log.Debug().Msg("NetworkControl discovery change received")
		case <-ctx.Done():
			// Context canceled
			return nil
		case <-time.After(time.Second * 2):
			// Retry
		}

		// Do we have discovery info for nwControl ?
		if nwControlInfo == nil {
			// Wait for more info
			continue
		}

		if err := func(nwControlInfo *api.ServiceInfo) error {
			// Dialog connection to nwControl
			nwControlConn, err := grpcutil.DialConn(nwControlInfo)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to dial NetworkControl service")
				return nil
			}
			defer nwControlConn.Close()
			nwControlClient := api.NewNetworkControlServiceClient(nwControlConn)

			// Initialization done, run loop
			workerCtx, workerCancel := context.WithCancel(ctx)
			s.mutex.Lock()
			s.workerCancel = workerCancel
			s.mutex.Unlock()
			err = s.runWorkerInEnvironment(workerCtx, log, nwControlClient)
			workerCancel()
			if err != nil {
				log.Debug().Err(err).Msg("runWorkerInEnvironment failed")
				return err
			}
			return nil
		}(nwControlInfo); err != nil {
			log.Warn().Err(err).Msg("Worker loop failed. Retrying...")
		}

		// If context cancelled, return
		if ctx.Err() != nil {
			return nil
		}
	}
}

// NetworkControlService has changed
func (s *service) nwControlChanged(info api.ServiceInfo) {
	networkControlServiceChangesTotal.Inc()
	s.mutex.Lock()
	cancel := s.workerCancel
	s.mutex.Unlock()
	if cancel != nil {
		s.Logger.Debug().Msg("nwControlChanged: canceling worker")
		cancel()
	}
	s.nwControlChanges <- info
}

// Loki service has changed
func (s *service) lokiChanged(info api.ServiceInfo) {
	lokiServiceChangesTotal.Inc()
	s.lokiChanges <- info
}

// Run the worker until the given context is cancelled.
func (s *service) runWorkerInEnvironment(ctx context.Context,
	log zerolog.Logger,
	nwControlClient api.NetworkControlServiceClient) error {

	// Prepare logger
	environmentID := atomic.AddUint32(&s.lastEnvironmentID, 1)
	log = log.With().Uint32("environment-id", environmentID).Logger()

	// Acquire environment semaphore
	if err := s.environmentSem.Acquire(ctx, 1); err != nil {
		log.Warn().Err(err).Msg("Failed to acquire environment semaphore")
		return err
	}
	defer s.environmentSem.Release(1)

	// Check context cancelation
	if err := ctx.Err(); err != nil {
		log.Warn().Err(err).Msg("Environment context canceled before we started")
		return err
	}

	// Set metrics
	currentEnvironmentIDGauge.Set(float64(environmentID))

	// Initialization done, run loop
	s.Bridge.SetGreenLED(true)
	s.Bridge.SetRedLED(false)

	defer func() {
		s.Bridge.SetGreenLED(false)
		s.Bridge.SetRedLED(true)
		if s.shutdown {
			if err := environment.Reboot(log); err != nil {
				log.Error().Err(err).Msg("Reboot failed")
			}
			// Sleep before exit to allow logs to be send
			log.Warn().Msg("About to exit process")
			time.Sleep(time.Second * 5)
			// Do actual exit
			log.Warn().Msg("Exiting process")
			os.Exit(1)
		}
	}()

	configChanged := make(chan *api.LocalWorkerConfig)
	defer close(configChanged)
	stopWorker := make(chan struct{})
	defer close(stopWorker)
	g, ctx := errgroup.WithContext(ctx)

	// Keep requesting configuration in stream
	g.Go(func() error {
		return s.runLoadConfig(ctx, log, nwControlClient, configChanged, s.timeOffsetChanges, stopWorker)
	})

	// Keep running a worker
	g.Go(func() error {
		return s.runWorkers(ctx, log, nwControlClient, configChanged, stopWorker)
	})

	return g.Wait()
}
