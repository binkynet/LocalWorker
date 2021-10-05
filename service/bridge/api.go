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
	"fmt"
	"time"
)

// API of the bridge, the hardware used to connect a Raspberry PI GPIO
// port to a buffered I2C network that local slaves are connected to.
type API interface {
	// Turn Green status led on/off
	SetGreenLED(on bool) error
	// Turn Red status led on/off
	SetRedLED(on bool) error
	// Blink Green status led with given duration between on/off
	BlinkGreenLED(delay time.Duration) error
	// Blink Red status led with given duration between on/off
	BlinkRedLED(delay time.Duration) error

	// Open the I2C bus
	I2CBus() (I2CBus, error)

	Close() error
}

func TestI2CBus(bus I2CBus) {
	if d, err := bus.OpenDevice(0x20); err != nil {
		fmt.Printf("Cannot open device: %v\n", err)
	} else {
		for r := byte(0); r <= 0x15; r++ {
			time.Sleep(time.Millisecond * 50)
			if v, err := d.ReadByteReg(r); err != nil {
				fmt.Printf("Cannot read register %2x: %#v\n", r, err)
			} else {
				fmt.Printf("Reg %2x == %2x\n", r, v)
			}
		}
		d.Close()
	}
}
