package devices

import (
	"github.com/binkynet/LocalWorker/model"
	"github.com/binkynet/LocalWorker/service/bridge"
	"github.com/pkg/errors"
)

// Service contains the API that is exposed by the device service.
type Service interface {
	// DeviceByID returns the device with given ID.
	// Return false if not found
	DeviceByID(id string) (Device, bool)
}

type service struct {
	devices map[string]Device
}

// NewService instantiates a new Service and Device's for the given
// device configurations.
func NewService(configs []model.HWDevice, bus *bridge.I2CBus) (Service, error) {
	s := &service{
		devices: make(map[string]Device),
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
