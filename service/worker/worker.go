// Copyright 2022 Ewout Prangsma
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

package worker

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/binkynet/LocalWorker/service/bridge"
	"github.com/binkynet/LocalWorker/service/devices"
	"github.com/binkynet/LocalWorker/service/objects"
)

// Service contains the API exposed by the worker service
type Service interface {
	// Gets the hash of m config
	GetConfigHash() string
	// Run the worker service until the given context is cancelled.
	Run(ctx context.Context) error

	// Perform device discovery
	DiscoverDevices(ctx context.Context, req *model.DiscoverDevicesRequest) (*model.DiscoverDevicesResult, error)
	// Set the given power state
	SetPower(ctx context.Context, req *model.PowerState) error
	// Set the given output state
	SetOutput(ctx context.Context, req *model.Output) error
	// Set the given switch state
	SetSwitch(ctx context.Context, req *model.Switch) error
}

type Config struct {
	model.LocalWorkerConfig
	ModuleID   string
	HardwareID string
}

type Dependencies struct {
	Log     zerolog.Logger
	Bridge  bridge.API
	Actuals objects.ServiceActuals
}

// NewService instantiates a new Service.
func NewService(config Config, deps Dependencies) (Service, error) {
	return &service{
		config:       config,
		Dependencies: deps,
	}, nil
}

type service struct {
	config Config
	Dependencies
	devService devices.Service
	objService objects.Service
}

// Gets the hash of m config
func (s *service) GetConfigHash() string {
	return s.config.GetHash()
}

// Run the worker service until the given context is cancelled.
func (s *service) Run(ctx context.Context) error {
	log := s.Log
	// Open I2C bus
	log.Debug().Msg("open I2C bus")
	bus, err := s.Bridge.I2CBus()
	if err != nil {
		log.Debug().Err(err).Msg("Open I2CBus failed")
		return fmt.Errorf("failed to open I2C bus: %w", err)
	}
	// Build devices service
	log.Debug().Msg("build devices service")
	devService, err := devices.NewService(s.config.HardwareID, s.config.ModuleID, s.config.GetDevices(), s.Bridge, bus, s.Log)
	if err != nil {
		log.Debug().Err(err).Msg("devices.NewService failed")
		return fmt.Errorf("devices.NewService failed: %w", err)
	}
	s.devService = devService

	defer func() {
		log.Debug().Msg("closing devices service")
		devService.Close()
	}()

	// Configure devices
	log.Debug().Msg("configure devices")
	if err := devService.Configure(ctx); err != nil {
		// Log error
		log.Error().Err(err).Msg("Not all devices are configured")
	}
	// Stop fast if context canceled
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Build objects service
	log.Debug().Msg("build objects service")
	objService, err := objects.NewService(s.config.ModuleID, s.config.Objects, devService, &s.Actuals, s.Log.With().Str("component", "worker.objects").Logger())
	if err != nil {
		log.Debug().Err(err).Msg("objects.NewService failed")
		return fmt.Errorf("objects.NewService failed: %w", err)
	}
	s.objService = objService

	// Configure objects
	s.Log.Debug().Msg("configure objects")
	if err := objService.Configure(ctx); err != nil {
		// Log error
		s.Log.Error().Err(err).Msg("Not all objects are configured")
	}
	// Stop fast if context canceled
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Run devices & objects
	g, lctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		log.Debug().Msg("run devices")
		if err := devService.Run(lctx); err != nil {
			log.Error().Err(err).Msg("Run devices failed")
			return fmt.Errorf("failed to run devices: %w", err)
		}
		log.Debug().Msg("run devices ended")
		return nil
	})
	g.Go(func() error {
		s.Log.Debug().Msg("run objects")
		if err := objService.Run(lctx); err != nil {
			log.Error().Err(err).Msg("Run objects failed")
			return fmt.Errorf("failed to run objects: %w", err)
		}
		s.Log.Debug().Msg("run objects ended")
		return nil
	})
	if err := g.Wait(); err != nil {
		return errors.Wrap(err, "Wait failed")
	}

	return nil
}

// Perform device discovery
func (s *service) DiscoverDevices(ctx context.Context, req *model.DiscoverDevicesRequest) (*model.DiscoverDevicesResult, error) {
	return s.devService.DiscoverDevices(ctx, req)
}

// Set the given power state
func (s *service) SetPower(ctx context.Context, req *model.PowerState) error {
	return s.objService.SetPower(ctx, req)
}

// Set the given output state
func (s *service) SetOutput(ctx context.Context, req *model.Output) error {
	return s.objService.SetOutput(ctx, req)
}

// Set the given switch state
func (s *service) SetSwitch(ctx context.Context, req *model.Switch) error {
	return s.objService.SetSwitch(ctx, req)
}
