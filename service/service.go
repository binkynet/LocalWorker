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
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/binkynet/BinkyNet/apis/util"
	api "github.com/binkynet/BinkyNet/apis/v1"
	discovery "github.com/binkynet/BinkyNet/discovery"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/binkynet/LocalWorker/pkg/environment"
	"github.com/binkynet/LocalWorker/service/bridge"
	"github.com/binkynet/LocalWorker/service/worker"
)

type Service interface {
	// Run the worker until the given context is cancelled.
	Run(ctx context.Context) error
}

type Config struct {
	ProgramVersion string
}

type Dependencies struct {
	Log    zerolog.Logger
	Bridge bridge.API
}

type service struct {
	Config
	Dependencies

	mutex        sync.Mutex
	hostID       string
	workerCancel func()
	shutdown     bool

	lwConfigListener  *discovery.ServiceListener
	lwControlListener *discovery.ServiceListener
	lwConfigChanges   chan api.ServiceInfo
	lwControlChanges  chan api.ServiceInfo
}

// NewService creates a Service instance and returns it.
func NewService(conf Config, deps Dependencies) (Service, error) {
	deps.Log = deps.Log.With().Str("component", "service").Logger()
	// Create host ID
	hostID, err := createHostID()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create host ID")
	}
	s := &service{
		Config:           conf,
		Dependencies:     deps,
		hostID:           hostID,
		lwConfigChanges:  make(chan api.ServiceInfo),
		lwControlChanges: make(chan api.ServiceInfo),
	}
	s.lwConfigListener = discovery.NewServiceListener(deps.Log, api.ServiceTypeLocalWorkerConfig, true, s.lwConfigChanged)
	s.lwControlListener = discovery.NewServiceListener(deps.Log, api.ServiceTypeLocalWorkerControl, true, s.lwControlChanged)
	return s, nil
}

// Run initialize the local worker and then continues
// to register the worker, followed by running the worker loop
// in a given environment.
func (s *service) Run(ctx context.Context) error {
	defer s.Bridge.Close()

	// Create host ID
	s.Log.Info().
		Str("id", s.hostID).
		Msg("Found host ID")

	// Fetch local slave configuration
	s.Bridge.BlinkGreenLED(time.Millisecond * 250)
	s.Bridge.BlinkRedLED(time.Millisecond * 250)

	var lwConfigInfo *api.ServiceInfo
	var lwControlInfo *api.ServiceInfo

	// Start discovery listeners
	go s.lwConfigListener.Run(ctx)
	go s.lwControlListener.Run(ctx)

	for {
		// Register worker
		s.Bridge.BlinkGreenLED(time.Millisecond * 250)
		s.Bridge.SetRedLED(false)

		select {
		case info := <-s.lwConfigChanges:
			// LocalWorkerConfigService discovery change detected
			lwConfigInfo = &info
			s.Log.Debug().Msg("LocalWorkerConfig discovery change received")
		case info := <-s.lwControlChanges:
			// LocalWorkerControlService discovery change detected
			lwControlInfo = &info
			s.Log.Debug().Msg("LocalWorkerControl discovery change received")
		case <-ctx.Done():
			// Context canceled
			return nil
		case <-time.After(time.Second * 2):
			// Retry
		}

		// Do we have discovery info for lwConfig & lwControl ?
		if lwConfigInfo == nil || lwControlInfo == nil {
			// Wait for more info
			continue
		}

		if err := func() error {
			// Dialog connection to lwConfig
			lwConfigConn, err := dialConn(lwConfigInfo)
			if err != nil {
				s.Log.Warn().Err(err).Msg("Failed to dial LocalWorkerConfig service")
				return nil
			}
			defer lwConfigConn.Close()
			lwConfigClient := api.NewLocalWorkerConfigServiceClient(lwConfigConn)
			// Dialog connection to lwControl
			lwControlConn, err := dialConn(lwControlInfo)
			if err != nil {
				s.Log.Warn().Err(err).Msg("Failed to dial LocalWorkerControl service")
				return nil
			}
			defer lwControlConn.Close()
			lwControlClient := api.NewLocalWorkerControlServiceClient(lwControlConn)

			// Initialization done, run loop
			workerCtx, workerCancel := context.WithCancel(ctx)
			s.mutex.Lock()
			s.workerCancel = workerCancel
			s.mutex.Unlock()
			err = s.runWorkerInEnvironment(workerCtx, lwConfigClient, lwControlClient)
			workerCancel()
			if err != nil {
				s.Log.Error().Err(err).Msg("registerWorker failed")
				return err
			}
			return nil
		}(); err != nil {
			return err
		}

		// If context cancelled, return
		if ctx.Err() != nil {
			return nil
		}
	}
}

// LocalWorkerConfigService has changed
func (s *service) lwConfigChanged(info api.ServiceInfo) {
	s.mutex.Lock()
	cancel := s.workerCancel
	s.mutex.Unlock()
	if cancel != nil {
		s.Log.Debug().Msg("lwConfigChanged: canceling worker")
		cancel()
	}
	s.lwConfigChanges <- info
}

// LocalWorkerControlService has changed
func (s *service) lwControlChanged(info api.ServiceInfo) {
	s.mutex.Lock()
	cancel := s.workerCancel
	s.mutex.Unlock()
	if cancel != nil {
		s.Log.Debug().Msg("lwControlChanged: canceling worker")
		cancel()
	}
	s.lwControlChanges <- info
}

// Run the worker until the given context is cancelled.
func (s *service) runWorkerInEnvironment(ctx context.Context, lwConfigClient api.LocalWorkerConfigServiceClient,
	lwControlClient api.LocalWorkerControlServiceClient) error {
	// Initialization done, run loop
	log := s.Log
	s.Bridge.SetGreenLED(true)
	s.Bridge.SetRedLED(false)

	defer func() {
		s.Bridge.SetGreenLED(false)
		s.Bridge.SetRedLED(true)
		if s.shutdown {
			if err := environment.Reboot(s.Log); err != nil {
				s.Log.Error().Err(err).Msg("Reboot failed")
			}
			os.Exit(1)
		}
	}()

	configChanged := make(chan *api.LocalWorkerConfig)
	defer close(configChanged)
	g, ctx := errgroup.WithContext(ctx)

	// Request configuration in stream
	g.Go(func() error {
		log := log.With().Str("component", "config-reader").Logger()
		confStream, err := lwConfigClient.GetConfig(ctx, &api.LocalWorkerInfo{
			Id:          s.hostID,
			Description: "Local worker",
			Version:     s.ProgramVersion,
			Uptime:      0, // TODO
		})
		if err != nil {
			log.Debug().Err(err).Msg("GetConfig failed")
			return err
		}
		defer confStream.CloseSend()
		for {
			// Read configuration
			conf, err := confStream.Recv()
			if util.IsStreamClosed(err) || ctx.Err() != nil {
				return nil
			} else if err != nil {
				log.Error().Err(err).Msg("Failed to read configuration")
				return nil
			}
			log.Debug().Msg("Received new configuration")
			select {
			case configChanged <- conf:
				// Continue
			case <-ctx.Done():
				// Context canceled
				return nil
			}
		}
	})

	// Keep running worker
	g.Go(func() error {
		log := log.With().Str("component", "worker-runner").Logger()
		var conf *api.LocalWorkerConfig
		lctx, cancel := context.WithCancel(ctx)
		for {
			select {
			case c := <-configChanged:
				// Start/restart worker
				if c != nil {
					conf = c
					log.Debug().Msg("Configuration changed")
					cancel()
				} else {
					log.Warn().Msg("Received nil configuration")
					continue
				}
			case <-ctx.Done():
				// Context canceled
				cancel()
				return nil
			case <-lctx.Done():
				// Worker finished
			}

			// Prepare new worker
			lctx, cancel = context.WithCancel(ctx)
			moduleID := s.hostID
			if alias := conf.GetAlias(); alias != "" {
				moduleID = alias
			}
			log = log.With().Str("module-id", moduleID).Logger()
			go func(ctx context.Context, conf *api.LocalWorkerConfig) {
				for {
					w, err := worker.NewService(worker.Config{
						LocalWorkerConfig: *conf,
						ProgramVersion:    s.ProgramVersion,
						ModuleID:          moduleID,
					}, worker.Dependencies{
						Log:    s.Log.With().Str("component", "worker").Logger(),
						Bridge: s.Bridge,
					})
					if err != nil {
						log.Error().Err(err).Msg("Failed to create worker")
						// Wait a bit and then retry
					} else {
						// Run worker
						if err := w.Run(ctx, lwControlClient); err != nil {
							log.Error().Err(err).Msg("Failed to run worker")
						} else {
							log.Info().Err(err).Msg("Worker ended")
						}
					}
					select {
					case <-ctx.Done():
						// Context canceled
						return
					case <-time.After(time.Second):
						// Retry
					}
				}
			}(lctx, conf)
		}
	})

	return g.Wait()
}

func dialConn(info *api.ServiceInfo) (*grpc.ClientConn, error) {
	address := net.JoinHostPort(info.GetApiAddress(), strconv.Itoa(int(info.GetApiPort())))
	var opts []grpc.DialOption
	if !info.Secure {
		opts = append(opts, grpc.WithInsecure())
	}
	return grpc.Dial(address, opts...)
}
