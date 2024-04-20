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
	"sync"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/pkg/service/bridge"
)

type binkyCarSensor struct {
	log        zerolog.Logger
	mutex      sync.Mutex
	onActive   func()
	config     model.Device
	bus        bridge.I2CBus
	address    byte
	outputData []byte
	pins       uint
}

const (
	// Register addresses
	RegVersionMajor   = 0x00 // No input, returns 1 version
	RegVersionMinor   = 0x01 // No input, returns 1 version
	RegVersionPatch   = 0x02 // No input, returns 1 version
	RegCarSensorCount = 0x03 // No input, returns 1 byte giving the number of detected car sensor bits (0..8)
	RegI2COutputCount = 0x04 // No input, returns 1 byte giving the number of detected I2C binary output pins (0, 8, 16, ..., 256)
	RegCarSensorState = 0x10 // No input, returns 1 byte with 8-bit car detection sensor state
	RegOutput         = 0x20 // 1 byte input, targeting 8 on-pcb output pins
	RegOutputI2C0     = 0x21 // 1 byte input, targeting 8 output pins on PCF8574 output device 0
	RegOutputI2C1     = 0x22 // 1 byte input, targeting 8 output pins on PCF8574 output device 1
	RegOutputI2C2     = 0x23 // 1 byte input, targeting 8 output pins on PCF8574 output device 2
	RegOutputI2C3     = 0x24 // 1 byte input, targeting 8 output pins on PCF8574 output device 3
	RegOutputI2C4     = 0x25 // 1 byte input, targeting 8 output pins on PCF8574 output device 4
	RegOutputI2C5     = 0x26 // 1 byte input, targeting 8 output pins on PCF8574 output device 5
	RegOutputI2C6     = 0x27 // 1 byte input, targeting 8 output pins on PCF8574 output device 6
	RegOutputI2C7     = 0x28 // 1 byte input, targeting 8 output pins on PCF8574 output device 7
	RegConfigurePWM0  = 0x30 // 1 byte input, pwm-value (0-256) of pin 0
	RegConfigurePWM1  = 0x31 // 1 byte input, pwm-value (0-256) of pin 1
	RegConfigurePWM2  = 0x32 // 1 byte input, pwm-value (0-256) of pin 2
	RegConfigurePWM3  = 0x33 // 1 byte input, pwm-value (0-256) of pin 3
	RegConfigurePWM4  = 0x34 // 1 byte input, pwm-value (0-256) of pin 4
	RegConfigurePWM5  = 0x35 // 1 byte input, pwm-value (0-256) of pin 5
	RegConfigurePWM6  = 0x36 // 1 byte input, pwm-value (0-256) of pin 6
	RegConfigurePWM7  = 0x37 // 1 byte input, pwm-value (0-256) of pin 7
)

// newBinkyCarSensor creates a GPIO instance for a BinkyCarSensor device with given config.
func newBinkyCarSensor(log zerolog.Logger, config model.Device, bus bridge.I2CBus, onActive func()) (GPIO, error) {
	if config.Type != model.DeviceTypeBinkyCarSensor {
		return nil, model.InvalidArgument("Invalid device type '%s'", string(config.Type))
	}
	address, err := parseAddress(config.Address)
	if err != nil {
		return nil, err
	}
	return &binkyCarSensor{
		log:        log,
		onActive:   onActive,
		config:     config,
		bus:        bus,
		address:    byte(address),
		outputData: make([]byte, 9),
		pins:       8,
	}, nil
}

// Configure is called once to put the device in the desired state.
func (d *binkyCarSensor) Configure(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.onActive()
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		versionMajor, err := dev.ReadByteReg(RegVersionMajor)
		if err != nil {
			return err
		}
		versionMinor, err := dev.ReadByteReg(RegVersionMinor)
		if err != nil {
			return err
		}
		versionPatch, err := dev.ReadByteReg(RegVersionPatch)
		if err != nil {
			return err
		}

		outputBitCount, err := dev.ReadByteReg(RegI2COutputCount)
		if err != nil {
			return err
		}
		sensorBitCount, err := dev.ReadByteReg(RegCarSensorCount)
		if err != nil {
			return err
		}
		d.pins = 16 + uint(outputBitCount) // 8 car sensor bits, 8 on-print output pins, X i2c output pins

		d.log.Info().Msgf("BinkyCarSensor version=%d.%d.%d carSensorBits=%d i2cOutputBits=%d\n", versionMajor, versionMinor, versionPatch, sensorBitCount, outputBitCount)

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
	for i := range d.outputData {
		d.outputData[i] = 0
	}
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		// Disable output
		if err := dev.WriteByteReg(RegOutput, 0); err != nil {
			return err
		}
		for i := uint8(0); i < 8; i++ {
			if err := dev.WriteByteReg(RegOutputI2C0+i, 0); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// PinCount returns the number of pins of the device
func (d *binkyCarSensor) PinCount() uint {
	return d.pins
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
	if pin < 9 {
		return PinDirectionInput, nil
	}
	return PinDirectionOutput, nil
}

// Set the pin at given index (1...) to the given value
func (d *binkyCarSensor) Set(ctx context.Context, pin model.DeviceIndex, value bool) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if pin < 8 {
		// First 8 bits are input only
		return errors.Wrapf(InvalidDirectionError, "pin %d has direction input", pin)
	}
	index, mask, err := d.bitMask(pin)
	if err != nil {
		return err
	}
	outputData := d.outputData[index]
	if value {
		outputData |= mask
	} else {
		outputData &= ^mask
	}
	if d.outputData[index] != outputData {
		d.log.Debug().
			Uint8("index", index).
			Uint8("old", d.outputData[index]).
			Uint8("new", outputData).
			Msg("Setting new output value")
		d.outputData[index] = outputData
	}
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		// Set output
		if err := dev.WriteByteReg(RegOutput+index, d.outputData[index]); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// Get the pin at given index (1...)
func (d *binkyCarSensor) Get(ctx context.Context, pin model.DeviceIndex) (bool, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	_, mask, err := d.bitMask(pin)
	if err != nil {
		return false, err
	}
	if pin > 8 {
		// Only first 8 bits are input
		return false, errors.Wrapf(InvalidPinError, "Pin must be between 1 and 8, got %d", pin)
	}
	var value uint8
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		var err error
		value, err = dev.ReadByteReg(RegCarSensorState)
		return err
	}); err != nil {
		return false, err
	}
	return mask&value != 0, nil
}

// PWMPinCount returns the number of PWM output pins of the device
func (d *binkyCarSensor) PWMPinCount() int {
	return 8
}

// MaxPWMValue returns the maximum valid value for onValue or offValue.
func (d *binkyCarSensor) MaxPWMValue() uint32 {
	return 4095
}

// SetPWM the output at given index (1...) to the given value
func (d *binkyCarSensor) SetPWM(ctx context.Context, output model.DeviceIndex, onValue, offValue uint32, enabled bool) error {
	ioIndex := byte(output - 1)
	value := uint8(offValue / 16)
	d.log.Debug().
		Uint8("ioIndex", ioIndex).
		Uint8("value", value).
		Msg("SetPWM")
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		if err := dev.WriteByteReg(RegConfigurePWM0+ioIndex, value); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil

}

// GetPWM the output at given index (1...)
// Returns onValue,offValue,enabled,error
func (d *binkyCarSensor) GetPWM(ctx context.Context, output model.DeviceIndex) (uint32, uint32, bool, error) {
	// Not implemented
	return 0, 0, false, nil
}

// bitMask calculates a bit map (bit set for the given pin) and the corresponding
// register offset (0..8)
func (d *binkyCarSensor) bitMask(pin model.DeviceIndex) (index, mask byte, err error) {
	if pin < 1 || pin > model.DeviceIndex(d.pins) {
		return 0, 0, errors.Wrapf(InvalidPinError, "Pin must be between 1 and %d, got %d", d.pins, pin)
	}
	p := pin - 1
	index = byte(p>>3) - 1
	mask = 1 << (p & 0x07)
	return index, mask, nil
}
