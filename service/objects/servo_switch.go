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
	"sync/atomic"
	"time"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/service/devices"
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
	address model.ObjectAddress
	sender  string
	servo   struct {
		device        devices.PWM
		index         model.DeviceIndex
		straightPL    int
		offPL         int
		stepSize      int
		phaseStraight *phaseRelay
		phaseOff      *phaseRelay
	}
	sendActualNeeded int32
	currentPL        int
	targetPL         int
}

type phaseRelay struct {
	device     devices.GPIO
	pin        model.DeviceIndex
	lastActive bool
}

func (r *phaseRelay) configure(ctx context.Context) error {
	if err := r.device.SetDirection(ctx, r.pin, devices.PinDirectionOutput); err != nil {
		return err
	}
	if err := r.device.Set(ctx, r.pin, false); err != nil {
		return err
	}
	r.lastActive = false
	return nil
}

func (r *phaseRelay) activateRelay(ctx context.Context) error {
	if !r.lastActive {
		if err := r.device.Set(ctx, r.pin, true); err != nil {
			return err
		}
		r.lastActive = true
	}
	return nil
}

func (r *phaseRelay) deactivateRelay(ctx context.Context) error {
	if r.lastActive {
		if err := r.device.Set(ctx, r.pin, false); err != nil {
			return err
		}
		r.lastActive = false
	}
	return nil
}

// newServoSwitch creates a new servo-switch object for the given configuration.
func newServoSwitch(sender string, oid model.ObjectID, address model.ObjectAddress, config model.Object, log zerolog.Logger, devService devices.Service) (Object, error) {
	if config.Type != model.ObjectTypeServoSwitch {
		return nil, model.InvalidArgument("Invalid object type '%s'", config.Type)
	}
	servoConn, servoPin, err := getSinglePin(oid, config, model.ConnectionNameServo)
	if err != nil {
		return nil, err
	}
	servoDev, err := getPWMForPin(servoPin, devService)
	if err != nil {
		return nil, model.InvalidArgument("%s: (connection %s in object %s)", err.Error(), model.ConnectionNameServo, oid)
	}
	straightPL := servoConn.GetIntConfig(model.ConfigKeyServoStraight)
	offPL := servoConn.GetIntConfig(model.ConfigKeyServoOff)
	stepSize := maxInt(1, servoConn.GetIntConfig(model.ConfigKeyServoStep))
	sw := &servoSwitch{
		log:       log,
		config:    config,
		address:   address,
		sender:    sender,
		currentPL: offPL,
		targetPL:  straightPL,
	}
	sw.servo.device = servoDev
	sw.servo.index = servoPin.Index
	sw.servo.straightPL = straightPL
	sw.servo.offPL = offPL
	sw.servo.stepSize = stepSize

	// Phase relay for straight direction
	if _, found := config.ConnectionByName(model.ConnectionNamePhaseStraightRelay); found {
		_, phaseStraightPin, err := getSinglePin(oid, config, model.ConnectionNamePhaseStraightRelay)
		if err != nil {
			return nil, err
		}
		phaseStraightDev, err := getGPIOForPin(phaseStraightPin, devService)
		if err != nil {
			return nil, errors.Wrapf(err, "%s: (pin %s in object %s)", err.Error(), model.ConnectionNamePhaseStraightRelay, oid)
		}
		sw.servo.phaseStraight = &phaseRelay{phaseStraightDev, phaseStraightPin.Index, true}
	}

	// Phase relay for off direction
	if _, found := config.ConnectionByName(model.ConnectionNamePhaseOffRelay); found {
		_, phaseOffPin, err := getSinglePin(oid, config, model.ConnectionNamePhaseOffRelay)
		if err != nil {
			return nil, err
		}
		phaseOffDev, err := getGPIOForPin(phaseOffPin, devService)
		if err != nil {
			return nil, errors.Wrapf(err, "%s: (pin %s in object %s)", err.Error(), model.ConnectionNamePhaseOffRelay, oid)
		}
		sw.servo.phaseOff = &phaseRelay{phaseOffDev, phaseOffPin.Index, true}
	}

	return sw, nil
}

// Return the type of this object.
func (o *servoSwitch) Type() *ObjectType {
	return switchType
}

// Configure is called once to put the object in the desired state.
func (o *servoSwitch) Configure(ctx context.Context) error {
	o.targetPL = o.servo.straightPL
	o.currentPL = (o.servo.straightPL + o.servo.offPL) / 2
	o.sendActualNeeded = 1

	if r := o.servo.phaseStraight; r != nil {
		if err := r.configure(ctx); err != nil {
			return err
		}
	}
	if r := o.servo.phaseOff; r != nil {
		if err := r.configure(ctx); err != nil {
			return err
		}
	}
	if err := o.servo.device.Set(ctx, o.servo.index, 0, o.currentPL); err != nil {
		return err
	}
	time.Sleep(time.Millisecond)
	return nil
}

// Run the object until the given context is cancelled.
func (o *servoSwitch) Run(ctx context.Context, requests RequestService, statuses StatusService, moduleID string) error {
	defer o.log.Debug().Msg("servoSwitch.Run terminated")
	for {
		targetPL := o.targetPL
		if targetPL != o.currentPL {
			// Ensure all phase relays are deactivated
			if r := o.servo.phaseStraight; r != nil {
				if err := r.deactivateRelay(ctx); err != nil {
					o.log.Warn().Err(err).Msg("Failed to deactivate phase straight array")
				}
			}
			if r := o.servo.phaseOff; r != nil {
				if err := r.deactivateRelay(ctx); err != nil {
					o.log.Warn().Err(err).Msg("Failed to deactivate phase off array")
				}
			}
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
		} else {
			// Requested position reached
			targetDirection := model.SwitchDirection_STRAIGHT
			if o.targetPL == o.servo.offPL {
				targetDirection = model.SwitchDirection_OFF
			}
			currentDirection := model.SwitchDirection_STRAIGHT
			if o.currentPL == o.servo.offPL {
				currentDirection = model.SwitchDirection_OFF
			}
			// Set phase relays
			if r := o.servo.phaseStraight; r != nil && currentDirection == model.SwitchDirection_STRAIGHT {
				if err := r.activateRelay(ctx); err != nil {
					o.log.Warn().Err(err).Msg("Failed to deactivate phase straight array")
				}
			}
			if r := o.servo.phaseOff; r != nil && currentDirection == model.SwitchDirection_OFF {
				if err := r.activateRelay(ctx); err != nil {
					o.log.Warn().Err(err).Msg("Failed to deactivate phase off array")
				}
			}
			// Send actual message (if needed)
			sendNeeded := atomic.CompareAndSwapInt32(&o.sendActualNeeded, 1, 0)
			if sendNeeded {
				msg := model.Switch{
					Address: o.address,
					Request: &model.SwitchState{
						Direction: targetDirection,
					},
					Actual: &model.SwitchState{
						Direction: currentDirection,
					},
				}
				statuses.PublishSwitchActual(msg)
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
func (o *servoSwitch) ProcessMessage(ctx context.Context, r model.Switch) error {
	direction := r.GetRequest().GetDirection()
	log := o.log.With().Str("direction", string(direction)).Logger()
	log.Debug().Msg("got request")

	switch direction {
	case model.SwitchDirection_STRAIGHT:
		o.targetPL = o.servo.straightPL
	case model.SwitchDirection_OFF:
		o.targetPL = o.servo.offPL
	}
	atomic.StoreInt32(&o.sendActualNeeded, 1)

	return nil
}

// ProcessPowerMessage acts upons a given power message.
func (o *servoSwitch) ProcessPowerMessage(ctx context.Context, m model.PowerState) error {
	if m.GetEnabled() {
		atomic.StoreInt32(&o.sendActualNeeded, 1)
	}
	return nil
}
