// Copyright 2024 Ewout Prangsma
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author Ewout Prangsma
//

package ncs

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	api "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/pkg/service/bridge"
	"github.com/binkynet/LocalWorker/pkg/service/intf"
)

// NetworkControlService runs a sequence of zero or more
// workers for a specific network control service.
type NetworkControlService interface {
	// Run workers until the given context is canceled.
	Run(context.Context) error
	intf.GetRequestService
}

// NewNetworkControlService constructs a new network control service for the given
// client.
func NewNetworkControlService(log zerolog.Logger, programVersion, hostID string,
	metricsPort, grpcPort, sshPort int, mqttBrokerAddress string,
	timeOffsetChanges chan int64, bridge bridge.API,
	nwControlClient api.NetworkControlServiceClient) NetworkControlService {
	// Prepare logger
	ncsID := atomic.AddUint32(&lastNcsID, 1)

	// Construct service
	ncs := &networkControlService{
		ncsID:             ncsID,
		programVersion:    programVersion,
		metricsPort:       metricsPort,
		grpcPort:          grpcPort,
		sshPort:           sshPort,
		mqttBrokerAddress: mqttBrokerAddress,
		hostID:            hostID,
		log: log.With().
			Str("component", "ncs").
			Uint32("ncs-id", ncsID).
			Logger(),
		timeOffsetChanges: timeOffsetChanges,
		bridge:            bridge,
		nwControlClient:   nwControlClient,
	}
	return ncs
}

// networkControlService implements a NetworkControlService.
type networkControlService struct {
	programVersion    string
	metricsPort       int
	grpcPort          int
	sshPort           int
	mqttBrokerAddress string
	hostID            string
	ncsID             uint32
	log               zerolog.Logger
	timeOffsetChanges chan int64
	bridge            bridge.API
	nwControlClient   api.NetworkControlServiceClient
	getrequestService intf.GetRequestService
}

var (
	// Last ID used for an NCS
	lastNcsID uint32
	// Last ID used for a worker
	lastWorkerID uint32
	// Semaphore used to guard from running multiple NCS instance
	// concurrently.
	ncsSem = semaphore.NewWeighted(1)
	// Error returned when config loader stopped unexpected
	errConfigLoaderStopped = errors.New("config loader stopped")
	// Error returned when workers stopped unexpected
	errWorkersStopped = errors.New("workers stopped")
)

// Run workers until the given context is canceled.
func (ncs *networkControlService) Run(ctx context.Context) error {
	// Prepare logger
	log := ncs.log

	// Acquire environment semaphore
	log.Debug().Msg("Acquiring ncs semaphore...")
	if err := ncsSem.Acquire(ctx, 1); err != nil {
		log.Warn().Err(err).Msg("Failed to acquire ncs semaphore")
		return err
	}
	defer func() {
		log.Debug().Msg("Releasing ncs semaphore...")
		ncsSem.Release(1)
		log.Debug().Msg("Released ncs semaphore.")
	}()
	log.Debug().Msg("Acquired ncs semaphore.")

	// Check context cancelation
	if err := ctx.Err(); err != nil {
		log.Warn().Err(err).Msg("Context canceled before NCS started")
		return err
	}

	// Set metrics
	currentNcsIDGauge.Set(float64(ncs.ncsID))

	// Initialization done, run loop
	ncs.bridge.SetGreenLED(true)
	ncs.bridge.SetRedLED(false)

	defer func() {
		ncs.bridge.SetGreenLED(false)
		ncs.bridge.SetRedLED(true)
	}()

	configChanged := make(chan *api.LocalWorkerConfig)
	defer close(configChanged)
	g, ctx := errgroup.WithContext(ctx)

	// Keep requesting configuration in stream
	g.Go(func() error {
		ncs.runLoadConfig(ctx, configChanged)
		if err := ctx.Err(); err != nil {
			return err
		}
		return errConfigLoaderStopped
	})

	// Keep running a worker
	g.Go(func() error {
		ncs.runWorkers(ctx, configChanged)
		if err := ctx.Err(); err != nil {
			return err
		}
		return errWorkersStopped
	})

	return g.Wait()
}

// Get access to the current request service
func (ncs *networkControlService) GetRequestService() intf.RequestService {
	if grs := ncs.getrequestService; grs != nil {
		return grs.GetRequestService()
	}
	return nil
}
