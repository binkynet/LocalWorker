package devices

import (
	"context"

	aerr "github.com/ewoutp/go-aggregate-error"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/LocalWorker/service/bridge"
	"github.com/pkg/errors"
)

// Service contains the API that is exposed by the device service.
type Service interface {
	// DeviceByID returns the device with given ID.
	// Return false if not found
	DeviceByID(id string) (Device, bool)
	// Configure is called once to put all devices in the desired state.
	Configure(ctx context.Context) error
	// Close brings all devices back to a safe state.
	Close() error
}

type service struct {
	log               zerolog.Logger
	devices           map[string]Device
	configuredDevices map[string]Device
}

// NewService instantiates a new Service and Device's for the given
// device configurations.
func NewService(configs []model.HWDevice, bus *bridge.I2CBus, log zerolog.Logger) (Service, error) {
	s := &service{
		log:               log.With().Str("component", "device-service").Logger(),
		devices:           make(map[string]Device),
		configuredDevices: make(map[string]Device),
	}
	for _, c := range configs {
		var dev Device
		var err error
		switch c.Type {
		case model.HWDeviceTypeMCP23017:
			dev, err = newMcp23017(c, bus)
		case model.HWDeviceTypePCA9685:
			dev, err = newPCA9685(c, bus)
		default:
			return nil, errors.Wrapf(model.ValidationError, "Unsupported device type '%s'", c.Type)
		}
		if err != nil {
			return nil, maskAny(err)
		}
		s.devices[c.ID] = dev
	}
	return s, nil
}

// DeviceByID returns the device with given ID.
// Return false if not found or not configured.
func (s *service) DeviceByID(id string) (Device, bool) {
	dev, ok := s.configuredDevices[id]
	return dev, ok
}

// Configure is called once to put all devices in the desired state.
func (s *service) Configure(ctx context.Context) error {
	var ae aerr.AggregateError
	configuredDevices := make(map[string]Device)
	for id, d := range s.devices {
		log.Debug().
			Str("id", id).
			Msg("Configuring device")
		if err := d.Configure(ctx); err != nil {
			log.Warn().
				Err(err).
				Str("id", id).
				Msg("Failed to configure device")
			ae.Add(maskAny(err))
		} else {
			configuredDevices[id] = d
		}
	}
	s.configuredDevices = configuredDevices
	log.Info().Int("count", len(configuredDevices)).Msg("Configured devices")
	return ae.AsError()
}

// Close brings all devices back to a safe state.
func (s *service) Close() error {
	var ae aerr.AggregateError
	for _, d := range s.devices {
		if err := d.Close(); err != nil {
			ae.Add(maskAny(err))
		}
	}
	return ae.AsError()
}
