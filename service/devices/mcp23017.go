package devices

import (
	"context"

	"github.com/pkg/errors"

	"github.com/binkynet/LocalWorker/model"
	"github.com/binkynet/LocalWorker/service/bridge"
)

type mcp23017 struct {
	config  model.HWDevice
	bus     *bridge.I2CBus
	address int
}

// newMcp23017 creates a GPIO instance for a mcp23017 device with given config.
func newMcp23017(config model.HWDevice, bus *bridge.I2CBus) (GPIO, error) {
	if config.Type != model.HWDeviceTypeMCP23017 {
		return nil, errors.Wrapf(model.ValidationError, "Invalid device type '%s'", string(config.Type))
	}
	address, err := parseAddress(config.Address)
	if err != nil {
		return nil, maskAny(err)
	}
	return &mcp23017{
		config:  config,
		bus:     bus,
		address: address,
	}, nil
}

// Configure is called once to put the device in the desired state.
func (d *mcp23017) Configure(ctx context.Context) error {
	return nil
}

// Close brings the device back to a safe state.
func (d *mcp23017) Close() error {
	return nil
}

// PinCount returns the number of pins of the device
func (d *mcp23017) PinCount() int {
	return 16
}

// Set the pin at given index (1...) to the given value
func (d *mcp23017) Set(ctx context.Context, pin int, value bool) error {

}

// Set the pin at given index (1...)
func (d *mcp23017) Get(ctx context.Context, pin int) (bool, error) {

}
