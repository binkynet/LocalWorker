// Copyright 2025 Ewout Prangsma
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

package devices

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/pkg/service/bridge"
)

type localGPIO struct {
	onActive func()
	config   model.Device
	api      bridge.API
	inputs   []bridge.InputPin
	outputs  []bridge.OutputPin
}

// newLocalGPIO creates a GPIO instance for a GPIO locally on the worker
func newLocalGPIO(config model.Device, api bridge.API, onActive func()) (GPIO, error) {
	if config.Type != model.DeviceTypeGPIO {
		return nil, model.InvalidArgument("Invalid device type '%s'", string(config.Type))
	}
	return &localGPIO{
		onActive: onActive,
		config:   config,
		api:      api,
	}, nil
}

// Configure is called once to put the device in the desired state.
func (d *localGPIO) Configure(ctx context.Context) error {
	d.onActive()
	d.inputs = make([]bridge.InputPin, d.PinCount())
	d.outputs = make([]bridge.OutputPin, d.PinCount())
	return nil
}

// Close brings the device back to a safe state.
func (d *localGPIO) Close(ctx context.Context) error {
	d.onActive()
	d.inputs = nil
	d.outputs = nil
	return nil
}

// PinCount returns the number of pins of the device
func (d *localGPIO) PinCount() uint {
	return uint(d.api.PinCount())
}

// Set the direction of the pin at given index (1...)
func (d *localGPIO) SetDirection(ctx context.Context, pin model.DeviceIndex, direction PinDirection) error {
	d.onActive()
	switch direction {
	case PinDirectionInput:
		p, err := d.api.Input(int(pin), true)
		if err != nil {
			return err
		}
		d.inputs[pin-1] = p
		d.outputs[pin-1] = nil
	case PinDirectionOutput:
		p, err := d.api.Output(int(pin), true, false)
		if err != nil {
			return err
		}
		d.inputs[pin-1] = nil
		d.outputs[pin-1] = p
	default:
		return fmt.Errorf("invalid direction")
	}
	return nil
}

// Get the direction of the pin at given index (1...)
func (d *localGPIO) GetDirection(ctx context.Context, pin model.DeviceIndex) (PinDirection, error) {
	index := pin - 1
	if d.inputs[index] != nil {
		return PinDirectionInput, nil
	}
	if d.outputs[index] != nil {
		return PinDirectionOutput, nil
	}
	return PinDirectionInput, fmt.Errorf("not configured")
}

// Set the pin at given index (1...) to the given value
func (d *localGPIO) Set(ctx context.Context, pin model.DeviceIndex, value bool) error {
	index := pin - 1
	if f := d.outputs[index]; f != nil {
		return f.Write(value)
	}
	return errors.Wrapf(InvalidDirectionError, "pin %d does not have direction output", pin)
}

// Set the pin at given index (1...)
func (d *localGPIO) Get(ctx context.Context, pin model.DeviceIndex) (bool, error) {
	index := pin - 1
	if f := d.inputs[index]; f != nil {
		return f.Read()
	}
	return false, errors.Wrapf(InvalidDirectionError, "pin %d does not have direction input", pin)
}
