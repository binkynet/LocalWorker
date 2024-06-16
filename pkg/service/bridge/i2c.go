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
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/binkynet/LocalWorker/pkg/service/util"
	"github.com/ecc1/gpio"
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
	location             string
	devices              map[uint8]*i2cDevice
	queue                chan func()
	sclPin               int
	tryRecoverFromLockup bool
}

const (
	I2C_RECOVER_NUM_CLOCKS = 10    /* # clock cycles for recovery  */
	I2C_RECOVER_CLOCK_FREQ = 50000 /* clock frequency for recovery */

	I2C_RECOVER_CLOCK_DELAY_US = (1000000 / (2 * I2C_RECOVER_CLOCK_FREQ))
)

// NewI2CBus returns accessors the the I2C bus at the given location.
func NewI2CBus(location string, sclPin int) (I2CBus, error) {
	b := &i2cBus{
		location:             location,
		devices:              make(map[uint8]*i2cDevice),
		queue:                make(chan func()),
		sclPin:               sclPin,
		tryRecoverFromLockup: false,
	}
	go b.queueProcessor(context.Background())
	if b.tryRecoverFromLockup {
		if err := b.recoverFromLockup(); err != nil {
			return nil, fmt.Errorf("failed to recover bus at startup: %w", err)
		}
		time.Sleep(time.Second * 2)
	}
	return b, nil
}

// Execute an option on the bus.
func (b *i2cBus) Execute(ctx context.Context, address uint8, op func(context.Context, I2CDevice) error) error {
	// Prepare result
	l := util.SpinLock{}
	done := false
	var result error

	// Prepare request
	req := func() {
		// Execute actual operation
		err := b.execute(ctx, address, op)

		// Store result
		l.Lock()
		result = err
		done = true
		l.Unlock()
	}

	// Put request in queue
	select {
	case b.queue <- req:
		// Request is on the queue
	case <-ctx.Done():
		// Context canceled
		return ctx.Err()
	}

	// Wait until result is available
	for {
		l.Lock()
		isDone := done
		l.Unlock()

		if isDone {
			return result
		}
	}
}

// Process bus requests from the queue until the given context is canceled.
func (b *i2cBus) queueProcessor(ctx context.Context) {
	// Ensure we're always using the same OS thread
	runtime.LockOSThread()

	// Process the queue
	for {
		select {
		case req, ok := <-b.queue:
			if ok {
				// Execute the given request
				req()
			} else {
				// Queue closed
				return
			}
		case <-ctx.Done():
			// Context canceled
			return
		}
	}
}

// Execute an option on the bus.
func (b *i2cBus) execute(ctx context.Context, address uint8, op func(context.Context, I2CDevice) error) error {
	i2cExecuteCounters.WithLabelValues(strconv.Itoa(int(address))).Inc()

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

		// Device call failed, close all devices
		for _, d := range b.devices {
			d.closeFile()
		}
		clear(b.devices)

		// Perform recovery (if configured)
		if b.tryRecoverFromLockup {
			i2cRecoveryAttemptsTotal.Inc()
			if err := b.recoverFromLockup(); err != nil {
				i2cRecoveryFailedTotal.Inc()
				return fmt.Errorf("i2c recovery failed: %w", err)
			}
			i2cRecoverySucceededTotal.Inc()
		} else {
			i2cRecoverySkippedTotal.Inc()
		}
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
	done := false
	l := util.SpinLock{}
	var ae aerr.AggregateError
	b.queue <- func() {
		// Set done on exit
		defer func() {
			l.Lock()
			done = true
			l.Unlock()
		}()

		// Capture all devices
		devices := make([]*i2cDevice, 0, len(b.devices))
		for _, d := range b.devices {
			devices = append(devices, d)
		}

		// Close all collected devices
		for _, d := range devices {
			if err := d.closeFile(); err != nil {
				ae.Add(err)
			}
			delete(b.devices, d.address)
		}
	}

	// Wait until ready
	for {
		l.Lock()
		isDone := done
		l.Unlock()
		if isDone {
			return ae.AsError()
		}
	}
}

// Try to recover the i2c bus from lockup.
func (b *i2cBus) recoverFromLockup() error {
	fmt.Println("Performing i2c recovery ...")
	activeLow := true // was false
	initialValue := true
	scl, err := gpio.Output(b.sclPin, activeLow, initialValue)
	if err != nil {
		return fmt.Errorf("failed to set scl pin to output: %w", err)
	}
	for i := 0; i < I2C_RECOVER_NUM_CLOCKS; i++ {
		time.Sleep(time.Microsecond * I2C_RECOVER_CLOCK_DELAY_US)
		if err := scl.Write(false); err != nil {
			return fmt.Errorf("failed to lower scl during i2c recovery: %w", err)
		}
		time.Sleep(time.Microsecond * I2C_RECOVER_CLOCK_DELAY_US)
		if err := scl.Write(true); err != nil {
			return fmt.Errorf("failed to raise scl during i2c recovery: %w", err)
		}
	}
	// Reset pin to be input
	if _, err := gpio.Input(b.sclPin, activeLow); err != nil {
		return fmt.Errorf("failed to reset scl pin to input: %w", err)
	}
	// Unexport the pin
	unexportPath := "/sys/class/gpio/unexport"
	unexportContent := strconv.Itoa(b.sclPin)
	if err := os.WriteFile(unexportPath, []byte(unexportContent), 0644); err != nil {
		return fmt.Errorf("failed to unexport scl pin to input: %w", err)
	}

	fmt.Println("Performed i2c recovery.")
	return nil
}
