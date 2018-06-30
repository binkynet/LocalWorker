package devices

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/LocalWorker/service/bridge"
)

type pca9685 struct {
	config  model.Device
	bus     *bridge.I2CBus
	address byte
}

const (
	pca9685MODE1Reg      = 0x00
	pca9685MODE2Reg      = 0x01
	pca9685LEDBaseReg    = 0x06
	pca9685OnLowRegOfs   = 0
	pca9685OnHighRegOfs  = 1
	pca9685OffLowRegOfs  = 2
	pca9685OffHighRegOfs = 3
	pca9685RegIncrement  = 4
)

// newPCA9685 creates a PWM instance for a pca9685 device with given config.
func newPCA9685(config model.Device, bus *bridge.I2CBus) (PWM, error) {
	if config.Type != model.DeviceTypePCA9685 {
		return nil, errors.Wrapf(model.ValidationError, "Invalid device type '%s'", string(config.Type))
	}
	address, err := parseAddress(config.Address)
	if err != nil {
		return nil, maskAny(err)
	}
	return &pca9685{
		config:  config,
		bus:     bus,
		address: byte(address),
	}, nil
}

// Configure is called once to put the device in the desired state.
func (d *pca9685) Configure(ctx context.Context) error {
	// Set MODE1: SLEEP=0, ALLCALL=1
	mode1 := uint8(0x01)
	if err := d.bus.WriteByteReg(d.address, pca9685MODE1Reg, mode1); err != nil {
		return maskAny(err)
	}
	return nil
}

// Close brings the device back to a safe state.
func (d *pca9685) Close() error {
	// Set MODE1: SLEEP=1, ALLCALL=1
	mode1 := uint8(0x11)
	if err := d.bus.WriteByteReg(d.address, pca9685MODE1Reg, mode1); err != nil {
		return maskAny(err)
	}
	return nil
}

// OutputCount returns the number of pwm outputs of the device
func (d *pca9685) OutputCount() int {
	return 16
}

// MaxValue returns the maximum valid value for onValue or offValue.
func (d *pca9685) MaxValue() int {
	return 4095
}

// Set the output at given index (1...) to the given value
func (d *pca9685) Set(ctx context.Context, output int, onValue, offValue int) error {
	regBase, err := d.regBase(output)
	if err != nil {
		return maskAny(err)
	}
	if err := d.bus.WriteByteReg(d.address, uint8(regBase+pca9685OnLowRegOfs), uint8(onValue&0xFF)); err != nil {
		return maskAny(err)
	}
	if err := d.bus.WriteByteReg(d.address, uint8(regBase+pca9685OnHighRegOfs), uint8((onValue>>8)&0xFF)); err != nil {
		return maskAny(err)
	}
	if err := d.bus.WriteByteReg(d.address, uint8(regBase+pca9685OffLowRegOfs), uint8(offValue&0xFF)); err != nil {
		return maskAny(err)
	}
	if err := d.bus.WriteByteReg(d.address, uint8(regBase+pca9685OffHighRegOfs), uint8((offValue>>8)&0xFF)); err != nil {
		return maskAny(err)
	}
	return nil
}

// Set the output at given index (1...)
func (d *pca9685) Get(ctx context.Context, output int) (int, int, error) {
	regBase, err := d.regBase(output)
	if err != nil {
		return 0, 0, maskAny(err)
	}
	onLow, err := d.bus.ReadByteReg(d.address, uint8(regBase+pca9685OnLowRegOfs))
	if err != nil {
		return 0, 0, maskAny(err)
	}
	onHigh, err := d.bus.ReadByteReg(d.address, uint8(regBase+pca9685OnHighRegOfs))
	if err != nil {
		return 0, 0, maskAny(err)
	}
	offLow, err := d.bus.ReadByteReg(d.address, uint8(regBase+pca9685OffLowRegOfs))
	if err != nil {
		return 0, 0, maskAny(err)
	}
	offHigh, err := d.bus.ReadByteReg(d.address, uint8(regBase+pca9685OffHighRegOfs))
	if err != nil {
		return 0, 0, maskAny(err)
	}
	on := int(onLow) | (int(onHigh) << 8)
	off := int(offLow) | (int(offHigh) << 8)
	return on, off, nil
}

// regBase returns the first register for the given output.
func (d *pca9685) regBase(output int) (int, error) {
	if output < 1 || output > 16 {
		return 0, maskAny(fmt.Errorf("Output must be in 1..16 range, got %d", output))
	}
	return pca9685LEDBaseReg + ((output - 1) * pca9685RegIncrement), nil
}
