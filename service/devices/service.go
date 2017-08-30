package devices

import (
	"context"

	aerr "github.com/ewoutp/go-aggregate-error"

	"github.com/binkynet/LocalWorker/model"
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
	devices           map[string]Device
	configuredDevices map[string]Device
}

// NewService instantiates a new Service and Device's for the given
// device configurations.
func NewService(configs []model.HWDevice, bus *bridge.I2CBus) (Service, error) {
	s := &service{
		devices:           make(map[string]Device),
		configuredDevices: make(map[string]Device),
	}
	for _, c := range configs {
		var dev Device
		var err error
		switch c.Type {
		case model.HWDeviceTypeMCP23017:
			dev, err = newMcp23017(c, bus)
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
	return s.configuredDevices[id]
}

// Configure is called once to put all devices in the desired state.
func (s *service) Configure(ctx context.Context) error {
	var ae aerr.Aggregate
	configuredDevices := make(map[string]Device)
	for id, d := range s.devices {
		if err := d.Configure(ctx); err != nil {
			ae.Add(maskAny(err))
		} else {
			configuredDevices[id] = d
		}
	}
	s.configuredDevices = configuredDevices
	return ae.AsError()
}

// Close brings all devices back to a safe state.
func (s *service) Close() error {
	var ae aerr.Aggregate
	for _, d := range s.devices {
		if err := d.Close(ctx); err != nil {
			ae.Add(maskAny(err))
		}
	}
	return ae.AsError()
}
