// Copyright 2020 Ewout Prangsma
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author Ewout Prangsma
//

package bridge

import (
	"fmt"
	"time"
)

type stubI2CBus struct {
}

func (s *stubI2CBus) OpenDevice(address uint8) (I2CDevice, error) {
	return s, nil
}

func (s *stubI2CBus) Close() (err error) {
	return nil
}

func (s *stubI2CBus) ReadByteReg(reg uint8) (uint8, error) {
	return 0, nil
}

func (s *stubI2CBus) WriteByteReg(reg uint8, val uint8) (err error) {
	return nil
}

func (s *stubI2CBus) ReadByte() (uint8, error) {
	return 0, nil
}

func (s *stubI2CBus) WriteByte(val uint8) (err error) {
	return nil
}

// Read a 16-bits word to the device
// S Addr Wr [A] Comm [A] Sr Addr Rd [A] [DataLow] A [DataHigh] NA P
func (s *stubI2CBus) ReadWordData(reg uint8) (val uint16, err error) {
	return 0, nil
}

// Write a 16-bits word to the device
// S Addr Wr [A] Comm [A] DataLow [A] DataHigh [A] P
func (s *stubI2CBus) WriteWordData(reg uint8, data uint16) (err error) {
	return nil
}

// Read a block of data (without count) to the device
// S Addr Wr [A] Comm [A] Sr Addr Rd [A] [Data] A [Data] A ... A [Data] NA P
func (s *stubI2CBus) ReadI2CBlock(reg uint8, data []byte) (err error) {
	return nil
}

// Write a block of data (without count) to the device
// S Addr Wr [A] Comm [A] Data [A] Data [A] ... [A] Data [A] P
func (s *stubI2CBus) WriteI2CBlock(reg uint8, data []byte) (err error) {
	return nil
}

// Read a block of data directly from the device (/dev/...)
func (s *stubI2CBus) ReadDevice(data []byte) (err error) {
	return nil
}

// Write a block of data directly to the device (/dev/...)
func (s *stubI2CBus) WriteDevice(data []byte) (err error) {
	return nil
}

// DetectSlaveAddresses probes the bus to detect available addresses.
func (s *stubI2CBus) DetectSlaveAddresses() []byte {
	return nil
}

func NewStub() API {
	return &stubAPI{}
}

type stubAPI struct {
	stubI2CBus
}

// Turn Green status led on/off
func (s *stubAPI) SetGreenLED(on bool) error {
	if on {
		fmt.Println("Green on")
	} else {
		fmt.Println("Green off")
	}
	return nil
}

// Turn Red status led on/off
func (s *stubAPI) SetRedLED(on bool) error {
	if on {
		fmt.Println("Red on")
	} else {
		fmt.Println("Red off")
	}
	return nil
}

// Blink Green status led with given duration between on/off
func (s *stubAPI) BlinkGreenLED(delay time.Duration) error {
	fmt.Println("Blink Green")
	<-time.After(delay)
	return nil
}

// Blink Red status led with given duration between on/off
func (s *stubAPI) BlinkRedLED(delay time.Duration) error {
	fmt.Println("Blink Red")
	<-time.After(delay)
	return nil
}

// Open the I2C bus
func (s *stubAPI) I2CBus() (I2CBus, error) {
	return &s.stubI2CBus, nil
}
