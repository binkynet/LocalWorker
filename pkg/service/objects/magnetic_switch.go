// Copyright 2023 Ewout Prangsma
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
	"github.com/binkynet/LocalWorker/pkg/service/devices"
	"github.com/rs/zerolog"
)

var (
	_ switchAPI = &magneticSwitch{}
)

type magneticSwitch struct {
	mutex              sync.Mutex
	log                zerolog.Logger
	config             model.Object
	address            model.ObjectAddress
	sender             string
	straight           magneticSwitchMagnet
	off                magneticSwitchMagnet
	disableAllNeeded   bool
	sendActualNeeded   int32
	requestedDirection model.SwitchDirection
}

type magneticSwitchMagnet struct {
	a, b magneticSwitchMagnetPin
}

type magneticSwitchMagnetPin struct {
	device devices.GPIO
	pin    model.DeviceIndex
	invert bool
}

// configure is called once to put the object in the desired state.
func (msm magneticSwitchMagnet) configure(ctx context.Context) error {
	if err := msm.a.device.SetDirection(ctx, msm.a.pin, devices.PinDirectionOutput); err != nil {
		return err
	}
	if err := msm.b.device.SetDirection(ctx, msm.b.pin, devices.PinDirectionOutput); err != nil {
		return err
	}
	return nil
}

func (msm magneticSwitchMagnet) activateMagnet(ctx context.Context, forward bool) error {
	if err := msm.a.device.Set(ctx, msm.a.pin, msm.a.pinValue(forward)); err != nil {
		return err
	}
	if err := msm.b.device.Set(ctx, msm.b.pin, msm.b.pinValue(!forward)); err != nil {
		return err
	}
	return nil
}

func (msm magneticSwitchMagnet) deactivateMagnet(ctx context.Context) error {
	if err := msm.a.device.Set(ctx, msm.a.pin, msm.a.pinValue(false)); err != nil {
		return err
	}
	if err := msm.b.device.Set(ctx, msm.b.pin, msm.b.pinValue(false)); err != nil {
		return err
	}
	return nil
}

func (msmp *magneticSwitchMagnetPin) initialize(connName model.ConnectionName, oid model.ObjectID, config model.Object, devService devices.Service) error {
	conn, pin, err := getSinglePin(oid, config, connName)
	if err != nil {
		return model.InvalidArgument("failed to get pin for '%s': %w", connName, err)
	}
	device, err := getGPIOForPin(pin, devService)
	if err != nil {
		return model.InvalidArgument("%s: (pin %s in object %s)", err.Error(), model.ConnectionNameStraightRelay, oid)
	}
	msmp.device = device
	msmp.pin = pin.Index
	msmp.invert = conn.GetBoolConfig(model.ConfigKeyInvert)
	return nil
}

func (msmp magneticSwitchMagnetPin) pinValue(value bool) bool {
	if msmp.invert {
		return !value
	}
	return value
}

// newRelaySwitch creates a new relay-switch object for the given configuration.
func newMagneticSwitch(sender string, oid model.ObjectID, address model.ObjectAddress, config model.Object, log zerolog.Logger, devService devices.Service) (Object, error) {
	if config.Type != model.ObjectTypeMagneticSwitch {
		return nil, model.InvalidArgument("Invalid object type '%s'", config.Type)
	}
	ms := &magneticSwitch{
		log:     log,
		config:  config,
		address: address,
		sender:  sender,
	}
	if err := ms.straight.a.initialize(model.ConnectionNameMagneticStraightA, oid, config, devService); err != nil {
		return nil, err
	}
	if err := ms.straight.b.initialize(model.ConnectionNameMagneticStraightB, oid, config, devService); err != nil {
		return nil, err
	}
	if err := ms.off.a.initialize(model.ConnectionNameMagneticOffA, oid, config, devService); err != nil {
		return nil, err
	}
	if err := ms.off.b.initialize(model.ConnectionNameMagneticOffB, oid, config, devService); err != nil {
		return nil, err
	}
	return ms, nil
}

// Return the type of this object.
func (o *magneticSwitch) Type() ObjectType {
	return switchTypeInstance
}

// Configure is called once to put the object in the desired state.
func (o *magneticSwitch) Configure(ctx context.Context) error {
	if err := o.straight.configure(ctx); err != nil {
		return err
	}
	if err := o.off.configure(ctx); err != nil {
		return err
	}
	return nil
}

// Run the object until the given context is cancelled.
func (o *magneticSwitch) Run(ctx context.Context, requests RequestService, statuses StatusService, moduleID string) error {
	for {
		o.mutex.Lock()
		var sendActualNeeded bool
		if o.disableAllNeeded {
			hasErrors := false
			if err := o.straight.deactivateMagnet(ctx); err != nil {
				o.log.Warn().Err(err).Msg("Straight relay did not de-activate")
				hasErrors = true
			}
			if err := o.off.deactivateMagnet(ctx); err != nil {
				o.log.Warn().Err(err).Msg("Off relay did not de-activate")
				hasErrors = true
			}
			if !hasErrors {
				o.disableAllNeeded = false
			}
		} else {
			sendActualNeeded = atomic.CompareAndSwapInt32(&o.sendActualNeeded, 1, 0)
		}
		o.mutex.Unlock()
		if sendActualNeeded {
			msg := model.Switch{
				Address: o.address,
				Request: &model.SwitchState{
					Direction: o.requestedDirection,
				},
				Actual: &model.SwitchState{
					Direction: o.requestedDirection,
				},
			}
			statuses.PublishSwitchActual(msg)
		}
		select {
		case <-ctx.Done():
			if o.disableAllNeeded {
				// Do one more loop to disable relays
			} else {
				return nil
			}
		case <-time.After(time.Millisecond * 10):
			// Continue
		}
	}
}

// ProcessMessage acts upons a given request.
func (o *magneticSwitch) ProcessMessage(ctx context.Context, r model.Switch) error {
	direction := r.GetRequest().GetDirection()
	enabled := r.GetRequest().GetIsUsed() || true // TODO remove true
	log := o.log.With().
		Str("direction", direction.String()).
		Bool("enabled", enabled).
		Logger()
	log.Debug().Msg("got magnetic-switch request")

	o.mutex.Lock()
	defer o.mutex.Unlock()
	if err := o.switchTo(ctx, direction, enabled); err != nil {
		return err
	}

	return nil
}

// switchTo initiates the switch action.
// The mutex must be held when calling this function.
func (o *magneticSwitch) switchTo(ctx context.Context, direction model.SwitchDirection, enabled bool) error {
	atomic.StoreInt32(&o.sendActualNeeded, 1)
	if err := o.straight.deactivateMagnet(ctx); err != nil {
		return err
	}
	if err := o.off.deactivateMagnet(ctx); err != nil {
		return err
	}
	if enabled {
		switch direction {
		case model.SwitchDirection_STRAIGHT:
			if err := o.off.activateMagnet(ctx, false); err != nil {
				return err
			}
			if err := o.straight.activateMagnet(ctx, true); err != nil {
				return err
			}
		case model.SwitchDirection_OFF:
			if err := o.straight.activateMagnet(ctx, false); err != nil {
				return err
			}
			if err := o.off.activateMagnet(ctx, true); err != nil {
				return err
			}
		}
	}
	o.requestedDirection = direction

	return nil
}

// ProcessPowerMessage acts upons a given power message.
func (o *magneticSwitch) ProcessPowerMessage(ctx context.Context, m model.PowerState) error {
	if m.GetEnabled() {
		atomic.StoreInt32(&o.sendActualNeeded, 1)
	}
	return nil
}
