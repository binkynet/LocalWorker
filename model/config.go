package model

import (
	"github.com/pkg/errors"
)

// LocalConfiguration holds the configuration of a single local worker.
type LocalConfiguration struct {
	// List of devices attached to the local worker
	Devices []HWDevice `json:"devices,omitempty"`
	// List of real world objects controlled by the local worker
	Objects []Object `json:"object,omitempty"`
}

// DeviceByID returns the device with given ID.
// Return false if not found.
func (c LocalConfiguration) DeviceByID(id string) (HWDevice, bool) {
	for _, d := range c.Devices {
		if d.ID == id {
			return d, true
		}
	}
	return HWDevice{}, false
}

// ObjectByID returns the object with given ID.
// Return false if not found.
func (c LocalConfiguration) ObjectByID(id string) (Object, bool) {
	for _, x := range c.Objects {
		if x.ID == id {
			return x, true
		}
	}
	return Object{}, false
}

// Validate the given configuration, returning nil on ok,
// or an error upon validation issues.
func (c LocalConfiguration) Validate() error {
	for _, d := range c.Devices {
		if err := d.Validate(); err != nil {
			return maskAny(err)
		}
	}
	for _, o := range c.Objects {
		if err := o.Validate(); err != nil {
			return maskAny(err)
		}
		for pinID, ps := range o.Pins {
			for _, p := range ps {
				if _, found := c.DeviceByID(p.DeviceID); !found {
					return errors.Wrapf(ValidationError, "Device '%s' not found in pin '%s' in object '%s'", p.DeviceID, pinID, o.ID)
				}
			}

		}
	}
	return nil
}
