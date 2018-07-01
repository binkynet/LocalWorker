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
		device     devices.PWM
		index      model.DeviceIndex
		straightPL int
		offPL      int
		stepSize   int
	}
	currentPL int
	targetPL  int
}

// newServoSwitch creates a new servo-switch object for the given configuration.
func newServoSwitch(oid model.ObjectID, address mqp.ObjectAddress, config model.Object, log zerolog.Logger, devService devices.Service) (Object, error) {
	if config.Type != model.ObjectTypeServoSwitch {
		return nil, errors.Wrapf(model.ValidationError, "Invalid object type '%s'", config.Type)
	}
	servoConn, servoPin, err := getSinglePin(oid, config, model.ConnectionNameServo)
	if err != nil {
		return nil, maskAny(err)
	}
	servoDev, err := getPWMForPin(servoPin, devService)
	if err != nil {
		return nil, errors.Wrapf(err, "%s: (connection %s in object %s)", err.Error(), model.ConnectionNameStraightRelay, oid)
	}
	straightPL := servoConn.GetIntConfig("straight", 150)
	offPL := servoConn.GetIntConfig("off", 550)
	stepSize := maxInt(1, servoConn.GetIntConfig("step", 5))
	sw := &servoSwitch{
		log:       log,
		config:    config,
		address:   address,
		currentPL: offPL,
		targetPL:  straightPL,
	}
	sw.servo.device = servoDev
	sw.servo.index = servoPin.Index
	sw.servo.straightPL = straightPL
	sw.servo.offPL = offPL
	sw.servo.stepSize = stepSize
	return sw, nil
}

// Return the type of this object.
func (o *servoSwitch) Type() *ObjectType {
	return switchType
}

// Configure is called once to put the object in the desired state.
func (o *servoSwitch) Configure(ctx context.Context) error {
	o.targetPL = o.servo.straightPL
	o.currentPL = o.servo.straightPL

	if err := o.servo.device.Set(ctx, o.servo.index, 0, o.servo.straightPL); err != nil {
		return maskAny(err)
	}
	time.Sleep(time.Millisecond)
	return nil
}

// Run the object until the given context is cancelled.
func (o *servoSwitch) Run(ctx context.Context, mqttService mqtt.Service, topicPrefix string) error {
	defer o.log.Debug().Msg("servoSwitch.Run terminated")
	for {
		targetPL := o.targetPL
		if targetPL != o.currentPL {
			// Make current pulse length closer to target pulse length
			step := minInt(o.servo.stepSize, absInt(targetPL-o.currentPL))
			var nextPL int
			if o.currentPL < targetPL {
				nextPL = o.currentPL + step
			} else {
				nextPL = o.currentPL - step
			}
			o.log.Debug().
				Int("pulse", nextPL).
				Msg("Set servo")
			if err := o.servo.device.Set(ctx, o.servo.index, 0, nextPL); err != nil {
				// oops
				o.log.Debug().
					Err(err).
					Int("pulse", nextPL).
					Msg("Set servo failed")
			} else {
				o.currentPL = nextPL
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
		o.targetPL = o.servo.straightPL
	case mqp.SwitchDirectionOff:
		o.targetPL = o.servo.offPL
	}

	return nil
}
