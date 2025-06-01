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
	"time"
)

type virtualBridge struct {
}

// NewVirtualBridge implements the bridge for a virtual local worker.
func NewVirtualBridge() (API, error) {
	return &virtualBridge{}, nil
}

// Turn Green status led on/off
func (p *virtualBridge) SetGreenLED(on bool) error {
	return nil
}

// Turn Red status led on/off
func (p *virtualBridge) SetRedLED(on bool) error {
	return nil
}

// Blink Green status led with given duration between on/off
func (p *virtualBridge) BlinkGreenLED(delay time.Duration) error {
	return nil
}

// Blink Red status led with given duration between on/off
func (p *virtualBridge) BlinkRedLED(delay time.Duration) error {
	return nil
}

// Open the I2C bus
func (p *virtualBridge) I2CBus() (I2CBus, error) {
	return p, nil
}

func (p *virtualBridge) Close() error {
	return nil
}

// Execute an option on the bus.
func (p *virtualBridge) Execute(ctx context.Context, address uint8, op func(ctx context.Context, dev I2CDevice) error) error {
	return fmt.Errorf("device %0x not found", address)
}

// DetectSlaveAddresses probes the bus to detect available addresses.
func (p *virtualBridge) DetectSlaveAddresses() []byte {
	return nil
}
