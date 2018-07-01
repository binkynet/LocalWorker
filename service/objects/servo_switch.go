// Copyright 2018 Ewout Prangsma
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
	"sync"
	"time"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/BinkyNet/mqp"
	"github.com/binkynet/LocalWorker/service/devices"
	"github.com/binkynet/LocalWorker/service/mqtt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	_ switchAPI = &servoSwitch{}
)

type servoSwitch struct {
	mutex   sync.Mutex
	log     zerolog.Logger
	config  model.Object
	address mqp.ObjectAddress
	servo   struct {
		device devices.PWM
		index  model.DeviceIndex
	}
	currentAngle int
	targetAngle  int
}

// newServoSwitch creates a new servo-switch object for the given configuration.
func newServoSwitch(oid model.ObjectID, address mqp.ObjectAddress, config model.Object, log zerolog.Logger, devService devices.Service) (Object, error) {
	if config.Type != model.ObjectTypeServoSwitch {
		return nil, errors.Wrapf(model.ValidationError, "Invalid object type '%s'", config.Type)
	}
	servoPin, err := getSinglePin(oid, config, model.ConnectionNameServo)
	if err != nil {
		return nil, maskAny(err)
	}
	servoDev, err := getPWMForPin(servoPin, devService)
	if err != nil {
		return nil, errors.Wrapf(err, "%s: (connection %s in object %s)", err.Error(), model.ConnectionNameStraightRelay, oid)
	}
	sw := &servoSwitch{
		log:          log,
		config:       config,
		address:      address,
		currentAngle: 80,
		targetAngle:  90,
	}
	sw.servo.device = servoDev
	sw.servo.index = servoPin.Index
	return sw, nil
}

// Return the type of this object.
func (o *servoSwitch) Type() *ObjectType {
	return switchType
}

// Configure is called once to put the object in the desired state.
func (o *servoSwitch) Configure(ctx context.Context) error {
	o.targetAngle = 0
	o.currentAngle = 0

	onValue, offValue := o.angleToOnOffValues(o.currentAngle, o.servo.device.MaxValue())
	if err := o.servo.device.Set(ctx, o.servo.index, onValue, offValue); err != nil {
		return maskAny(err)
	}
	return nil
}

// Run the object until the given context is cancelled.
func (o *servoSwitch) Run(ctx context.Context, mqttService mqtt.Service, topicPrefix string) error {
	for {
		targetAngle := o.targetAngle
		if targetAngle != o.currentAngle {
			// Make current angle closer to target angle
			step := minInt(2, absInt(targetAngle-o.currentAngle))
			var nextAngle int
			if o.currentAngle < targetAngle {
				nextAngle = o.currentAngle + step
			} else {
				nextAngle = o.currentAngle - step
			}
			onValue, offValue := o.angleToOnOffValues(nextAngle, o.servo.device.MaxValue())
			o.log.Debug().
				Int("on", onValue).
				Int("off", offValue).
				Msg("Set servo")
			if err := o.servo.device.Set(ctx, o.servo.index, onValue, offValue); err != nil {
				// oops
			} else {
				o.currentAngle = nextAngle
			}
		}
		select {
		case <-time.After(time.Millisecond * 50):
			// Continue
		case <-ctx.Done():
			return nil
		}
	}
}

// ProcessMessage acts upons a given request.
func (o *servoSwitch) ProcessMessage(ctx context.Context, r mqp.SwitchMessage) error {
	log := o.log.With().Str("direction", string(r.Direction)).Logger()
	log.Debug().Msg("got request")

	switch r.Direction {
	case mqp.SwitchDirectionStraight:
		o.targetAngle = 180
	case mqp.SwitchDirectionOff:
		o.targetAngle = 0
	}

	return nil
}

func (o *servoSwitch) angleToOnOffValues(angle int, maxDevValue int) (int, int) {
	minValue := 120
	maxValue := 550
	step := float64(maxValue-minValue) / 180.0
	on := 0
	off := minValue + int(step*float64(angle))
	return int(on), int(off)
}
