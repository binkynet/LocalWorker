// Copyright 2021 Ewout Prangsma
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

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/pkg/service/bridge"
	"github.com/pkg/errors"
)

type pcf8574 struct {
	onActive  func()
	config    model.Device
	bus       bridge.I2CBus
	address   byte // Address for writing. Address for reading is 1 higher
	direction byte // 1/0 per bit means read/write
	output    byte // 1/0 bit per pin
}

// newPCF8574 creates a GPIO instance for a pcf8574 device with given config.
func newPCF8574(config model.Device, bus bridge.I2CBus, onActive func()) (GPIO, error) {
	if config.Type != model.DeviceTypePCF8574 {
		return nil, model.InvalidArgument("Invalid device type '%s'", string(config.Type))
	}
	address, err := parseAddress(config.Address)
	if err != nil {
		return nil, err
	}
	return &pcf8574{
		onActive:  onActive,
		config:    config,
		bus:       bus,
		address:   byte(address),
		direction: 0xff, // All read
		output:    0,    // All 0
	}, nil
}

// Configure is called once to put the device in the desired state.
func (d *pcf8574) Configure(ctx context.Context) error {
	d.onActive()
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		// Initialize to all INPUT HIGH
		d.direction = 0xff
		d.output = 0
		return dev.WriteByte(0xff)
	}); err != nil {
		return err
	}
	return nil
}

// Close brings the device back to a safe state.
func (d *pcf8574) Close(ctx context.Context) error {
	d.onActive()
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		// Set all to INPUT HIGH
		d.direction = 0xff
		d.output = 0
		return dev.WriteByte(0xff)
	}); err != nil {
		return err
	}
	return nil
}

// PinCount returns the number of pins of the device
func (d *pcf8574) PinCount() uint {
	return 8
}

// Set the direction of the pin at given index (1...)
func (d *pcf8574) SetDirection(ctx context.Context, index model.DeviceIndex, direction PinDirection) error {
	// Calculate direction mask
	mask, err := d.bitMask(index)
	if err != nil {
		return err
	}

	// Update state
	d.onActive()

	// Set initial value
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		d.output &= ^mask
		if direction == PinDirectionInput {
			d.direction |= mask
		} else {
			d.direction &= ^mask
		}
		return dev.WriteByte(d.mergeDirectionAndOutput())
	}); err != nil {
		return err
	}

	return nil
}

// Get the direction of the pin at given index (1...)
func (d *pcf8574) GetDirection(ctx context.Context, index model.DeviceIndex) (PinDirection, error) {
	// Calculate direction mask
	mask, err := d.bitMask(index)
	if err != nil {
		return 0, err
	}

	if d.direction&mask != 0 {
		return PinDirectionInput, nil
	} else {
		return PinDirectionOutput, nil
	}
}

// Set the pin at given index (1...) to the given value
func (d *pcf8574) Set(ctx context.Context, index model.DeviceIndex, value bool) error {
	// Calculate direction mask
	mask, err := d.bitMask(index)
	if err != nil {
		return err
	}

	// Check direction
	if d.direction&mask != 0 {
		return errors.Wrapf(InvalidDirectionError, "pin %d has direction input", index)
	}

	// Update state
	d.onActive()

	// Set updated value
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		if value {
			d.output |= mask
		} else {
			d.output &= ^mask
		}
		return dev.WriteByte(d.mergeDirectionAndOutput())
	}); err != nil {
		return err
	}

	return nil
}

// Set the pin at given index (1...)
func (d *pcf8574) Get(ctx context.Context, index model.DeviceIndex) (bool, error) {
	// Calculate direction mask
	mask, err := d.bitMask(index)
	if err != nil {
		return false, err
	}

	// Read current value
	var x uint8
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		var err error
		x, err = dev.ReadByte()
		return err
	}); err != nil {
		return false, err
	}

	return x&mask != 0, nil
}

// bitMask calculates a bit map (bit set for the given pin)
func (d *pcf8574) bitMask(pin model.DeviceIndex) (mask byte, err error) {
	if pin < 1 || pin > 8 {
		return 0, errors.Wrapf(InvalidPinError, "Pin must be between 1 and 8, got %d", pin)
	}
	mask = 1 << uint(pin-1)
	return mask, nil
}

// mergeDirectionAndOutput creates the value to write to the device that merges
// direction & output.
// Per bit the following applies:
// - Direction is input -> bit is set to 1
// - Direction is output -> bit is set to output bit
func (d *pcf8574) mergeDirectionAndOutput() byte {
	return d.direction | d.output
}
