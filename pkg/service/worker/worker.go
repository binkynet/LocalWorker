package worker

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/binkynet/LocalWorker/pkg/service/bridge"
	"github.com/binkynet/LocalWorker/pkg/service/devices"
	"github.com/binkynet/LocalWorker/pkg/service/intf"
	"github.com/binkynet/LocalWorker/pkg/service/objects"
)

// Service contains the API exposed by the worker service
type Service interface {
	// Run the worker service until the given context is cancelled.
	Run(ctx context.Context) error
	intf.GetRequestService
}

type Config struct {
	model.LocalWorkerConfig
	ProgramVersion    string
	ModuleID          string
	HardwareID        string
	MetricsPort       int
	GRPCPort          int
	SSHPort           int
	MQTTBrokerAddress string
}

type Dependencies struct {
	Log             zerolog.Logger
	Bridge          bridge.API
	NwControlClient model.NetworkControlServiceClient
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
	devService, err := devices.NewService(s.config.HardwareID, s.config.ModuleID, s.config.ProgramVersion, s.config.MQTTBrokerAddress, s.config.GetDevices(), s.Bridge, bus, s.Log)
	if err != nil {
		log.Debug().Err(err).Msg("devices.NewService failed")
		return fmt.Errorf("devices.NewService failed: %w", err)
	}
	s.devService = devService

	defer func() {
		log.Debug().Msg("closing devices service")
		devService.Close(context.Background())
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
	objService, err := objects.NewService(s.config.ModuleID, s.config.ProgramVersion,
		s.config.MetricsPort, s.config.GRPCPort, s.config.SSHPort, s.config.Objects,
		devService, s.Log.With().Str("component", "worker.objects").Logger())
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
		// Broadcast service info
		sb := model.NewServiceBroadcaster(log, "",
			model.ServiceTypePrometheusProvider, model.ServiceInfo{
				ApiVersion:   "v1",
				ApiPort:      int32(s.config.MetricsPort),
				Secure:       false,
				ProviderName: s.config.ModuleID,
			})
		return sb.Run(ctx)
	})
	g.Go(func() error {
		log.Debug().Msg("run devices")
		if err := devService.Run(lctx, s.NwControlClient); err != nil {
			log.Error().Err(err).Msg("Run devices failed")
			return fmt.Errorf("failed to run devices: %w", err)
		}
		log.Debug().Msg("run devices ended")
		return nil
	})
	g.Go(func() error {
		s.Log.Debug().Msg("run objects")
		if err := objService.Run(lctx, s.NwControlClient); err != nil {
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

func (s *service) GetRequestService() intf.RequestService {
	if os := s.objService; os != nil {
		return os
	}
	return nil
}
