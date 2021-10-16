// Copyright 2021 Ewout Prangsma
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
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	trackInverterType = &ObjectType{
		Run: func(ctx context.Context, log zerolog.Logger, requests RequestService, statuses StatusService, service Service, moduleID string) error {
			cancel := requests.RegisterOutputRequestReceiver(func(msg model.Output) error {
				log := log.With().Str("address", string(msg.Address)).Logger()
				//log.Debug().Msg("got message")
				if obj, found := service.ObjectByAddress(msg.Address); found {
					if x, ok := obj.(*trackInverter); ok {
						if err := x.ProcessMessage(ctx, msg); err != nil {
							return err
						}
					} else {
						return errors.Errorf("Expected object of type trackInverter")
					}
				} else {
					log.Debug().Msg("object not found")
				}
				return nil
			})
			defer cancel()
			<-ctx.Done()
			return nil
		},
	}
)

type trackInverter struct {
	log              zerolog.Logger
	address          model.ObjectAddress
	relayOutAInA     *phaseRelay
	relayOutAInB     *phaseRelay
	relayOutBInA     *phaseRelay
	relayOutBInB     *phaseRelay
	targetState      model.TrackInverterState
	currentState     model.TrackInverterState
	sendActualNeeded int32
}

// newTrackInverter creates a new track-inverter object for the given configuration.
func newTrackInverter(sender string, oid model.ObjectID, address model.ObjectAddress, config model.Object, log zerolog.Logger, devService devices.Service) (Object, error) {
	if config.Type != model.ObjectTypeTrackInverter {
		return nil, model.InvalidArgument("Invalid object type '%s'", config.Type)
	}
	getGPIOAndPin := func(connectionName model.ConnectionName) (*phaseRelay, error) {
		conn, ok := config.ConnectionByName(connectionName)
		if !ok {
			return nil, model.InvalidArgument("Pin '%s' not found in object '%s'", connectionName, oid)
		}
		if len(conn.Pins) != 1 {
			return nil, model.InvalidArgument("Pin '%s' must have 1 pin in object '%s', got %d", connectionName, oid, len(conn.Pins))
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
			return nil, model.InvalidArgument("Pin '%s' in object '%s' is out of range. Got %d. Range [1..%d]", connectionName, oid, pin, gpio.PinCount())
		}
		return &phaseRelay{
			device: gpio,
			pin:    pin,
		}, nil
	}
	obj := &trackInverter{
		log:     log,
		address: address,
	}
	var err error
	if obj.relayOutAInA, err = getGPIOAndPin(model.ConnectionNameRelayOutAInA); err != nil {
		return nil, err
	}
	if obj.relayOutAInB, err = getGPIOAndPin(model.ConnectionNameRelayOutAInB); err != nil {
		return nil, err
	}
	if obj.relayOutBInA, err = getGPIOAndPin(model.ConnectionNameRelayOutBInA); err != nil {
		return nil, err
	}
	if obj.relayOutBInB, err = getGPIOAndPin(model.ConnectionNameRelayOutBInB); err != nil {
		return nil, err
	}
	return obj, nil
}

// Return the type of this object.
func (o *trackInverter) Type() *ObjectType {
	return trackInverterType
}

// Configure is called once to put the object in the desired state.
func (o *trackInverter) Configure(ctx context.Context) error {
	for i := 0; i < 5; i++ {
		if err := o.relayOutAInA.configure(ctx); err != nil {
			return err
		}
		if err := o.relayOutAInB.configure(ctx); err != nil {
			return err
		}
		if err := o.relayOutBInA.configure(ctx); err != nil {
			return err
		}
		if err := o.relayOutBInB.configure(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Run the object until the given context is cancelled.
func (o *trackInverter) Run(ctx context.Context, requests RequestService, statuses StatusService, moduleID string) error {
	defer o.log.Debug().Msg("trackInverter.Run terminated")
	initialized := false
	for {
		if !initialized || o.targetState != o.currentState {
			// First deactivate all relays
			if r := o.relayOutAInA; r != nil {
				if err := r.deactivateRelay(ctx); err != nil {
					o.log.Warn().Err(err).Msg("Failed to deactivate relayOutAInA")
				}
			}
			if r := o.relayOutAInB; r != nil {
				if err := r.deactivateRelay(ctx); err != nil {
					o.log.Warn().Err(err).Msg("Failed to deactivate relayOutAInA")
				}
			}
			if r := o.relayOutBInA; r != nil {
				if err := r.deactivateRelay(ctx); err != nil {
					o.log.Warn().Err(err).Msg("Failed to deactivate relayOutAInA")
				}
			}
			if r := o.relayOutBInB; r != nil {
				if err := r.deactivateRelay(ctx); err != nil {
					o.log.Warn().Err(err).Msg("Failed to deactivate relayOutAInA")
				}
			}

			// Now set the desired relays
			if o.targetState == model.TrackInverterStateDefault {
				if r := o.relayOutAInA; r != nil {
					if err := r.activateRelay(ctx); err != nil {
						o.log.Warn().Err(err).Msg("Failed to activate relayOutAInA")
					}
				}
				if r := o.relayOutBInB; r != nil {
					if err := r.activateRelay(ctx); err != nil {
						o.log.Warn().Err(err).Msg("Failed to activate relayOutAInA")
					}
				}
			} else if o.targetState == model.TrackInverterStateReverse {
				if r := o.relayOutAInB; r != nil {
					if err := r.activateRelay(ctx); err != nil {
						o.log.Warn().Err(err).Msg("Failed to activate relayOutAInA")
					}
				}
				if r := o.relayOutBInA; r != nil {
					if err := r.activateRelay(ctx); err != nil {
						o.log.Warn().Err(err).Msg("Failed to activate relayOutAInA")
					}
				}
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
					Value: int32(o.targetState),
				},
				Actual: &model.OutputState{
					Value: int32(o.targetState),
				},
			}
			statuses.PublishOutputActual(msg)
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
func (o *trackInverter) ProcessMessage(ctx context.Context, r model.Output) error {
	value := r.GetRequest().GetValue()
	o.targetState = model.TrackInverterState(value)
	atomic.StoreInt32(&o.sendActualNeeded, 1)
	return nil
}

// ProcessPowerMessage acts upons a given power message.
func (o *trackInverter) ProcessPowerMessage(ctx context.Context, m model.PowerState) error {
	if m.GetEnabled() {
		atomic.StoreInt32(&o.sendActualNeeded, 1)
	}
	return nil
}
