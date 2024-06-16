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

const (
	lastSensorSentThreshold = time.Second * 15
)

type binarySensor struct {
	log       zerolog.Logger
	config    model.Object
	address   model.ObjectAddress
	sender    string
	reader    binarySensorReader
	invert    bool
	sendNow   int32
	lastPower bool
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
	debug := conn.GetBoolConfig(model.ConfigKeyDebug)
	var reader binarySensorReader
	if gpio, ok := device.(devices.GPIO); ok {
		pin := conn.Pins[0].Index
		if pin < 1 || uint(pin) > gpio.PinCount() {
			return nil, model.InvalidArgument("Pin '%s' in object '%s' is out of range. Got %d. Range [1..%d]", model.ConnectionNameSensor, oid, pin, gpio.PinCount())
		}
		var err error
		reader, err = newBinarySensorGPIOReader(gpio, pin)
		if err != nil {
			return nil, err
		}
	} else if adc, ok := device.(devices.ADC); ok {
		pin := conn.Pins[0].Index
		if pin < 1 || uint(pin) > adc.PinCount() {
			return nil, model.InvalidArgument("Pin '%s' in object '%s' is out of range. Got %d. Range [1..%d]", model.ConnectionNameSensor, oid, pin, adc.PinCount())
		}
		threshold := conn.GetIntConfig(model.ConfigKeyThreshold)
		var err error
		reader, err = newBinarySensorADCReader(log, adc, pin, threshold, debug)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, model.InvalidArgument("Device '%s' in object '%s' is not a GPIO or ADC", conn.Pins[0].DeviceId, oid)
	}
	invert := conn.GetBoolConfig(model.ConfigKeyInvert)
	return &binarySensor{
		log:     log,
		config:  config,
		address: address,
		sender:  sender,
		reader:  reader,
		invert:  invert,
	}, nil
}

// Return the type of this object.
func (o *binarySensor) Type() ObjectType {
	return binarySensorTypeInstance
}

// Configure is called once to put the object in the desired state.
func (o *binarySensor) Configure(ctx context.Context) error {
	return o.reader.Configure(ctx)
}

// Run the object until the given context is cancelled.
func (o *binarySensor) Run(ctx context.Context, requests RequestService, statuses StatusService, moduleID string) error {
	id := string(o.address)
	lastValue := false
	changes := 0
	recentErrors := 0
	log := o.log
	lastSent := time.Now()
	actual := model.Sensor{
		Address: o.address,
		Actual: &model.SensorState{
			Value: 0,
		},
	}
	for {
		delay := time.Millisecond * 50

		// Read state
		value, err := o.reader.Read(ctx)
		if err != nil {
			// Try again soon
			if recentErrors == 0 {
				log.Error().Err(err).Msg("Read value failed")
			}
			recentErrors++
			// Make sensor active to avoid trains running wild
			value = true
			// Update metrics
			binarySensorReadErrorsTotal.WithLabelValues(id).Inc()
		} else {
			// Read succeeded, invert if needed
			if o.invert {
				value = !value
			}
			// Reset reset error count
			recentErrors = 0
		}
		force := atomic.CompareAndSwapInt32(&o.sendNow, 1, 0)
		timeoutThreshold := time.Since(lastSent) > lastSensorSentThreshold
		valueChanged := lastValue != value
		if force || valueChanged || changes == 0 || timeoutThreshold {
			// Send feedback data
			log = log.With().
				Bool("value", value).
				Bool("last_value", lastValue).
				Logger()
			if force || valueChanged || changes == 0 {
				log.Debug().Bool("force", force).Msg("change detected")
			}
			// Update metrics
			binarySensorActualGauge.WithLabelValues(id).Set(float64(boolToInt32(value)))
			if valueChanged {
				binarySensorChangesTotal.WithLabelValues(id).Inc()
			}
			actual.Actual.Value = boolToInt32(value)
			if statuses.PublishSensorActual(actual) {
				lastValue = value
				changes++
				lastSent = time.Now()
			} else {
				// Failed to enqueue actual sent.
				// Force next update
				atomic.StoreInt32(&o.sendNow, 1)
			}
		}

		// Wait a bit
		select {
		case <-ctx.Done():
			// Context cancelled
			return nil
		case <-time.After(delay):
			// Continue
		}
	}
}

// ProcessPowerMessage acts upons a given power message.
func (o *binarySensor) ProcessPowerMessage(ctx context.Context, m model.PowerState) error {
	enabled := m.GetEnabled()
	if o.lastPower != enabled {
		o.lastPower = enabled
		if m.GetEnabled() {
			atomic.StoreInt32(&o.sendNow, 1)
		}
	}
	return nil
}
