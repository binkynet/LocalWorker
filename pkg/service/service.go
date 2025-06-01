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
	"net"
	"os"
	"sync"
	"time"

	api "github.com/binkynet/BinkyNet/apis/v1"
	discovery "github.com/binkynet/BinkyNet/discovery"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"

	"github.com/binkynet/LocalWorker/pkg/environment"
	"github.com/binkynet/LocalWorker/pkg/service/bridge"
	"github.com/binkynet/LocalWorker/pkg/service/intf"
	"github.com/binkynet/LocalWorker/pkg/service/ncs"
	grpcutil "github.com/binkynet/LocalWorker/pkg/service/util"
)

type Service interface {
	// Run the worker until the given context is cancelled.
	Run(ctx context.Context)
	api.LocalWorkerServiceServer
}

type Config struct {
	ProgramVersion     string
	MetricsPort        int
	GRPCPort           int
	SSHPort            int
	HostID             string // Only used if not empty
	IsVirtual          bool
	VirtualServiceInfo *api.ServiceInfo
}

type Dependencies struct {
	Logger     zerolog.Logger
	Bridge     bridge.API
	LokiLogger LokiLogger
	// Semaphore used to guard from running multiple NCS instance
	// concurrently.
	NcsSem    *semaphore.Weighted
	WorkerSem *semaphore.Weighted
}

type service struct {
	Config
	Dependencies

	mutex             sync.Mutex
	hostID            string
	ncsCancel         func()
	shutdown          bool
	startedAt         time.Time
	getRequestService intf.GetRequestService

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
	hostID := conf.HostID
	if hostID == "" {
		var err error
		hostID, err = createHostID()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create host ID")
		}
	}
	deps.Logger = deps.Logger.With().Str("module-id", hostID).Logger()
	s := &service{
		Config:            conf,
		Dependencies:      deps,
		hostID:            hostID,
		nwControlChanges:  make(chan api.ServiceInfo),
		lokiChanges:       make(chan api.ServiceInfo),
		timeOffsetChanges: make(chan int64),
		startedAt:         time.Now(),
	}
	if !s.IsVirtual {
		s.nwControlListener = discovery.NewServiceListener(deps.Logger, api.ServiceTypeNetworkControl, true, s.nwControlChanged)
	}
	s.lokiListener = discovery.NewServiceListener(deps.Logger, api.ServiceTypeLokiProvider, true, s.lokiChanged)
	return s, nil
}

// Run initialize the local worker and then continues
// to register the worker, followed by running the worker loop
// in a given environment.
func (s *service) Run(ctx context.Context) {
	log := s.Logger.With().Str("host-id", s.hostID).Logger()
	defer s.Bridge.Close()

	// Create host ID
	log.Info().Msg("Found host ID")

	// Fetch local slave configuration
	s.Bridge.BlinkGreenLED(time.Millisecond * 250)
	s.Bridge.BlinkRedLED(time.Millisecond * 250)

	var nwControlInfo *api.ServiceInfo

	// Start discovery listeners
	if !s.IsVirtual {
		go s.nwControlListener.Run(ctx)
	}
	go s.lokiListener.Run(ctx)
	go s.LokiLogger.Run(ctx, log, s.hostID, s.lokiChanges, s.timeOffsetChanges)

	for {
		// Register worker
		s.Bridge.BlinkGreenLED(time.Millisecond * 250)
		s.Bridge.SetRedLED(false)

		if s.IsVirtual {
			nwControlInfo = s.VirtualServiceInfo
		} else {
			select {
			case info := <-s.nwControlChanges:
				// NetworkControlService discovery change detected
				nwControlInfo = &info
				log.Debug().Msg("NetworkControl discovery change received")
			case <-ctx.Done():
				// Context canceled
				return
			case <-time.After(time.Second * 2):
				// Retry
			}
		}

		// Do we have discovery info for nwControl ?
		if nwControlInfo == nil {
			// Wait for more info
			continue
		}

		// Dialog connection to nwControl
		log.Debug().Msg("Dialing NetworkControl service...")
		mqttBrokerAddress := net.JoinHostPort(nwControlInfo.ApiAddress, "1883")
		nwControlConn, err := grpcutil.DialConn(nwControlInfo)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to dial NetworkControl service")
			// Reset service info, so we need to get a fresh update
			nwControlInfo = nil
		}

		// Run NCS with given nwControlConn in local func,
		// so we can close the connection on exit.
		func(ncControlConn *grpc.ClientConn, mqttBrokerAddress string) {
			// Ensure we close the connection on exit
			defer nwControlConn.Close()
			nwControlClient := api.NewNetworkControlServiceClient(nwControlConn)

			// Initialization done, run loop
			ncsCtx, ncsCancel := context.WithCancel(ctx)
			s.mutex.Lock()
			s.ncsCancel = ncsCancel
			s.mutex.Unlock()
			ncs := ncs.NewNetworkControlService(log, s.ProgramVersion, s.hostID,
				s.MetricsPort, s.GRPCPort, s.SSHPort, mqttBrokerAddress,
				s.timeOffsetChanges, s.Bridge, s.IsVirtual, nwControlClient, s.WorkerSem)
			s.mutex.Lock()
			s.getRequestService = ncs
			s.mutex.Unlock()
			runErr := ncs.Run(ncsCtx, s.NcsSem)
			ncsCancel()
			s.mutex.Lock()
			s.getRequestService = nil
			s.mutex.Unlock()
			if runErr != nil {
				log.Debug().Err(runErr).Msg("ncs.Run failed")
			}
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
		}(nwControlConn, mqttBrokerAddress)

		// If context cancelled, return
		if ctx.Err() != nil {
			return
		}
	}
}

// NetworkControlService has changed
func (s *service) nwControlChanged(info api.ServiceInfo) {
	log := s.Logger
	networkControlServiceChangesTotal.Inc()
	s.mutex.Lock()
	cancel := s.ncsCancel
	s.mutex.Unlock()
	if cancel != nil {
		log.Debug().Msg("nwControlChanged: canceling NCS")
		cancel()
	}
	log.Debug().Msg("Sending nwControl change into channel...")
	s.nwControlChanges <- info
	log.Debug().Msg("Sent nwControl change into channel.")
}

// Loki service has changed
func (s *service) lokiChanged(info api.ServiceInfo) {
	log := s.Logger
	lokiServiceChangesTotal.Inc()
	log.Debug().Msg("Sending loki change into channel...")
	s.lokiChanges <- info
	log.Debug().Msg("Sent loki change into channel.")
}
