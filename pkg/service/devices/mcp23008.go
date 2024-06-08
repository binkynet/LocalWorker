// Copyright 2020 Ewout Prangsma
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author Ewout Prangsma
//

package devices

import (
	"context"

	"github.com/pkg/errors"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/pkg/service/bridge"
)

type mcp23008 struct {
	onActive func()
	config   model.Device
	bus      bridge.I2CBus
	address  byte
	iodir    byte
	value    byte
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
func newMcp23008(config model.Device, bus bridge.I2CBus, onActive func()) (GPIO, error) {
	if config.Type != model.DeviceTypeMCP23008 {
		return nil, model.InvalidArgument("Invalid device type '%s'", string(config.Type))
	}
	address, err := parseAddress(config.Address)
	if err != nil {
		return nil, err
	}
	return &mcp23008{
		onActive: onActive,
		config:   config,
		bus:      bus,
		address:  byte(address),
		iodir:    0xff,
		value:    0xff, // Default high
	}, nil
}

// Configure is called once to put the device in the desired state.
func (d *mcp23008) Configure(ctx context.Context) error {
	d.onActive()
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		d.iodir = 0xff
		if err := dev.WriteByteReg(mcp23008RegIOCON, 0x20); err != nil {
			return err
		}
		if err := dev.WriteByteReg(mcp23008RegIODIR, d.iodir); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// Close brings the device back to a safe state.
func (d *mcp23008) Close(ctx context.Context) error {
	d.onActive()
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		// Restore all to input
		d.iodir = 0xff
		if err := dev.WriteByteReg(mcp23008RegIODIR, d.iodir); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
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
		return err
	}
	d.onActive()
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		if direction == PinDirectionInput {
			d.iodir |= mask
		} else {
			d.iodir &= ^mask
		}
		if err := dev.WriteByteReg(mcp23008RegIODIR, d.iodir); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// Get the direction of the pin at given index (1...)
func (d *mcp23008) GetDirection(ctx context.Context, pin model.DeviceIndex) (PinDirection, error) {
	mask, err := d.bitMask(pin)
	if err != nil {
		return PinDirectionInput, err
	}
	var value uint8
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		var err error
		value, err = dev.ReadByteReg(mcp23008RegIODIR)
		return err
	}); err != nil {
		return PinDirectionInput, err
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
		return err
	}
	if d.iodir&mask == 0 {
		// IODIR == output
		d.onActive()
		if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
			if value {
				d.value |= mask
			} else {
				d.value &= ^mask
			}
			if err := dev.WriteByteReg(mcp23008RegGPIO, d.value); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	}
	return errors.Wrapf(InvalidDirectionError, "pin %d has direction input", pin)
}

// Set the pin at given index (1...)
func (d *mcp23008) Get(ctx context.Context, pin model.DeviceIndex) (bool, error) {
	mask, err := d.bitMask(pin)
	if err != nil {
		return false, err
	}
	var value uint8
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		var err error
		value, err = dev.ReadByteReg(mcp23008RegGPIO)
		return err
	}); err != nil {
		return false, err
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
