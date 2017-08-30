package model

import "github.com/pkg/errors"

// HWDevice holds configuration data for a specif hardward device.
// Typically a hardware device is attached to a bus.
type HWDevice struct {
	// Unique identifier of the device (instance)
	ID string `json:"id"`
	// Address is used to identify the device on a bus.
	Address string `json:"address"`
	// Type of the device
	Type HWDeviceType `json:"type"`
}

// HWDeviceType identifies a type of devices (typically chip name)
type HWDeviceType string

const (
	HWDeviceTypeMCP23017 = "mcp23017"
)

// Validate the given type, returning nil on ok,
// or an error upon validation issues.
func (t HWDeviceType) Validate() error {
	switch t {
	case HWDeviceTypeMCP23017:
		return nil
	default:
		return errors.Wrapf(ValidationError, "invalid device type '%s'", string(t))
	}
}

// Validate the given configuration, returning nil on ok,
// or an error upon validation issues.
func (d HWDevice) Validate() error {
	if d.ID == "" {
		return errors.Wrap(ValidationError, "ID is empty")
	}
	if err := d.Type.Validate(); err != nil {
		return errors.Wrapf(ValidationError, "Error in Type of '%s': %s", d.ID, err.Error())
	}
	if d.Address == "" {
		return errors.Wrapf(ValidationError, "Address of '%s' is empty", d.ID)
	}
	return nil
}
