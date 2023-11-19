// Copyright 2023 Ewout Prangsma
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
	"fmt"
	"sync"

	"github.com/pkg/errors"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/pkg/service/bridge"
)

type binkyCarSensor struct {
	mutex      sync.Mutex
	onActive   func()
	config     model.Device
	bus        bridge.I2CBus
	address    byte
	outputData byte
}

const (
	// Registry addresses
	bcsRegData    = 0x00
	bcsRegVersion = 0x01
)

// newBinkyCarSensor creates a GPIO instance for a BinkyCarSensor device with given config.
func newBinkyCarSensor(config model.Device, bus bridge.I2CBus, onActive func()) (GPIO, error) {
	if config.Type != model.DeviceTypeBinkyCarSensor {
		return nil, model.InvalidArgument("Invalid device type '%s'", string(config.Type))
	}
	address, err := parseAddress(config.Address)
	if err != nil {
		return nil, err
	}
	return &binkyCarSensor{
		onActive:   onActive,
		config:     config,
		bus:        bus,
		address:    byte(address),
		outputData: 0,
	}, nil
}

// Configure is called once to put the device in the desired state.
func (d *binkyCarSensor) Configure(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.onActive()
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		if version, err := dev.ReadByteReg(bcsRegVersion); err != nil {
			return err
		} else {
			fmt.Println("version: 0x%x", version)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// Close brings the device back to a safe state.
func (d *binkyCarSensor) Close(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Restore all to input
	d.onActive()
	d.outputData = 0
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		// Disable output
		if err := dev.WriteByteReg(bcsRegData, 0); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// PinCount returns the number of pins of the device
func (d *binkyCarSensor) PinCount() uint {
	return 8
}

// Set the direction of the pin at given index (1...)
// This is a no-op
func (d *binkyCarSensor) SetDirection(ctx context.Context, pin model.DeviceIndex, direction PinDirection) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.onActive()
	return nil
}

// Get the direction of the pin at given index (1...)
func (d *binkyCarSensor) GetDirection(ctx context.Context, pin model.DeviceIndex) (PinDirection, error) {
	return PinDirectionInput, nil
}

// Set the pin at given index (1...) to the given value
func (d *binkyCarSensor) Set(ctx context.Context, pin model.DeviceIndex, value bool) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	mask, err := d.bitMask(pin)
	if err != nil {
		return err
	}
	if value {
		d.outputData |= mask
	} else {
		d.outputData &= ^mask
	}
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		// Set output
		if err := dev.WriteByteReg(bcsRegData, d.outputData); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// TODO

	return errors.Wrapf(InvalidDirectionError, "pin %d has direction input", pin)
}

// Set the pin at given index (1...)
func (d *binkyCarSensor) Get(ctx context.Context, pin model.DeviceIndex) (bool, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	mask, err := d.bitMask(pin)
	if err != nil {
		return false, err
	}
	var value uint8
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		var err error
		value, err = dev.ReadByteReg(bcsRegData)
		return err
	}); err != nil {
		return false, err
	}
	return mask&value != 0, nil
}

// bitMask calculates a bit map (bit set for the given pin) and the corresponding
// register offset (0, 1)
func (d *binkyCarSensor) bitMask(pin model.DeviceIndex) (mask byte, err error) {
	if pin < 1 || pin > 8 {
		return 0, errors.Wrapf(InvalidPinError, "Pin must be between 1 and 8, got %d", pin)
	}
	mask = 1 << uint(pin-1)
	return mask, nil
}
