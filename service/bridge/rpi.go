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
	"fmt"
	"sync"
	"time"

	"github.com/ecc1/gpio"
	"golang.org/x/exp/io/i2c"
)

const (
	greenLedPin = 23
	redLedPin   = 24
	i2cDev      = "/dev/i2c-1"
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
	greenLed statusLed
	redLed   statusLed
	devFs    *i2c.Devfs
}

// NewRaspberryPiBridge implements the bridge for Raspberry PI's
func NewRaspberryPiBridge() (API, error) {
	activeLow := true
	greenLed, err := gpio.Output(greenLedPin, activeLow)
	if err != nil {
		return nil, maskAny(err)
	}
	redLed, err := gpio.Output(redLedPin, activeLow)
	if err != nil {
		return nil, maskAny(err)
	}
	return &piBridge{
		greenLed: statusLed{pin: greenLed},
		redLed:   statusLed{pin: redLed},
		devFs:    &i2c.Devfs{Dev: i2cDev},
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

// Try to detect all known addresses of local slaves.
func (p *piBridge) DetectLocalSlaveAddresses() ([]int, error) {
	var result []int
	/*for addr := 0; addr < 128; addr++ {
		dev, err := i2c.Open(p.devFs, addr)
		if err == nil {
			dev.Close()
			result = append(result, addr)
		}
	}*/
	return result, nil
}

func (p *piBridge) Test() {
	dev, err := Bus(1)
	if err != nil {
		fmt.Printf("Cannot open slave: %#v\n", err)
		return
	}
	//defer dev.Close()
	for r := byte(0); r <= 0x15; r++ {
		time.Sleep(time.Millisecond * 50)
		if v, err := dev.ReadByteBlock(0x20, r, 1); err != nil {
			fmt.Printf("Cannot read register %2x: %#v\n", r, err)
		} else {
			fmt.Printf("Reg %2x == %2x\n", r, v[0])
		}
	}
}
