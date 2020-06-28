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

package objects

import (
	"context"
	"sync/atomic"
	"time"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/service/devices"
	"github.com/rs/zerolog"
)

var (
	binarySensorType = &ObjectType{
		Run: nil,
	}
)

type binarySensor struct {
	log         zerolog.Logger
	config      model.Object
	address     model.ObjectAddress
	sender      string
	inputDevice devices.GPIO
	pin         model.DeviceIndex
	sendNow     int32
}

// newBinarySensor creates a new binary-sensor object for the given configuration.
func newBinarySensor(sender string, oid model.ObjectID, address model.ObjectAddress, config model.Object, log zerolog.Logger, devService devices.Service) (Object, error) {
	if config.Type != model.ObjectTypeBinarySensor {
		return nil, model.InvalidArgument("Invalid object type '%s'", config.Type)
	}
	conn, ok := config.ConnectionByName(model.ConnectionNameSensor)
	if !ok {
		return nil, model.InvalidArgument("Pin '%s' not found in object '%s'", model.ConnectionNameSensor, oid)
	}
	if len(conn.Pins) != 1 {
		return nil, model.InvalidArgument("Pin '%s' must have 1 pin in object '%s', got %d", model.ConnectionNameSensor, oid, len(conn.Pins))
	}
	device, ok := devService.DeviceByID(conn.Pins[0].DeviceId)
	if !ok {
		return nil, model.InvalidArgument("Device '%s' not found in object '%s'", conn.Pins[0].DeviceId, oid)
	}
	gpio, ok := device.(devices.GPIO)
	if !ok {
		return nil, model.InvalidArgument("Device '%s' in object '%s' is not a GPIO", conn.Pins[0].DeviceId, oid)
	}
	pin := conn.Pins[0].Index
	if pin < 1 || uint(pin) > gpio.PinCount() {
		return nil, model.InvalidArgument("Pin '%s' in object '%s' is out of range. Got %d. Range [1..%d]", model.ConnectionNameSensor, oid, pin, gpio.PinCount())
	}
	return &binarySensor{
		log:         log,
		config:      config,
		address:     address,
		sender:      sender,
		inputDevice: gpio,
		pin:         pin,
	}, nil
}

// Return the type of this object.
func (o *binarySensor) Type() *ObjectType {
	return binarySensorType
}

// Configure is called once to put the object in the desired state.
func (o *binarySensor) Configure(ctx context.Context) error {
	if err := o.inputDevice.SetDirection(ctx, o.pin, devices.PinDirectionInput); err != nil {
		return err
	}
	return nil
}

// Run the object until the given context is cancelled.
func (o *binarySensor) Run(ctx context.Context, requests RequestService, statuses StatusService, moduleID string) error {
	lastValue := false
	changes := 0
	recentErrors := 0
	log := o.log
	for {
		delay := time.Millisecond * 10

		// Read state
		value, err := o.inputDevice.Get(ctx, o.pin)
		if err != nil {
			// Try again soon
			if recentErrors == 0 {
				log.Info().Err(err).Msg("Get value failed")
			}
			recentErrors++
		} else {
			recentErrors = 0
			force := atomic.CompareAndSwapInt32(&o.sendNow, 1, 0)
			if force || lastValue != value || changes == 0 {
				// Send feedback data
				log = log.With().Bool("value", value).Logger()
				log.Debug().Bool("force", force).Msg("change detected")
				actual := model.Sensor{
					Address: o.address,
					Actual: &model.SensorState{
						Value: boolToInt32(value),
					},
				}
				statuses.PublishSensorActual(actual)
				changes++
			}
		}

		// Wait a bit
		select {
		case <-time.After(delay):
			// Continue
		case <-ctx.Done():
			// Context cancelled
			return nil
		}
	}
}

// ProcessPowerMessage acts upons a given power message.
func (o *binarySensor) ProcessPowerMessage(ctx context.Context, m model.PowerState) error {
	if m.GetEnabled() {
		atomic.StoreInt32(&o.sendNow, 1)
	}
	return nil
}
