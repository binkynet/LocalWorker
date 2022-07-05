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
	"sync"
	"sync/atomic"
	"time"

	api "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/rs/zerolog"
	"golang.org/x/sync/semaphore"

	"github.com/binkynet/LocalWorker/service/bridge"
	"github.com/binkynet/LocalWorker/service/objects"
	"github.com/binkynet/LocalWorker/service/worker"
)

type workerRunner struct {
	hostID        string
	bridge        bridge.API
	actuals       objects.ServiceActuals
	lastWorkerID  uint32
	workerSem     *semaphore.Weighted
	currentWorker struct {
		worker.Service
		mutex sync.RWMutex
	}
}

// newWorkerRunner creates a new worker runner
func newWorkerRunner(hostID string, bridge bridge.API, actuals objects.ServiceActuals) *workerRunner {
	return &workerRunner{
		hostID:    hostID,
		bridge:    bridge,
		actuals:   actuals,
		workerSem: semaphore.NewWeighted(1),
	}
}

// Gets the current worker (if any)
func (s *workerRunner) GetWorker() worker.Service {
	s.currentWorker.mutex.RLock()
	defer s.currentWorker.mutex.RUnlock()
	return s.currentWorker.Service
}

// runWorkers keeps creating and running workers until the given context is cancelled.
func (s *workerRunner) runWorkers(ctx context.Context,
	log zerolog.Logger,
	configChanged <-chan *api.LocalWorkerConfig) error {

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
				s.runWorkerWithConfig(ctx, log, conf, moduleID)
			}(lctx, log, *conf)
		}
	}
}

// runWorkerWithConfig runs a worker with given config until the given context is cancelled.
func (s *workerRunner) runWorkerWithConfig(ctx context.Context,
	log zerolog.Logger,
	conf api.LocalWorkerConfig,
	moduleID string) {

	defer func() {
		s.currentWorker.mutex.Lock()
		s.currentWorker.Service = nil
		s.currentWorker.mutex.Unlock()

		if err := recover(); err != nil {
			log.Error().Interface("err", err).Msg("Recovered from panic")
		}
	}()
	for {
		log.Debug().Msg("Creating new worker service")
		w, err := worker.NewService(worker.Config{
			LocalWorkerConfig: conf,
			HardwareID:        s.hostID,
			ModuleID:          moduleID,
		}, worker.Dependencies{
			Log:     log,
			Bridge:  s.bridge,
			Actuals: s.actuals,
		})
		if err != nil {
			log.Error().Err(err).Msg("Failed to create worker")
			// Wait a bit and then retry
		} else {
			// Run worker
			log.Debug().Msg("start to run worker...")
			s.currentWorker.mutex.Lock()
			s.currentWorker.Service = w
			s.currentWorker.mutex.Unlock()
			if err := w.Run(ctx); ctx.Err() != nil {
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
