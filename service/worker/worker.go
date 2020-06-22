package worker

import (
	"context"

	"golang.org/x/sync/errgroup"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/rs/zerolog"

	"github.com/binkynet/LocalWorker/service/bridge"
	"github.com/binkynet/LocalWorker/service/devices"
	"github.com/binkynet/LocalWorker/service/objects"
)

// Service contains the API exposed by the worker service
type Service interface {
	// Run the worker service until the given context is cancelled.
	Run(ctx context.Context, lwControlClient model.LocalWorkerControlServiceClient) error
}

type Config struct {
	model.LocalWorkerConfig
	ProgramVersion string
	ModuleID       string
}

type Dependencies struct {
	Log    zerolog.Logger
	Bridge bridge.API
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

// Run the worker service until the given context is cancelled.
func (s *service) Run(ctx context.Context, lwControlClient model.LocalWorkerControlServiceClient) error {
	// Open I2C bus
	s.Log.Debug().Msg("open I2C bus")
	bus, err := s.Bridge.I2CBus()
	if err != nil {
		return maskAny(err)
	}
	// Build devices service
	s.Log.Debug().Msg("build devices service")
	devService, err := devices.NewService(s.config.GetDevices(), bus, s.Log)
	if err != nil {
		return maskAny(err)
	}
	s.devService = devService

	defer func() {
		s.Log.Debug().Msg("closing devices service")
		devService.Close()
	}()

	// Configure devices
	s.Log.Debug().Msg("configure devices")
	if err := devService.Configure(ctx); err != nil {
		// Log error
		s.Log.Error().Err(err).Msg("Not all devices are configured")
	}

	// Build objects service
	s.Log.Debug().Msg("build objects service")
	objService, err := objects.NewService(s.config.ModuleID, s.config.ProgramVersion, s.config.Objects, devService, s.Log.With().Str("component", "worker.objects").Logger())
	if err != nil {
		return maskAny(err)
	}
	s.objService = objService

	// Configure objects
	s.Log.Debug().Msg("configure objects")
	if err := objService.Configure(ctx); err != nil {
		// Log error
		s.Log.Error().Err(err).Msg("Not all objects are configured")
	}

	// Run devices & objects
	g, lctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		s.Log.Debug().Msg("run devices")
		if err := devService.Run(lctx); err != nil {
			s.Log.Error().Err(err).Msg("Run failed")
			return maskAny(err)
		}
		return nil
	})
	g.Go(func() error {
		s.Log.Debug().Msg("run objects")
		if err := objService.Run(lctx, lwControlClient); err != nil {
			s.Log.Error().Err(err).Msg("Run failed")
			return maskAny(err)
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return maskAny(err)
	}

	return nil
}
