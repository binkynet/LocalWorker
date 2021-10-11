//    Copyright 2021 Ewout Prangsma
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
	"sync/atomic"
	"time"

	api "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/rs/zerolog"

	"github.com/binkynet/LocalWorker/service/worker"
)

// runWorkers keeps creating and running workers until the given context is cancelled.
func (s *service) runWorkers(ctx context.Context,
	log zerolog.Logger,
	lwConfigClient api.LocalWorkerConfigServiceClient,
	lwControlClient api.LocalWorkerControlServiceClient,
	configChanged <-chan *api.LocalWorkerConfig,
	stopWorker <-chan struct{}) error {

	// Keep running a worker
	log = log.With().Str("component", "worker-runner").Logger()
	var conf *api.LocalWorkerConfig
	var cancel context.CancelFunc
	for {
		select {
		case c := <-configChanged:
			// Start/restart worker
			if c != nil {
				conf = c
				log.Debug().Msg("Configuration changed")
				if cancel != nil {
					cancel()
				}
			} else {
				log.Warn().Msg("Received nil configuration")
				continue
			}
		case <-stopWorker:
			log.Info().Msg("Stop worker")
			conf = nil
			if cancel != nil {
				cancel()
			}
			return nil
		case <-ctx.Done():
			// Context canceled
			if cancel != nil {
				cancel()
			}
			return nil
		}

		// Prepare new worker
		if conf != nil {
			var lctx context.Context
			lctx, cancel = context.WithCancel(ctx)
			moduleID := s.hostID
			if alias := conf.GetAlias(); alias != "" {
				moduleID = alias
			}
			workerID := atomic.AddUint32(&s.lastWorkerID, 1)
			log := log.With().
				Str("module-id", moduleID).
				Uint32("worker-id", workerID).
				Logger()
			go func(ctx context.Context, log zerolog.Logger, conf api.LocalWorkerConfig) {
				// Aqcuire the semaphore
				log.Debug().Msg("Acquiring worker semaphore...")
				if err := s.workerSem.Acquire(ctx, 1); err != nil {
					log.Warn().Err(err).Msg("Failed to acquire worker semaphore")
					return
				}
				// Release semaphore when worker is done.
				defer s.workerSem.Release(1)
				log.Debug().Msg("Acquired worker semaphore")

				// Check context cancelation
				if err := ctx.Err(); err != nil {
					log.Warn().Err(err).Msg("Worker context canceled before we started")
					return
				}

				// Run the worker
				s.runWorkerWithConfig(ctx, log, lwConfigClient, lwControlClient, conf, moduleID)
			}(lctx, log, *conf)
		}
	}
}

// runWorkerWithConfig runs a worker with given config until the given context is cancelled.
func (s *service) runWorkerWithConfig(ctx context.Context,
	log zerolog.Logger,
	lwConfigClient api.LocalWorkerConfigServiceClient,
	lwControlClient api.LocalWorkerControlServiceClient,
	conf api.LocalWorkerConfig,
	moduleID string) {

	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	for {
		log.Debug().Msg("Creating new worker service")
		w, err := worker.NewService(worker.Config{
			LocalWorkerConfig: conf,
			ProgramVersion:    s.ProgramVersion,
			HardwareID:        s.hostID,
			ModuleID:          moduleID,
		}, worker.Dependencies{
			Log:    log,
			Bridge: s.Bridge,
		})
		if err != nil {
			log.Error().Err(err).Msg("Failed to create worker")
			// Wait a bit and then retry
		} else {
			// Run worker
			log.Debug().Msg("start to run worker...")
			if err := w.Run(ctx, lwControlClient); ctx.Err() != nil {
				log.Info().Msg("Worker ended with context cancellation")
				return
			} else if err != nil {
				log.Error().Err(err).Msg("Failed to run worker")
			} else {
				log.Info().Err(err).Msg("Worker ended without context cancellation")
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
}
