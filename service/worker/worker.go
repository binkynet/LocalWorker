package worker

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/LocalWorker/service/bridge"
	"github.com/binkynet/LocalWorker/service/devices"
	"github.com/binkynet/LocalWorker/service/mqtt"
	"github.com/binkynet/LocalWorker/service/objects"
)

// Service contains the API exposed by the worker service
type Service interface {
	// Run the worker service until the given context is cancelled.
	Run(ctx context.Context) error
}

type Dependencies struct {
	Log         zerolog.Logger
	Bridge      bridge.API
	MQTTService mqtt.Service
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
	objService objects.Service
}

// Run the worker service until the given context is cancelled.
func (s *service) Run(ctx context.Context) error {
	// Open I2C bus
	bus, err := s.Bridge.I2CBus()
	if err != nil {
		return maskAny(err)
	}
	// Build services
	devService, err := devices.NewService(s.config.Devices, bus)
	if err != nil {
		return maskAny(err)
	}
	objService, err := objects.NewService(s.config.Objects)
	if err != nil {
		return maskAny(err)
	}
	s.devService = devService
	s.objService = objService

	defer func() {
		devService.Close()
		objService.Close()
	}()

	// Configure devices
	if err := devService.Configure(ctx); err != nil {
		// Log error
		s.Log.Error().Err(err).Msg("Not all devices are configured")
	}
	// Configure objects
	if err := objService.Configure(ctx); err != nil {
		// Log error
		s.Log.Error().Err(err).Msg("Not all objects are configured")
	}

	<-ctx.Done()
	return nil
}
