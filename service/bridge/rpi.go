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
)

const (
	greenLedPin = 23
	redLedPin   = 24
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
		return maskAny(err)
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
		value := false
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
	bus      *I2CBus
}

// NewRaspberryPiBridge implements the bridge for Raspberry PI's
func NewRaspberryPiBridge() (API, error) {
	activeLow := true
	initialValue := false
	greenLed, err := gpio.Output(greenLedPin, activeLow, initialValue)
	if err != nil {
		return nil, maskAny(err)
	}
	redLed, err := gpio.Output(redLedPin, activeLow, initialValue)
	if err != nil {
		return nil, maskAny(err)
	}
	return &piBridge{
		greenLed: statusLed{pin: greenLed},
		redLed:   statusLed{pin: redLed},
	}, nil
}

// Turn Green status led on/off
func (p *piBridge) SetGreenLED(on bool) error {
	if err := p.greenLed.Set(on); err != nil {
		return maskAny(err)
	}
	return nil
}

// Turn Red status led on/off
func (p *piBridge) SetRedLED(on bool) error {
	if err := p.redLed.Set(on); err != nil {
		return maskAny(err)
	}
	return nil
}

// Blink Green status led with given duration between on/off
func (p *piBridge) BlinkGreenLED(delay time.Duration) error {
	if err := p.greenLed.Blink(delay); err != nil {
		return maskAny(err)
	}
	return nil
}

// Blink Red status led with given duration between on/off
func (p *piBridge) BlinkRedLED(delay time.Duration) error {
	if err := p.redLed.Blink(delay); err != nil {
		return maskAny(err)
	}
	return nil
}

// Open the I2C bus
func (p *piBridge) I2CBus() (*I2CBus, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.bus == nil {
		bus, err := NewI2cDevice("/dev/i2c-1")
		if err != nil {
			return nil, maskAny(err)
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
			return maskAny(err)
		}
	}
	return nil
}
