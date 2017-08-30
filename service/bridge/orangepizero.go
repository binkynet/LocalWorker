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

	"golang.org/x/exp/io/i2c"
)

type orangepizeroBridge struct {
	devFs *i2c.Devfs
}

// NewOrangePIZeroBridge implements the bridge for an Orange PI Zero
func NewOrangePIZeroBridge() (API, error) {
	return &orangepizeroBridge{
		devFs: &i2c.Devfs{Dev: "/dev/i2c-0"},
	}, nil
}

// Turn Green status led on/off
func (p *orangepizeroBridge) SetGreenLED(on bool) error {
	return nil
}

// Turn Red status led on/off
func (p *orangepizeroBridge) SetRedLED(on bool) error {
	return nil
}

// Blink Green status led with given duration between on/off
func (p *orangepizeroBridge) BlinkGreenLED(delay time.Duration) error {
	return nil
}

// Blink Red status led with given duration between on/off
func (p *orangepizeroBridge) BlinkRedLED(delay time.Duration) error {
	return nil
}

// Try to detect all known addresses of local slaves.
func (p *orangepizeroBridge) DetectLocalSlaveAddresses() ([]int, error) {
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

func (p *orangepizeroBridge) Test() {
	dev, err := Bus(0)
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
