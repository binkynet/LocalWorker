package devices

import (
	"context"

	"github.com/pkg/errors"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/LocalWorker/service/bridge"
)

type mcp23017 struct {
	config  model.Device
	bus     *bridge.I2CBus
	address byte
	iodir   []byte
	value   []byte
}

const (
	// Registry addresses with IOCON.BANK=0
	mcp23017RegIODIRA   = 0x00
	mcp23017RegIODIRB   = 0x01
	mcp23017RegIPOLA    = 0x02
	mcp23017RegIPOLB    = 0x03
	mcp23017RegGPINTENA = 0x04
	mcp23017RegGPINTENB = 0x05
	mcp23017RegDEFVALA  = 0x06
	mcp23017RegDEFVALB  = 0x07
	mcp23017RegINTCONA  = 0x08
	mcp23017RegINTCONB  = 0x09
	mcp23017RegIOCON    = 0x0a
	mcp23017RegGPPUA    = 0x0c
	mcp23017RegGPPUB    = 0x0d
	mcp23017RegINTFA    = 0x0e
	mcp23017RegINTFB    = 0x0f
	mcp23017RegINTCAPA  = 0x10
	mcp23017RegINTCAPB  = 0x11
	mcp23017RegGPIOA    = 0x12
	mcp23017RegGPIOB    = 0x13
	mcp23017RegOLATA    = 0x14
	mcp23017RegOLATB    = 0x15
)

// newMcp23017 creates a GPIO instance for a mcp23017 device with given config.
func newMcp23017(config model.Device, bus *bridge.I2CBus) (GPIO, error) {
	if config.Type != model.DeviceTypeMCP23017 {
		return nil, errors.Wrapf(model.ValidationError, "Invalid device type '%s'", string(config.Type))
	}
	address, err := parseAddress(config.Address)
	if err != nil {
		return nil, maskAny(err)
	}
	return &mcp23017{
		config:  config,
		bus:     bus,
		address: byte(address),
		iodir:   []byte{0xff, 0xff},
		value:   []byte{0, 0},
	}, nil
}

// Configure is called once to put the device in the desired state.
func (d *mcp23017) Configure(ctx context.Context) error {
	d.iodir[0] = 0xff
	d.iodir[1] = 0xff
	if err := d.bus.WriteByteReg(d.address, mcp23017RegIOCON, 0x20); err != nil {
		return maskAny(err)
	}
	if err := d.bus.WriteByteReg(d.address, mcp23017RegIODIRA, d.iodir[0]); err != nil {
		return maskAny(err)
	}
	if err := d.bus.WriteByteReg(d.address, mcp23017RegIODIRB, d.iodir[1]); err != nil {
		return maskAny(err)
	}
	return nil
}

// Close brings the device back to a safe state.
func (d *mcp23017) Close() error {
	// Restore all to input
	d.iodir[0] = 0xff
	d.iodir[1] = 0xff
	if err := d.bus.WriteByteReg(d.address, mcp23017RegIODIRA, d.iodir[0]); err != nil {
		return maskAny(err)
	}
	if err := d.bus.WriteByteReg(d.address, mcp23017RegIODIRB, d.iodir[1]); err != nil {
		return maskAny(err)
	}
	return nil
}

// PinCount returns the number of pins of the device
func (d *mcp23017) PinCount() uint {
	return 16
}

// Set the direction of the pin at given index (1...)
func (d *mcp23017) SetDirection(ctx context.Context, pin model.DeviceIndex, direction PinDirection) error {
	mask, regOffset, err := d.bitMask(pin)
	if err != nil {
		return maskAny(err)
	}
	if direction == PinDirectionInput {
		d.iodir[regOffset] |= mask
	} else {
		d.iodir[regOffset] &= ^mask
	}
	if err := d.bus.WriteByteReg(d.address, mcp23017RegIODIRA+regOffset, d.iodir[regOffset]); err != nil {
		return maskAny(err)
	}
	return nil
}

// Get the direction of the pin at given index (1...)
func (d *mcp23017) GetDirection(ctx context.Context, pin model.DeviceIndex) (PinDirection, error) {
	mask, regOffset, err := d.bitMask(pin)
	if err != nil {
		return PinDirectionInput, maskAny(err)
	}
	value, err := d.bus.ReadByteReg(d.address, mcp23017RegIODIRA+regOffset)
	if err != nil {
		return PinDirectionInput, maskAny(err)
	}
	if value&mask == 0 {
		return PinDirectionOutput, nil
	}
	return PinDirectionInput, nil
}

// Set the pin at given index (1...) to the given value
func (d *mcp23017) Set(ctx context.Context, pin model.DeviceIndex, value bool) error {
	mask, regOffset, err := d.bitMask(pin)
	if err != nil {
		return maskAny(err)
	}
	if d.iodir[regOffset]&mask == 0 {
		// IODIR == output
		if value {
			d.value[regOffset] |= mask
		} else {
			d.value[regOffset] &= ^mask
		}
		if err := d.bus.WriteByteReg(d.address, mcp23017RegGPIOA+regOffset, d.value[regOffset]); err != nil {
			return maskAny(err)
		}
		return nil
	}
	return errors.Wrapf(InvalidDirectionError, "pin %d has direction input", pin)
}

// Set the pin at given index (1...)
func (d *mcp23017) Get(ctx context.Context, pin model.DeviceIndex) (bool, error) {
	mask, regOffset, err := d.bitMask(pin)
	if err != nil {
		return false, maskAny(err)
	}
	value, err := d.bus.ReadByteReg(d.address, mcp23017RegGPIOA+regOffset)
	if err != nil {
		return false, maskAny(err)
	}
	return mask&value != 0, nil
}

// bitMask calculates a bit map (bit set for the given pin) and the corresponding
// register offset (0, 1)
func (d *mcp23017) bitMask(pin model.DeviceIndex) (mask, regOffset byte, err error) {
	if pin < 1 || pin > 16 {
		return 0, 0, errors.Wrapf(InvalidPinError, "Pin must be between 1 and 16, got %d", pin)
	}
	if pin <= 8 {
		mask = 1 << uint(pin-1)
		regOffset = 0
	} else {
		mask = 1 << uint(pin-9)
		regOffset = 1
	}
	return mask, regOffset, nil
}
