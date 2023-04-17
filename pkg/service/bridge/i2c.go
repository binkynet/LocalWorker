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

package bridge

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	aerr "github.com/ewoutp/go-aggregate-error"
)

type I2CBus interface {
	// Execute an option on the bus.
	Execute(ctx context.Context, address uint8, op func(ctx context.Context, dev I2CDevice) error) error
	// DetectSlaveAddresses probes the bus to detect available addresses.
	DetectSlaveAddresses() []byte
	// Close the bus and all devices on it
	Close() error
}

// I2CDevice communicates with a device on the I2C Bus that has a specific address.
type I2CDevice interface {
	// Read a byte from given register
	ReadByteReg(reg uint8) (uint8, error)
	// Write a byte to given register
	WriteByteReg(reg uint8, val uint8) (err error)
	// Read a byte from device
	ReadByte() (uint8, error)
	// Write a byte to device
	WriteByte(val uint8) (err error)
	// Read a block of data directly from the device (/dev/...)
	ReadDevice(data []byte) (err error)
	// Write a block of data directly to the device (/dev/...)
	WriteDevice(data []byte) (err error)
}

type i2cBus struct {
	mutex    sync.Mutex
	location string
	devices  map[uint8]*i2cDevice
}

// NewI2CBus returns accessors the the I2C bus at the given location.
func NewI2CBus(location string) (I2CBus, error) {
	b := &i2cBus{
		location: location,
		devices:  make(map[uint8]*i2cDevice),
	}
	return b, nil
}

// Execute an option on the bus.
func (b *i2cBus) Execute(ctx context.Context, address uint8, op func(context.Context, I2CDevice) error) error {
	i2cExecuteCounters.WithLabelValues(strconv.Itoa(int(address))).Inc()
	b.mutex.Lock()
	defer b.mutex.Unlock()

	var err error
	for attempt := 0; attempt < 2; attempt++ {
		// Open device
		var dev *i2cDevice
		dev, err = b.openDevice(address)
		if err != nil {
			return fmt.Errorf("openDevice(%d) failed: %w", address, err)
		}

		// Execute operation
		err = op(ctx, dev)
		if err == nil {
			// Success
			return nil
		}

		// Close dev
		dev.closeFile()
		delete(b.devices, address)
	}
	// Return error
	i2cExecuteErrorCounters.WithLabelValues(strconv.Itoa(int(address))).Inc()
	return fmt.Errorf("execute operation in i2c bus failed: %w", err)
}

// Open a connection to a device at the given address.
func (b *i2cBus) openDevice(address uint8) (*i2cDevice, error) {
	// Did we already open the device?
	if d, found := b.devices[address]; found {
		return d, nil
	}

	// Open new device
	d, err := newI2CDevice(b, b.location, address)
	if err != nil {
		return nil, err
	}

	// Register device
	b.devices[address] = d

	return d, nil
}

// DetectSlaveAddresses probes the bus to detect available addresses.
func (b *i2cBus) DetectSlaveAddresses() []byte {
	var result []byte
	for addr := uint8(1); addr < 128; addr++ {
		if d, err := newI2CDevice(b, b.location, addr); err == nil {
			if err := d.DetectDevice(); err == nil {
				result = append(result, addr)
			}
			d.closeFile()
		}
	}
	return result
}

// Close the bus and all devices on it
func (b *i2cBus) Close() error {
	// Collect all existing devices
	b.mutex.Lock()
	devices := make([]*i2cDevice, 0, len(b.devices))
	for _, d := range b.devices {
		devices = append(devices, d)
	}
	b.mutex.Unlock()

	// Close all collected devices
	var ae aerr.AggregateError
	for _, d := range devices {
		if err := d.closeFile(); err != nil {
			ae.Add(err)
		}
		delete(b.devices, d.address)
	}

	return ae.AsError()
}
