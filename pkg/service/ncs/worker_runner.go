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

package ncs

import (
	"context"
	"sync/atomic"
	"time"

	api "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/rs/zerolog"
	"golang.org/x/sync/semaphore"

	"github.com/binkynet/LocalWorker/pkg/service/worker"
)

var (
	// Semaphore used to guard from running multiple worker instances
	// concurrently.
	workerSem = semaphore.NewWeighted(1)
)

// runWorkers keeps creating and running workers until the given context is cancelled.
func (ncs *networkControlService) runWorkers(ctx context.Context,
	configChanged <-chan *api.LocalWorkerConfig) {

	// Keep running a worker
	log := ncs.log.With().Str("component", "worker-runner").Logger()
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
		case <-ctx.Done():
			// Context canceled
			log.Info().Msg("ncs.Worker context canceled. Stopping worker (if any)")
			if cancel != nil {
				cancel()
			}
			return
		}

		// Prepare new worker
		if conf != nil {
			var lctx context.Context
			lctx, cancel = context.WithCancel(ctx)
			moduleID := ncs.hostID
			if alias := conf.GetAlias(); alias != "" {
				moduleID = alias
			}
			workerID := atomic.AddUint32(&lastWorkerID, 1)
			log := log.With().
				Str("module-id", moduleID).
				Uint32("worker-id", workerID).
				Logger()
			workerCountTotal.Inc()
			go func(ctx context.Context, log zerolog.Logger, conf api.LocalWorkerConfig, workerID uint32) {
				// Aqcuire the semaphore
				log.Debug().Msg("Acquiring worker semaphore...")
				timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*10)
				defer cancel()
				if err := workerSem.Acquire(timeoutCtx, 1); err != nil {
					log.Warn().Err(err).Msg("Failed to acquire worker semaphore")
					if timeoutCtx.Err() != nil {
						log.Fatal().Msg("Failed to acquire worker semaphore in time. Restarting")
					}
					return
				}
				// Release semaphore when worker is done.
				defer func() {
					log.Debug().Msg("Releasing worker semaphore...")
					workerSem.Release(1)
					log.Debug().Msg("Released worker semaphore.")
				}()
				log.Debug().Msg("Acquired worker semaphore.")

				// Check context cancelation
				if err := ctx.Err(); err != nil {
					log.Warn().Err(err).Msg("Worker context canceled before we started")
					return
				}

				// Run the worker
				currentWorkerIDGauge.Set(float64(workerID))
				ncs.runWorkerWithConfig(ctx, log, conf, moduleID)
			}(lctx, log, *conf, workerID)
		}
	}
}

// runWorkerWithConfig runs a worker with given config until the given context is cancelled.
func (ncs *networkControlService) runWorkerWithConfig(ctx context.Context,
	log zerolog.Logger,
	conf api.LocalWorkerConfig,
	moduleID string) {

	defer func() {
		if err := recover(); err != nil {
			log.Error().Interface("err", err).Msg("Recovered from panic")
		}
	}()
	for {
		log.Debug().Msg("Creating new worker service")
		w, err := worker.NewService(worker.Config{
			LocalWorkerConfig: conf,
			ProgramVersion:    ncs.programVersion,
			HardwareID:        ncs.hostID,
			ModuleID:          moduleID,
			MetricsPort:       ncs.metricsPort,
			GRPCPort:          ncs.grpcPort,
		}, worker.Dependencies{
			Log:             log,
			Bridge:          ncs.bridge,
			NwControlClient: ncs.nwControlClient,
		})
		if err != nil {
			log.Error().Err(err).Msg("Failed to create worker")
			// Wait a bit and then retry
		} else {
			// Run worker
			ncs.getrequestService = w
			log.Debug().Msg("start to run worker...")
			if err := w.Run(ctx); ctx.Err() != nil {
				log.Info().Msg("Worker ended with context cancellation")
				return
			} else if err != nil {
				log.Error().Err(err).Msg("Worker ended with unknown error")
			} else {
				log.Info().Err(err).Msg("Worker ended without context cancellation")
			}
			ncs.getrequestService = nil
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
