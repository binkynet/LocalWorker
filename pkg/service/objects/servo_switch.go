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
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/pkg/service/devices"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	_ switchAPI = &servoSwitch{}
)

const (
	relaySettleTimeout = time.Millisecond * 500
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
		straightPL    uint32
		offPL         uint32
		stepSize      int
		phaseStraight *phaseRelay
		phaseOff      *phaseRelay
	}
	sendActualNeeded int32
	currentPL        uint32
	targetPL         uint32
}

type phaseRelay struct {
	device           devices.GPIO
	pin              model.DeviceIndex
	lastActive       bool
	invert           bool
	mqttStateTopic   string
	mqttCommandTopic string
}

func (r *phaseRelay) configure(ctx context.Context) error {
	if err := r.device.SetDirection(ctx, r.pin, devices.PinDirectionOutput); err != nil {
		return err
	}
	if err := r.device.Set(ctx, r.pin, r.pinValue(false)); err != nil {
		return err
	}
	if mqtt, ok := r.device.(devices.MQTT); ok {
		if err := mqtt.SetStateTopic(r.pin, r.mqttStateTopic); err != nil {
			return err
		}
		if err := mqtt.SetCommandTopic(r.pin, r.mqttCommandTopic); err != nil {
			return err
		}
	}
	r.lastActive = false
	return nil
}

// Activate a relay
// Returns: changed, error
func (r *phaseRelay) activateRelay(ctx context.Context) (bool, error) {
	if !r.lastActive {
		if err := r.device.Set(ctx, r.pin, r.pinValue(true)); err != nil {
			return true, err
		}
		r.lastActive = true
		return true, nil
	}
	return false, nil
}

// Deactivate a relay
// Returns: changed, error
func (r *phaseRelay) deactivateRelay(ctx context.Context) (bool, error) {
	if r.lastActive {
		if err := r.device.Set(ctx, r.pin, r.pinValue(false)); err != nil {
			return true, err
		}
		r.lastActive = false
		return true, nil
	}
	return false, nil
}

func (r *phaseRelay) pinValue(value bool) bool {
	if r.invert {
		return !value
	}
	return value
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
	straightPL := uint32(servoConn.GetIntConfig(model.ConfigKeyServoStraight))
	offPL := uint32(servoConn.GetIntConfig(model.ConfigKeyServoOff))
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
	if mqtt, ok := servoDev.(devices.MQTT); ok {
		if err := mqtt.SetStateTopic(servoPin.Index, servoConn.GetStringConfig(model.ConfigKeyMQTTStateTopic)); err != nil {
			return nil, err
		}
		if err := mqtt.SetCommandTopic(servoPin.Index, servoConn.GetStringConfig(model.ConfigKeyMQTTCommandTopic)); err != nil {
			return nil, err
		}
	}

	// Phase relay for straight direction
	if _, found := config.ConnectionByName(model.ConnectionNamePhaseStraightRelay); found {
		conn, phaseStraightPin, err := getSinglePin(oid, config, model.ConnectionNamePhaseStraightRelay)
		if err != nil {
			return nil, err
		}
		phaseStraightDev, err := getGPIOForPin(phaseStraightPin, devService)
		if err != nil {
			return nil, errors.Wrapf(err, "%s: (pin %s in object %s)", err.Error(), model.ConnectionNamePhaseStraightRelay, oid)
		}
		invert := conn.GetBoolConfig(model.ConfigKeyInvert)
		mqttStateTopic := conn.GetStringConfig(model.ConfigKeyMQTTStateTopic)
		mqttCommandTopic := conn.GetStringConfig(model.ConfigKeyMQTTCommandTopic)
		sw.servo.phaseStraight = &phaseRelay{phaseStraightDev, phaseStraightPin.Index, true, invert, mqttStateTopic, mqttCommandTopic}
	}

	// Phase relay for off direction
	if _, found := config.ConnectionByName(model.ConnectionNamePhaseOffRelay); found {
		conn, phaseOffPin, err := getSinglePin(oid, config, model.ConnectionNamePhaseOffRelay)
		if err != nil {
			return nil, err
		}
		phaseOffDev, err := getGPIOForPin(phaseOffPin, devService)
		if err != nil {
			return nil, errors.Wrapf(err, "%s: (pin %s in object %s)", err.Error(), model.ConnectionNamePhaseOffRelay, oid)
		}
		invert := conn.GetBoolConfig(model.ConfigKeyInvert)
		mqttStateTopic := conn.GetStringConfig(model.ConfigKeyMQTTStateTopic)
		mqttCommandTopic := conn.GetStringConfig(model.ConfigKeyMQTTCommandTopic)
		sw.servo.phaseOff = &phaseRelay{phaseOffDev, phaseOffPin.Index, true, invert, mqttStateTopic, mqttCommandTopic}
	}

	return sw, nil
}

// Return the type of this object.
func (o *servoSwitch) Type() ObjectType {
	return switchTypeInstance
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
	enabled := true
	if err := o.servo.device.SetPWM(ctx, o.servo.index, 0, o.currentPL, enabled, false); err != nil {
		return err
	}
	time.Sleep(time.Millisecond * 100)
	enabled = false
	if err := o.servo.device.SetPWM(ctx, o.servo.index, 0, o.currentPL, enabled, true); err != nil {
		return err
	}
	return nil
}

type servoSwitchRunState byte

const (
	// In the idle state, the relays are activated in
	// the current position and we keep sending our
	// actual position.
	servoSwitchRunStateIdle servoSwitchRunState = iota
	// In this state, we de-activate the relays and wait for them to settle.
	servoSwitchRunStateDeactivateRelays
	// In this state, we move the servo closer to the target position.
	servoSwitchRunStateMoveServo
	// In this state, the servo was told to move to its final
	// position and we're giving it time to get there.
	servoSwitchRunStateServoSettle
	// In this state, we assume the servo has reached its target
	// position and we'll disable its motor.
	servoSwitchRunStateDisableServo
	// In this state, we activate the relays and wait for them to settle.
	servoSwitchRunStateActivateRelays
)

// Run the object until the given context is cancelled.
func (o *servoSwitch) Run(ctx context.Context, requests RequestService, statuses StatusService, moduleID string) error {
	defer o.log.Debug().Msg("servoSwitch.Run terminated")
	// Ensure we initialize directly after start
	atomic.StoreInt32(&o.sendActualNeeded, 1)
	lastSendActual := time.Now()
	state := servoSwitchRunStateIdle
	targetPL := atomic.LoadUint32(&o.targetPL)
	targetDirection := model.SwitchDirection_STRAIGHT
	var startServoMoveTime time.Time
	var servoSettleDoneTime time.Time
	for {
		switch state {
		// In the idle state, the relays are activated in
		// the current position and we keep sending our
		// actual position.
		case servoSwitchRunStateIdle:
			// Fetch new target (if any)
			targetPL = atomic.LoadUint32(&o.targetPL)
			// Calculate target direction
			targetDirection = model.SwitchDirection_STRAIGHT
			if targetPL == o.servo.offPL {
				targetDirection = model.SwitchDirection_OFF
			}

			// Should we move?
			if targetPL != o.currentPL {
				// Yes we should move
				state = servoSwitchRunStateDeactivateRelays
			} else {
				// No need to move, keep sending our actual position
				currentDirection := model.SwitchDirection_STRAIGHT
				if o.currentPL == o.servo.offPL {
					currentDirection = model.SwitchDirection_OFF
				}
				// Send actual message (if needed)
				sendNeeded := atomic.CompareAndSwapInt32(&o.sendActualNeeded, 1, 0)
				if sendNeeded {
					o.log.Debug().
						Uint32("pulse", o.currentPL).
						Msg("Servo reached state, sending actual")
				} else if time.Since(lastSendActual) > time.Second*5 {
					// We need to keep sending actual status on regular interval
					sendNeeded = true
					lastSendActual = time.Now()
				}
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
		// In this state, we de-activate the relays and wait for them to settle.
		case servoSwitchRunStateDeactivateRelays:
			// Ensure all phase relays are deactivated
			relaysChanged := false
			if r := o.servo.phaseStraight; r != nil {
				if changed, err := r.deactivateRelay(ctx); err != nil {
					o.log.Warn().Err(err).Msg("Failed to deactivate phase straight array")
				} else {
					relaysChanged = relaysChanged || changed
				}
			}
			if r := o.servo.phaseOff; r != nil {
				if changed, err := r.deactivateRelay(ctx); err != nil {
					o.log.Warn().Err(err).Msg("Failed to deactivate phase off array")
				} else {
					relaysChanged = relaysChanged || changed
				}
			}
			// Wait a bit to settle relays if they changed
			if relaysChanged {
				time.Sleep(relaySettleTimeout)
			}
			// Move to next state
			startServoMoveTime = time.Now()
			state = servoSwitchRunStateMoveServo
		// In this state, we move the servo closer to the target position.
		case servoSwitchRunStateMoveServo:
			// First check if we reached the target position
			if targetPL == o.currentPL {
				// Yes the last servo position we send was the final one
				// Calculate the time to settle
				moveDuration := time.Since(startServoMoveTime)
				// We assume the servo is settled after another have of the move time
				servoSettleDoneTime = time.Now().Add(moveDuration / 2)
				// Switch to the settle state
				state = servoSwitchRunStateServoSettle
			} else {
				// We'll make the current pulse length closer to target pulse length.
				step := minInt(o.servo.stepSize, absInt(int(targetPL)-int(o.currentPL)))
				var nextPL uint32
				if o.currentPL < targetPL {
					nextPL = o.currentPL + uint32(step)
				} else {
					nextPL = o.currentPL - uint32(step)
				}
				finalState := nextPL == targetPL
				if err := o.servo.device.SetPWM(ctx, o.servo.index, 0, nextPL, true, finalState); err != nil {
					// oops
					o.log.Warn().
						Err(err).
						Uint32("pulse", nextPL).
						Msg("Set servo failed")
				} else {
					o.currentPL = nextPL
					o.log.Debug().
						Uint32("pulse", nextPL).
						Str("servo_dev", fmt.Sprintf("%T", o.servo.device)).
						Msg("Set servo succeeded")
				}
			}
		// In this state, the servo was told to move to its final
		// position and we're giving it time to get there.
		case servoSwitchRunStateServoSettle:
			// Did we reach the settle time?
			if time.Now().After(servoSettleDoneTime) {
				// Yes, we now assume the servo has settled.
				state = servoSwitchRunStateDisableServo
			} else {
				// We wait a bit more
			}
		// In this state, we assume the servo has reached its target
		// position and we'll disable its motor.
		case servoSwitchRunStateDisableServo:
			// Relays have settled, now we can disactivate the servo.
			if err := o.servo.device.SetPWM(ctx, o.servo.index, 0, o.currentPL, false, true); err != nil {
				// oops
				o.log.Warn().
					Err(err).
					Uint32("pulse", o.currentPL).
					Msg("Set servo (disabled) failed")
			}
			state = servoSwitchRunStateActivateRelays
			// In this state, we activate the relays and wait for them to settle.
		case servoSwitchRunStateActivateRelays:
			// Set phase relays
			relaysChanged := false
			if r := o.servo.phaseStraight; r != nil && targetDirection == model.SwitchDirection_STRAIGHT {
				if changed, err := r.activateRelay(ctx); err != nil {
					o.log.Warn().Err(err).Msg("Failed to activate phase straight array")
				} else {
					relaysChanged = relaysChanged || changed
				}
				//o.log.Debug().Msg("Activated straight")
			}
			if r := o.servo.phaseOff; r != nil && targetDirection == model.SwitchDirection_OFF {
				if changed, err := r.activateRelay(ctx); err != nil {
					o.log.Warn().Err(err).Msg("Failed to activate phase off array")
				} else {
					relaysChanged = relaysChanged || changed
				}
				//o.log.Debug().Msg("Activated off")
			}
			// Wait a bit to settle relays if they changed
			if relaysChanged {
				time.Sleep(relaySettleTimeout)
			}
			// Ensure we send an update next
			atomic.StoreInt32(&o.sendActualNeeded, 1)
			// Move to idle state
			state = servoSwitchRunStateIdle
		}

		select {
		case <-ctx.Done():
			// Context canceled
			return nil
		case <-time.After(time.Millisecond * 50):
			// Continue
		}
	}
}

// ProcessMessage acts upons a given request.
func (o *servoSwitch) ProcessMessage(ctx context.Context, r model.Switch) error {
	direction := r.GetRequest().GetDirection()

	var targetPL uint32
	switch direction {
	case model.SwitchDirection_STRAIGHT:
		targetPL = o.servo.straightPL
	case model.SwitchDirection_OFF:
		targetPL = o.servo.offPL
	default:
		return fmt.Errorf("unknown switch direction %d", direction)
	}
	oldTargetPL := atomic.SwapUint32(&o.targetPL, targetPL)
	if oldTargetPL != targetPL {
		atomic.StoreInt32(&o.sendActualNeeded, 1)
		log := o.log.With().Str("direction", direction.String()).Logger()
		log.Debug().Msg("got new servo-switch request")
	}

	return nil
}

// ProcessPowerMessage acts upons a given power message.
func (o *servoSwitch) ProcessPowerMessage(ctx context.Context, m model.PowerState) error {
	if m.GetEnabled() {
		atomic.StoreInt32(&o.sendActualNeeded, 1)
	}
	return nil
}
