//    Copyright 2017 Ewout Prangsma
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package bridge

import (
	"sync"
	"time"

	"github.com/ecc1/gpio"
	"github.com/pkg/errors"
)

type orangepizeroBridge struct {
	mutex    sync.Mutex
	greenLed statusLed
	redLed   statusLed
	bus      I2CBus
}

const (
	opzGreenLedPin = 19
	opzRedLedPin   = 18
)

// NewOrangePIZeroBridge implements the bridge for an Orange PI Zero
func NewOrangePIZeroBridge() (API, error) {
	activeLow := true
	initialValue := false

	greenLed, err := gpio.Output(opzGreenLedPin, activeLow, initialValue)
	if err != nil {
		return nil, errors.Wrap(err, "Output[greenLed] failed")
	}
	redLed, err := gpio.Output(opzRedLedPin, activeLow, initialValue)
	if err != nil {
		return nil, errors.Wrap(err, "Output[redLed] failed")
	}

	return &orangepizeroBridge{
		greenLed: statusLed{pin: greenLed},
		redLed:   statusLed{pin: redLed},
	}, nil
}

// Turn Green status led on/off
func (p *orangepizeroBridge) SetGreenLED(on bool) error {
	if err := p.greenLed.Set(on); err != nil {
		return errors.Wrap(err, "Set[greenLed] failed")
	}
	return nil
}

// Turn Red status led on/off
func (p *orangepizeroBridge) SetRedLED(on bool) error {
	if err := p.redLed.Set(on); err != nil {
		return errors.Wrap(err, "Set[redLed] failed")
	}
	return nil
}

// Blink Green status led with given duration between on/off
func (p *orangepizeroBridge) BlinkGreenLED(delay time.Duration) error {
	if err := p.greenLed.Blink(delay); err != nil {
		return errors.Wrap(err, "Blink[greenLed] failed")
	}
	return nil
}

// Blink Red status led with given duration between on/off
func (p *orangepizeroBridge) BlinkRedLED(delay time.Duration) error {
	if err := p.redLed.Blink(delay); err != nil {
		return errors.Wrap(err, "Blink[redLed] failed")
	}
	return nil
}

// Open the I2C bus
func (p *orangepizeroBridge) I2CBus() (I2CBus, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.bus == nil {
		bus, err := NewI2cDevice("/dev/i2c-0")
		if err != nil {
			return nil, errors.Wrap(err, "NewI2cDevice failed")
		}
		p.bus = bus
	}
	return p.bus, nil
}

func (p *orangepizeroBridge) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.bus != nil {
		bus := p.bus
		p.bus = nil
		if err := bus.Close(); err != nil {
			return errors.Wrap(err, "Close failed")
		}
	}
	return nil
}
