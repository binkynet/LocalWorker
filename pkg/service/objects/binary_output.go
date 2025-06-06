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
	"github.com/binkynet/LocalWorker/pkg/service/devices"
	"github.com/rs/zerolog"
)

type binaryOutput struct {
	log              zerolog.Logger
	config           model.Object
	address          model.ObjectAddress
	sender           string
	outputDevice     devices.GPIO
	pin              model.DeviceIndex
	invert           bool
	targetState      int32
	currentState     int32
	sendActualNeeded int32
}

// newBinaryOutput creates a new binary-output object for the given configuration.
func newBinaryOutput(sender string, oid model.ObjectID, address model.ObjectAddress, config model.Object, log zerolog.Logger, devService devices.Service) (Object, error) {
	if config.Type != model.ObjectTypeBinaryOutput {
		return nil, model.InvalidArgument("Invalid object type '%s'", config.Type)
	}
	conn, ok := config.ConnectionByName(model.ConnectionNameOutput)
	if !ok {
		return nil, model.InvalidArgument("Pin '%s' not found in object '%s'", model.ConnectionNameOutput, oid)
	}
	if len(conn.Pins) != 1 {
		return nil, model.InvalidArgument("Pin '%s' must have 1 pin in object '%s', got %d", model.ConnectionNameOutput, oid, len(conn.Pins))
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
		return nil, model.InvalidArgument("Pin '%s' in object '%s' is out of range. Got %d. Range [1..%d]", model.ConnectionNameOutput, oid, pin, gpio.PinCount())
	}
	if mqtt, ok := device.(devices.MQTT); ok {
		mqtt.SetStateTopic(pin, conn.GetStringConfig(model.ConfigKeyMQTTStateTopic))
		mqtt.SetCommandTopic(pin, conn.GetStringConfig(model.ConfigKeyMQTTCommandTopic))
	}
	invert := conn.GetBoolConfig(model.ConfigKeyInvert)
	return &binaryOutput{
		log:          log,
		config:       config,
		address:      address,
		sender:       sender,
		outputDevice: gpio,
		pin:          pin,
		invert:       invert,
	}, nil
}

// Return the type of this object.
func (o *binaryOutput) Type() ObjectType {
	return outputTypeInstance
}

// Configure is called once to put the object in the desired state.
func (o *binaryOutput) Configure(ctx context.Context) error {
	for i := 0; i < 5; i++ {
		if err := o.outputDevice.SetDirection(ctx, o.pin, devices.PinDirectionOutput); err != nil {
			return err
		}
	}
	return nil
}

// Run the object until the given context is cancelled.
func (o *binaryOutput) Run(ctx context.Context, requests RequestService, statuses StatusService, moduleID string) error {
	defer o.log.Debug().Msg("binaryOutput.Run terminated")
	initialized := false
	for {
		delay := time.Millisecond * 5
		if !initialized || o.targetState != o.currentState {
			delay = time.Millisecond

			// Now set the desired value
			if err := o.outputDevice.Set(ctx, o.pin, o.pinValue(int32ToBool(o.targetState))); err != nil {
				o.log.Warn().Err(err).Msg("GPIO.set failed")
			}

			o.currentState = o.targetState
			initialized = true
		}
		// Send actual message (if needed)
		sendNeeded := atomic.CompareAndSwapInt32(&o.sendActualNeeded, 1, 0)
		if sendNeeded {
			msg := model.Output{
				Address: o.address,
				Request: &model.OutputState{
					Value: o.currentState,
				},
				Actual: &model.OutputState{
					Value: o.currentState,
				},
			}
			statuses.PublishOutputActual(msg)
			// o.log.Debug().Msg("Sent output actual")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay):
			// Continue
		}
	}
}

// ProcessMessage acts upons a given request.
func (o *binaryOutput) ProcessMessage(ctx context.Context, r model.Output) error {
	o.targetState = r.GetRequest().GetValue()
	atomic.StoreInt32(&o.sendActualNeeded, 1)

	/*value := r.GetRequest().GetValue()
	log := o.log.With().Int32("value", value).Logger()
	log.Debug().Msg("got request")
	if err := o.outputDevice.Set(ctx, o.pin, o.pinValue(int32ToBool(value))); err != nil {
		log.Debug().Err(err).Msg("GPIO.set failed")
		return err
	}*/
	return nil
}

// Update metrics
func (o *binaryOutput) UpdateMetrics(msg model.Output) {
	id := string(msg.Address)
	binaryOutputRequestsTotal.WithLabelValues(id).Inc()
	binaryOutputRequestGauge.WithLabelValues(id).Set(float64(msg.GetRequest().GetValue()))
}

// ProcessPowerMessage acts upons a given power message.
func (o *binaryOutput) ProcessPowerMessage(ctx context.Context, m model.PowerState) error {
	return nil // TODO
}

func (o *binaryOutput) pinValue(value bool) bool {
	if o.invert {
		return !value
	}
	return value
}
