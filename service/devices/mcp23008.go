package devices

import (
	"context"

	"github.com/pkg/errors"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/LocalWorker/service/bridge"
)

type mcp23008 struct {
	config  model.Device
	bus     *bridge.I2CBus
	address byte
	iodir   byte
	value   byte
}

const (
	// Registry addresses with IOCON.BANK=0
	mcp23008RegIODIR   = 0x00
	mcp23008RegIPOL    = 0x01
	mcp23008RegGPINTEN = 0x02
	mcp23008RegDEFVAL  = 0x03
	mcp23008RegINTCON  = 0x04
	mcp23008RegIOCON   = 0x05
	mcp23008RegGPPU    = 0x06
	mcp23008RegINTF    = 0x07
	mcp23008RegINTCAP  = 0x08
	mcp23008RegGPIO    = 0x09
	mcp23008RegOLAT    = 0x0a
)

// newMcp23008 creates a GPIO instance for a mcp23008 device with given config.
func newMcp23008(config model.Device, bus *bridge.I2CBus) (GPIO, error) {
	if config.Type != model.DeviceTypeMCP23008 {
		return nil, errors.Wrapf(model.ValidationError, "Invalid device type '%s'", string(config.Type))
	}
	address, err := parseAddress(config.Address)
	if err != nil {
		return nil, maskAny(err)
	}
	return &mcp23008{
		config:  config,
		bus:     bus,
		address: byte(address),
		iodir:   0xff,
		value:   0,
	}, nil
}

// Configure is called once to put the device in the desired state.
func (d *mcp23008) Configure(ctx context.Context) error {
	d.iodir = 0xff
	if err := d.bus.WriteByteReg(d.address, mcp23008RegIOCON, 0x20); err != nil {
		return maskAny(err)
	}
	if err := d.bus.WriteByteReg(d.address, mcp23008RegIODIR, d.iodir); err != nil {
		return maskAny(err)
	}
	return nil
}

// Close brings the device back to a safe state.
func (d *mcp23008) Close() error {
	// Restore all to input
	d.iodir = 0xff
	if err := d.bus.WriteByteReg(d.address, mcp23008RegIODIR, d.iodir); err != nil {
		return maskAny(err)
	}
	return nil
}

// PinCount returns the number of pins of the device
func (d *mcp23008) PinCount() uint {
	return 8
}

// Set the direction of the pin at given index (1...)
func (d *mcp23008) SetDirection(ctx context.Context, pin model.DeviceIndex, direction PinDirection) error {
	mask, err := d.bitMask(pin)
	if err != nil {
		return maskAny(err)
	}
	if direction == PinDirectionInput {
		d.iodir |= mask
	} else {
		d.iodir &= ^mask
	}
	if err := d.bus.WriteByteReg(d.address, mcp23008RegIODIR, d.iodir); err != nil {
		return maskAny(err)
	}
	return nil
}

// Get the direction of the pin at given index (1...)
func (d *mcp23008) GetDirection(ctx context.Context, pin model.DeviceIndex) (PinDirection, error) {
	mask, err := d.bitMask(pin)
	if err != nil {
		return PinDirectionInput, maskAny(err)
	}
	value, err := d.bus.ReadByteReg(d.address, mcp23008RegIODIR)
	if err != nil {
		return PinDirectionInput, maskAny(err)
	}
	if value&mask == 0 {
		return PinDirectionOutput, nil
	}
	return PinDirectionInput, nil
}

// Set the pin at given index (1...) to the given value
func (d *mcp23008) Set(ctx context.Context, pin model.DeviceIndex, value bool) error {
	mask, err := d.bitMask(pin)
	if err != nil {
		return maskAny(err)
	}
	if d.iodir&mask == 0 {
		// IODIR == output
		if value {
			d.value |= mask
		} else {
			d.value &= ^mask
		}
		if err := d.bus.WriteByteReg(d.address, mcp23008RegGPIO, d.value); err != nil {
			return maskAny(err)
		}
		return nil
	}
	return errors.Wrapf(InvalidDirectionError, "pin %d has direction input", pin)
}

// Set the pin at given index (1...)
func (d *mcp23008) Get(ctx context.Context, pin model.DeviceIndex) (bool, error) {
	mask, err := d.bitMask(pin)
	if err != nil {
		return false, maskAny(err)
	}
	value, err := d.bus.ReadByteReg(d.address, mcp23008RegGPIO)
	if err != nil {
		return false, maskAny(err)
	}
	return mask&value != 0, nil
}

// bitMask calculates a bit map (bit set for the given pin) and the corresponding
// register offset (0, 1)
func (d *mcp23008) bitMask(pin model.DeviceIndex) (mask byte, err error) {
	if pin < 1 || pin > 8 {
		return 0, errors.Wrapf(InvalidPinError, "Pin must be between 1 and 8, got %d", pin)
	}
	mask = 1 << uint(pin-1)
	return mask, nil
}
