package worker

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/binkynet/LocalWorker/model"
	"github.com/binkynet/LocalWorker/service/bridge"
	"github.com/binkynet/LocalWorker/service/devices"
)

// Service contains the API exposed by the worker service
type Service interface {
	// Run the worker service until the given context is cancelled.
	Run(ctx context.Context) error
}

type Dependencies struct {
	Log    zerolog.Logger
	Bridge bridge.Bridge
}

// NewService instantiates a new Service.
func NewService(config model.LocalConfiguration, deps Dependencies) (Service, error) {
	return &service{
		config:       config,
		Dependencies: deps,
	}, nil
}

type service struct {
	config model.LocalConfiguration
	Dependencies
	devService devices.Service
}

// Run the worker service until the given context is cancelled.
func (s *service) Run(ctx context.Context) error {
	// Open I2C bus
	bus, err := s.Bridge.I2CBus()
	if err != nil {
		return maskAny(err)
	}
	// Configure devices
	devService, err := devices.NewService(s.config.Devices, bus)
	if err != nil {
		return maskAny(err)
	}
	s.devService = devService
	if err := devService.Configure(ctx); err != nil {
		// Log error
		s.Log.Error().Err(err).Msg("Not all devices are configured")
	}

	<-ctx.Done()
	return nil
}
