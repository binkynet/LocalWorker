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
	"github.com/rs/zerolog"
)

var (
	_ switchAPI = &relaySwitch{}
)

type relaySwitch struct {
	mutex              sync.Mutex
	log                zerolog.Logger
	config             model.Object
	address            model.ObjectAddress
	sender             string
	straight           relaySwitchDirection
	off                relaySwitchDirection
	disableAllNeeded   bool
	disableAllTime     time.Time
	sendActualNeeded   int32
	requestedDirection model.SwitchDirection
}

type relaySwitchDirection struct {
	device devices.GPIO
	pin    model.DeviceIndex
}

func (rsd relaySwitchDirection) activateRelay(ctx context.Context) error {
	if err := rsd.device.Set(ctx, rsd.pin, true); err != nil {
		return err
	}
	return nil
}

func (rsd relaySwitchDirection) deactivateRelay(ctx context.Context) error {
	if err := rsd.device.Set(ctx, rsd.pin, false); err != nil {
		return err
	}
	return nil
}

// newRelaySwitch creates a new relay-switch object for the given configuration.
func newRelaySwitch(sender string, oid model.ObjectID, address model.ObjectAddress, config model.Object, log zerolog.Logger, devService devices.Service) (Object, error) {
	if config.Type != model.ObjectTypeRelaySwitch {
		return nil, model.InvalidArgument("Invalid object type '%s'", config.Type)
	}
	_, straightPin, err := getSinglePin(oid, config, model.ConnectionNameStraightRelay)
	if err != nil {
		return nil, err
	}
	straightDev, err := getGPIOForPin(straightPin, devService)
	if err != nil {
		return nil, model.InvalidArgument("%s: (pin %s in object %s)", err.Error(), model.ConnectionNameStraightRelay, oid)
	}
	_, offPin, err := getSinglePin(oid, config, model.ConnectionNameOffRelay)
	if err != nil {
		return nil, err
	}
	offDev, err := getGPIOForPin(offPin, devService)
	if err != nil {
		return nil, model.InvalidArgument("%s: (pin %s in object %s)", err.Error(), model.ConnectionNameOffRelay, oid)
	}
	return &relaySwitch{
		log:      log,
		config:   config,
		address:  address,
		sender:   sender,
		straight: relaySwitchDirection{straightDev, straightPin.Index},
		off:      relaySwitchDirection{offDev, offPin.Index},
	}, nil
}

// Return the type of this object.
func (o *relaySwitch) Type() *ObjectType {
	return switchType
}

// Configure is called once to put the object in the desired state.
func (o *relaySwitch) Configure(ctx context.Context) error {
	if err := o.straight.device.SetDirection(ctx, o.straight.pin, devices.PinDirectionOutput); err != nil {
		return err
	}
	if err := o.straight.deactivateRelay(ctx); err != nil {
		return err
	}
	if err := o.off.device.SetDirection(ctx, o.off.pin, devices.PinDirectionOutput); err != nil {
		return err
	}
	if err := o.off.deactivateRelay(ctx); err != nil {
		return err
	}
	if err := o.switchTo(ctx, model.SwitchDirection_STRAIGHT); err != nil {
		return err
	}
	return nil
}

// Run the object until the given context is cancelled.
func (o *relaySwitch) Run(ctx context.Context, requests RequestService, statuses StatusService, moduleID string) error {
	for {
		o.mutex.Lock()
		var sendActualNeeded bool
		if o.disableAllNeeded {
			if time.Now().After(o.disableAllTime) {
				hasErrors := false
				if err := o.straight.deactivateRelay(ctx); err != nil {
					o.log.Warn().Err(err).Msg("Straight relay did not de-activate")
					hasErrors = true
				}
				if err := o.off.deactivateRelay(ctx); err != nil {
					o.log.Warn().Err(err).Msg("Off relay did not de-activate")
					hasErrors = true
				}
				if !hasErrors {
					o.disableAllNeeded = false
				}
			}
		} else {
			sendActualNeeded = atomic.CompareAndSwapInt32(&o.sendActualNeeded, 1, 0)
		}
		o.mutex.Unlock()
		if sendActualNeeded {
			msg := model.Switch{
				Address:   o.address,
				Direction: o.requestedDirection,
			}
			statuses.PublishSwitchActual(msg)
		}
		select {
		case <-time.After(time.Millisecond * 10):
			// Continue
		case <-ctx.Done():
			if o.disableAllNeeded {
				// Do one more loop to disable relays
				o.disableAllTime = time.Now().Add(-time.Second)
			} else {
				return nil
			}
		}
	}
}

// ProcessMessage acts upons a given request.
func (o *relaySwitch) ProcessMessage(ctx context.Context, r model.Switch) error {
	log := o.log.With().Str("direction", string(r.Direction)).Logger()
	log.Debug().Msg("got request")

	o.mutex.Lock()
	defer o.mutex.Unlock()
	if err := o.switchTo(ctx, r.Direction); err != nil {
		return err
	}

	return nil
}

// switchTo initiates the switch action.
// The mutex must be held when calling this function.
func (o *relaySwitch) switchTo(ctx context.Context, direction model.SwitchDirection) error {
	atomic.StoreInt32(&o.sendActualNeeded, 1)
	switch direction {
	case model.SwitchDirection_STRAIGHT:
		if err := o.off.deactivateRelay(ctx); err != nil {
			return err
		}
		if err := o.straight.activateRelay(ctx); err != nil {
			return err
		}
	case model.SwitchDirection_OFF:
		if err := o.straight.deactivateRelay(ctx); err != nil {
			return err
		}
		if err := o.off.activateRelay(ctx); err != nil {
			return err
		}
	}
	o.disableAllNeeded = true
	o.disableAllTime = time.Now().Add(time.Second)
	o.requestedDirection = direction

	return nil
}

// ProcessPowerMessage acts upons a given power message.
func (o *relaySwitch) ProcessPowerMessage(ctx context.Context, m model.Power) error {
	if m.Enabled {
		atomic.StoreInt32(&o.sendActualNeeded, 1)
	}
	return nil
}
