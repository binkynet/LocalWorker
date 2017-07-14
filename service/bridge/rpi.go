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
	"github.com/ecc1/gpio"
)

const (
	greenLedPin = 23
	redLedPin   = 24
)

type piBridge struct {
	greenLed gpio.OutputPin
	redLed   gpio.OutputPin
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
		greenLed: greenLed,
		redLed:   redLed,
	}, nil
}

// Turn Green status led on/off
func (p *piBridge) SetGreenLED(on bool) error {
	if err := p.greenLed.Write(on); err != nil {
		return maskAny(err)
	}
	return nil
}

// Turn Red status led on/off
func (p *piBridge) SetRedLED(on bool) error {
	if err := p.redLed.Write(on); err != nil {
		return maskAny(err)
	}
	return nil
}
