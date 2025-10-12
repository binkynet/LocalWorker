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
	"context"
	"sync"
	"time"

	"github.com/ecc1/gpio"
	"github.com/pkg/errors"
)

const (
	greenLedPin = 23
	redLedPin   = 24
	rpiSclPin   = -1
)

type statusLed struct {
	sync.Mutex
	pin         gpio.OutputPin
	cancelBlink func()
}

// Turn led on/off, cancel blink
func (l *statusLed) Set(on bool) error {
	l.Mutex.Lock()
	defer l.Mutex.Unlock()

	if cancel := l.cancelBlink; cancel != nil {
		l.cancelBlink = nil
		cancel()
	}
	if err := l.pin.Write(on); err != nil {
		return errors.Wrap(err, "Write failed")
	}
	return nil
}

// Blink led on/off
func (l *statusLed) Blink(delay time.Duration) error {
	l.Mutex.Lock()
	defer l.Mutex.Unlock()

	if cancel := l.cancelBlink; cancel != nil {
		l.cancelBlink = nil
		cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	l.cancelBlink = cancel
	go func() {
		value := true
		for {
			l.Mutex.Lock()
			if ctx.Err() == nil {
				l.pin.Write(value)
				value = !value
			}
			l.Mutex.Unlock()
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

type piBridge struct {
	mutex    sync.Mutex
	greenLed statusLed
	redLed   statusLed
	bus      I2CBus
}

// NewRaspberryPiBridge implements the bridge for Raspberry PI's
func NewRaspberryPiBridge() (API, error) {
	activeLow := true
	initialValue := false
	greenLed, err := gpio.Output(greenLedPin, activeLow, initialValue)
	if err != nil {
		return nil, errors.Wrap(err, "Output[greenLed] failed")
	}
	redLed, err := gpio.Output(redLedPin, activeLow, initialValue)
	if err != nil {
		return nil, errors.Wrap(err, "Output[redLed] failed")
	}
	return &piBridge{
		greenLed: statusLed{pin: greenLed},
		redLed:   statusLed{pin: redLed},
	}, nil
}

// Returns number of local pins
func (p *piBridge) PinCount() int {
	return 17 // TODO
}

// Input initializes a GPIO input pin with the given pin number.
func (p *piBridge) Input(pinNumber int, activeLow bool) (InputPin, error) {
	return gpio.Input(pinNumber, activeLow)
}

// Output initializes a GPIO output pin with the given pin number
// and initial logical value.
func (p *piBridge) Output(pinNumber int, activeLow bool, initialValue bool) (OutputPin, error) {
	return gpio.Output(pinNumber, activeLow, initialValue)
}

// Turn Green status led on/off
func (p *piBridge) SetGreenLED(on bool) error {
	if err := p.greenLed.Set(on); err != nil {
		return errors.Wrap(err, "Set[greenLed] failed")
	}
	return nil
}

// Turn Red status led on/off
func (p *piBridge) SetRedLED(on bool) error {
	if err := p.redLed.Set(on); err != nil {
		return errors.Wrap(err, "Set[redLed] failed")
	}
	return nil
}

// Blink Green status led with given duration between on/off
func (p *piBridge) BlinkGreenLED(delay time.Duration) error {
	if err := p.greenLed.Blink(delay); err != nil {
		return errors.Wrap(err, "Blink[greenLed] failed")
	}
	return nil
}

// Blink Red status led with given duration between on/off
func (p *piBridge) BlinkRedLED(delay time.Duration) error {
	if err := p.redLed.Blink(delay); err != nil {
		return errors.Wrap(err, "Blink[redLed] failed")
	}
	return nil
}

// Open the I2C bus
func (p *piBridge) I2CBus() (I2CBus, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.bus == nil {
		bus, err := NewI2CBus("/dev/i2c-1", rpiSclPin)
		if err != nil {
			return nil, errors.Wrap(err, "NewI2cDevice failed")
		}
		p.bus = bus
	}
	return p.bus, nil
}

func (p *piBridge) Close() error {
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
