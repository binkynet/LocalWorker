// Copyright 2022 Ewout Prangsma
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
	"time"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/pkg/service/bridge"
)

type ads1115 struct {
	mutex    sync.Mutex
	onActive func()
	config   model.Device
	bus      bridge.I2CBus
	address  byte
}

const (
	// Registry addresses
	ads1115RegConversion    = 0x00
	ads1115RegConfig        = 0x01
	ads1115RegLowThreshold  = 0x02
	ads1115RegHighThreshold = 0x03
)

const (
	// Config mode flags
	ADS1X15_REG_CONFIG_OS_MASK    = (0x8000) ///< OS Mask
	ADS1X15_REG_CONFIG_OS_SINGLE  = (0x8000) ///< Write: Set to start a single-conversion
	ADS1X15_REG_CONFIG_OS_BUSY    = (0x0000) ///< Read: Bit = 0 when conversion is in progress
	ADS1X15_REG_CONFIG_OS_NOTBUSY = (0x8000) ///< Read: Bit = 1 when device is not performing a conversion

	ADS1X15_REG_CONFIG_MUX_MASK     = (0x7000) ///< Mux Mask
	ADS1X15_REG_CONFIG_MUX_DIFF_0_1 = (0x0000) ///< Differential P = AIN0, N = AIN1 (default)
	ADS1X15_REG_CONFIG_MUX_DIFF_0_3 = (0x1000) ///< Differential P = AIN0, N = AIN3
	ADS1X15_REG_CONFIG_MUX_DIFF_1_3 = (0x2000) ///< Differential P = AIN1, N = AIN3
	ADS1X15_REG_CONFIG_MUX_DIFF_2_3 = (0x3000) ///< Differential P = AIN2, N = AIN3
	ADS1X15_REG_CONFIG_MUX_SINGLE_0 = (0x4000) ///< Single-ended AIN0
	ADS1X15_REG_CONFIG_MUX_SINGLE_1 = (0x5000) ///< Single-ended AIN1
	ADS1X15_REG_CONFIG_MUX_SINGLE_2 = (0x6000) ///< Single-ended AIN2
	ADS1X15_REG_CONFIG_MUX_SINGLE_3 = (0x7000) ///< Single-ended AIN3

	ADS1X15_REG_CONFIG_PGA_MASK   = (0x0E00) ///< PGA Mask
	ADS1X15_REG_CONFIG_PGA_6_144V = (0x0000) ///< +/-6.144V range = Gain 2/3
	ADS1X15_REG_CONFIG_PGA_4_096V = (0x0200) ///< +/-4.096V range = Gain 1
	ADS1X15_REG_CONFIG_PGA_2_048V = (0x0400) ///< +/-2.048V range = Gain 2 (default)
	ADS1X15_REG_CONFIG_PGA_1_024V = (0x0600) ///< +/-1.024V range = Gain 4
	ADS1X15_REG_CONFIG_PGA_0_512V = (0x0800) ///< +/-0.512V range = Gain 8
	ADS1X15_REG_CONFIG_PGA_0_256V = (0x0A00) ///< +/-0.256V range = Gain 16

	ADS1X15_REG_CONFIG_MODE_MASK   = (0x0100) ///< Mode Mask
	ADS1X15_REG_CONFIG_MODE_CONTIN = (0x0000) ///< Continuous conversion mode
	ADS1X15_REG_CONFIG_MODE_SINGLE = (0x0100) ///< Power-down single-shot mode (default)

	ADS1X15_REG_CONFIG_RATE_MASK = (0x00E0) ///< Data Rate Mask

	ADS1X15_REG_CONFIG_CMODE_MASK   = (0x0010) ///< CMode Mask
	ADS1X15_REG_CONFIG_CMODE_TRAD   = (0x0000) ///< Traditional comparator with hysteresis (default)
	ADS1X15_REG_CONFIG_CMODE_WINDOW = (0x0010) ///< Window comparator

	ADS1X15_REG_CONFIG_CPOL_MASK    = (0x0008) ///< CPol Mask
	ADS1X15_REG_CONFIG_CPOL_ACTVLOW = (0x0000) ///< ALERT/RDY pin is low when active (default)
	ADS1X15_REG_CONFIG_CPOL_ACTVHI  = (0x0008) ///< ALERT/RDY pin is high when active

	ADS1X15_REG_CONFIG_CLAT_MASK   = (0x0004) ///< Determines if ALERT/RDY pin latches once asserted
	ADS1X15_REG_CONFIG_CLAT_NONLAT = (0x0000) ///< Non-latching comparator (default)
	ADS1X15_REG_CONFIG_CLAT_LATCH  = (0x0004) ///< Latching comparator

	ADS1X15_REG_CONFIG_CQUE_MASK  = (0x0003) ///< CQue Mask
	ADS1X15_REG_CONFIG_CQUE_1CONV = (0x0000) ///< Assert ALERT/RDY after one conversions
	ADS1X15_REG_CONFIG_CQUE_2CONV = (0x0001) ///< Assert ALERT/RDY after two conversions
	ADS1X15_REG_CONFIG_CQUE_4CONV = (0x0002) ///< Assert ALERT/RDY after four conversions
	ADS1X15_REG_CONFIG_CQUE_NONE  = (0x0003) ///< Disable the comparator and put ALERT/RDY in high state (default)
)

var (
	MUX_BY_CHANNEL []uint16 = []uint16{
		ADS1X15_REG_CONFIG_MUX_SINGLE_0, ///< Single-ended AIN0
		ADS1X15_REG_CONFIG_MUX_SINGLE_1, ///< Single-ended AIN1
		ADS1X15_REG_CONFIG_MUX_SINGLE_2, ///< Single-ended AIN2
		ADS1X15_REG_CONFIG_MUX_SINGLE_3, ///< Single-ended AIN3
	} ///< MUX config by channel
)

const (
	// Data rates
	RATE_ADS1115_8SPS   = (0x0000) ///< 8 samples per second
	RATE_ADS1115_16SPS  = (0x0020) ///< 16 samples per second
	RATE_ADS1115_32SPS  = (0x0040) ///< 32 samples per second
	RATE_ADS1115_64SPS  = (0x0060) ///< 64 samples per second
	RATE_ADS1115_128SPS = (0x0080) ///< 128 samples per second (default)
	RATE_ADS1115_250SPS = (0x00A0) ///< 250 samples per second
	RATE_ADS1115_475SPS = (0x00C0) ///< 475 samples per second
	RATE_ADS1115_860SPS = (0x00E0) ///< 860 samples per second
)

// newADS1115 creates an ADC instance for an ADS1115 device with given config.
func newADS1115(config model.Device, bus bridge.I2CBus, onActive func()) (ADC, error) {
	if config.Type != model.DeviceTypeADS1115 {
		return nil, model.InvalidArgument("Invalid device type '%s'", string(config.Type))
	}
	address, err := parseAddress(config.Address)
	if err != nil {
		return nil, err
	}
	return &ads1115{
		onActive: onActive,
		config:   config,
		bus:      bus,
		address:  byte(address),
	}, nil
}

// Configure is called once to put the device in the desired state.
func (d *ads1115) Configure(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.onActive()
	configBits := d.createConfigBits(1)
	if err := d.writeConfig(ctx, configBits); err != nil {
		return err
	}
	return nil
}

// Close brings the device back to a safe state.
func (d *ads1115) Close(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Restore all to defaults
	d.onActive()
	configBits := d.createConfigBits(1)
	if err := d.writeConfig(ctx, configBits); err != nil {
		return err
	}
	return nil
}

// PinCount returns the number of pins of the device
func (d *ads1115) PinCount() uint {
	return 4
}

// Set the pin at given index (1...)
func (d *ads1115) Get(ctx context.Context, pin model.DeviceIndex) (int, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Trigger a conversion
	configBits := d.createConfigBits(1) | ADS1X15_REG_CONFIG_OS_SINGLE
	if err := d.writeConfig(ctx, configBits); err != nil {
		return 0, err
	}
	// Fetch config
	currentBits, err := d.readConfig(ctx)
	if err != nil {
		return 0, err
	}
	osStatus := currentBits & ADS1X15_REG_CONFIG_OS_MASK
	if osStatus != ADS1X15_REG_CONFIG_OS_BUSY {
		fmt.Printf("Expected OS_BUSY, got %0x\n", osStatus)
	}

	// Wait until conversion ready
	for {
		if err := ctx.Err(); err != nil {
			// Context canceled
			return 0, err
		}
		// Check conversion status
		complete, _, err := d.isConversionComplete(ctx)
		if err != nil {
			return 0, err
		}
		if complete {
			// Conversion completed
			break
		}
		//fmt.Printf("Conversion not complete yet; got 0x%0x\n", status)
		time.Sleep(time.Millisecond * 1)
	}

	// Read conversion value
	result, err := d.readConversion(ctx)
	if err != nil {
		return 0, err
	}
	return int(result), nil
}

// read the Conversion registry
func (d *ads1115) readConversion(ctx context.Context) (uint16, error) {
	return d.readWordReg(ctx, ads1115RegConversion)
}

// Is the conversion complete?
func (d *ads1115) isConversionComplete(ctx context.Context) (bool, uint16, error) {
	status, err := d.readWordReg(ctx, ads1115RegConfig)
	if err != nil {
		return false, 0, err
	}
	osStatus := status & ADS1X15_REG_CONFIG_OS_MASK
	return osStatus == ADS1X15_REG_CONFIG_OS_NOTBUSY, status, nil
}

// read the config registry
func (d *ads1115) readConfig(ctx context.Context) (uint16, error) {
	return d.readWordReg(ctx, ads1115RegConfig)
}

// write the config registry
func (d *ads1115) writeConfig(ctx context.Context, configBits uint16) error {
	return d.writeWordReg(ctx, ads1115RegConfig, configBits)
}

// read a 16-bit register
func (d *ads1115) readWordReg(ctx context.Context, reg uint8) (uint16, error) {
	// Send registry first
	var result uint16
	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		var buf [3]uint8
		buf[0] = reg
		if err := dev.WriteDevice(buf[:1]); err != nil {
			return fmt.Errorf("failed to write registry: %w", err)
		}

		// Read msb-lsb
		if err := dev.ReadDevice(buf[1:]); err != nil {
			return fmt.Errorf("failed to read word: %w", err)
		}
		// ADS115 transfer MSB first, then LSB
		// So we have to flip bytes
		result = (uint16(buf[1]) << 8) | uint16(buf[2])
		return nil
	}); err != nil {
		return 0, err
	}
	return result, nil
}

// write a 16-bit register value
func (d *ads1115) writeWordReg(ctx context.Context, reg uint8, value uint16) error {
	var buf [3]uint8
	// ADS115 transfer MSB first, then LSB
	buf[0] = reg
	buf[1] = uint8((value >> 8) & 0xFF)
	buf[2] = uint8(value & 0xFF)

	if err := d.bus.Execute(ctx, d.address, func(ctx context.Context, dev bridge.I2CDevice) error {
		return dev.WriteDevice(buf[:])
	}); err != nil {
		return err
	}
	return nil
}

// createConfigBits creates bits for the Config registry for a single shot
// on pin 1..4.
// Note that the start for a single conversion bit is not included.
func (d *ads1115) createConfigBits(pin int) uint16 {
	return ADS1X15_REG_CONFIG_CQUE_NONE |
		ADS1X15_REG_CONFIG_CLAT_NONLAT |
		ADS1X15_REG_CONFIG_CPOL_ACTVLOW |
		ADS1X15_REG_CONFIG_CMODE_TRAD |
		RATE_ADS1115_250SPS |
		ADS1X15_REG_CONFIG_MODE_SINGLE |
		ADS1X15_REG_CONFIG_PGA_6_144V |
		MUX_BY_CHANNEL[pin-1]
}
