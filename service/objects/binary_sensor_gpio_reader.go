// Copyright 2022 Ewout Prangsma
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

package objects

import (
	"context"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/service/devices"
)

// binarySensorGPIOReader is a binary sensor reader that read a pin of a GPIO device.
type binarySensorGPIOReader struct {
	inputDevice devices.GPIO
	pin         model.DeviceIndex
}

// newBinarySensor creates a new binary-sensor object for the given configuration.
func newBinarySensorGPIOReader(gpio devices.GPIO, pin model.DeviceIndex) (binarySensorReader, error) {
	return &binarySensorGPIOReader{
		inputDevice: gpio,
		pin:         pin,
	}, nil
}

// Configure is called once to put the object in the desired state.
func (o *binarySensorGPIOReader) Configure(ctx context.Context) error {
	if err := o.inputDevice.SetDirection(ctx, o.pin, devices.PinDirectionInput); err != nil {
		return err
	}
	return nil
}

// Read the current sensor value
func (o *binarySensorGPIOReader) Read(ctx context.Context) (bool, error) {
	return o.inputDevice.Get(ctx, o.pin)
}
