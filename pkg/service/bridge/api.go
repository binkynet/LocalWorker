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

	// Access to local GPIO

	// Returns number of local pins
	PinCount() int
	// Input initializes a GPIO input pin with the given pin number.
	Input(pinNumber int, activeLow bool) (InputPin, error)
	// Output initializes a GPIO output pin with the given pin number
	// and initial logical value.
	Output(pinNumber int, activeLow bool, initialValue bool) (OutputPin, error)

	Close() error
}

// InputPin is the interface satisfied by GPIO input pins.
type InputPin interface {
	Read() (bool, error)
}

// OutputPin is the interface satisfied by GPIO output pins.
type OutputPin interface {
	Write(bool) error
}
