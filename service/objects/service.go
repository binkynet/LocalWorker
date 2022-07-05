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
	"sync"
	"time"

	aerr "github.com/ewoutp/go-aggregate-error"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/service/devices"

	utils "github.com/binkynet/LocalWorker/service/util"
)

// Service contains the API that is exposed by the object service.
type Service interface {
	// ObjectByAddress returns the object with given address.
	// Returns: Object, IsGlobal, error
	ObjectByAddress(address model.ObjectAddress) (Object, bool, bool)
	// Configure is called once to put all objects in the desired state.
	Configure(ctx context.Context) error
	// Run all required topics until the given context is cancelled.
	Run(ctx context.Context) error
	// Set the given power state
	SetPower(ctx context.Context, req *model.PowerState) error
	// Set the given output state
	SetOutput(ctx context.Context, req *model.Output) error
	// Set the given switch state
	SetSwitch(ctx context.Context, req *model.Switch) error
}

type service struct {
	startTime         time.Time
	moduleID          string
	objects           map[model.ObjectAddress]Object
	configuredObjects map[model.ObjectAddress]Object
	log               zerolog.Logger
	requestService    *requestService
	actuals           *ServiceActuals
}

// NewService instantiates a new Service and Object's for the given
// object configurations.
func NewService(moduleID string, configs []*model.Object, devService devices.Service, actuals *ServiceActuals, log zerolog.Logger) (Service, error) {
	s := &service{
		startTime:         time.Now(),
		moduleID:          moduleID,
		objects:           make(map[model.ObjectAddress]Object),
		configuredObjects: make(map[model.ObjectAddress]Object),
		log:               log.With().Str("component", "object-service").Logger(),
		requestService:    newRequestService(log),
		actuals:           actuals,
	}
	for _, c := range configs {
		var obj Object
		var err error
		id := c.Id
		address := model.JoinModuleLocal(moduleID, string(id))
		log := log.With().
			Str("address", string(address)).
			Str("type", string(c.Type)).
			Logger()
		log.Debug().Msg("creating object...")
		switch c.Type {
		case model.ObjectTypeBinarySensor:
			obj, err = newBinarySensor(moduleID, id, address, *c, log, devService)
		case model.ObjectTypeBinaryOutput:
			obj, err = newBinaryOutput(moduleID, id, address, *c, log, devService)
		case model.ObjectTypeRelaySwitch:
			obj, err = newRelaySwitch(moduleID, id, address, *c, log, devService)
		case model.ObjectTypeServoSwitch:
			obj, err = newServoSwitch(moduleID, id, address, *c, log, devService)
		case model.ObjectTypeTrackInverter:
			obj, err = newTrackInverter(moduleID, id, address, *c, log, devService)
		default:
			err = model.InvalidArgument("Unsupported object type '%s'", c.Type)
		}
		if err != nil {
			log.Error().Err(err).Msg("Failed to create object")
			//return nil, maskAny(err)
		} else {
			s.objects[address] = obj
		}
	}
	log.Debug().Msgf("created %d objects", len(s.objects))
	return s, nil
}

// ObjectByAddress returns the object with given object address.
// Returns: Object, IsGlobal, error
func (s *service) ObjectByAddress(address model.ObjectAddress) (Object, bool, bool) {
	// Split address
	module, id, _ := model.SplitAddress(address)
	isGlobal := module == model.GlobalModuleID

	// Try module local addresses
	if dev, ok := s.configuredObjects[address]; ok {
		return dev, isGlobal, true
	}
	// Try global addresses
	if isGlobal {
		localAddr := model.JoinModuleLocal(s.moduleID, id)
		if dev, ok := s.configuredObjects[localAddr]; ok {
			return dev, true, true
		}
	}
	return nil, isGlobal, false
}

// Configure is called once to put all objects in the desired state.
func (s *service) Configure(ctx context.Context) error {
	var ae aerr.AggregateError
	configuredObjects := make(map[model.ObjectAddress]Object)
	log := s.log
	for addr, obj := range s.objects {
		log := log.With().Str("address", string(addr)).Logger()
		log.Debug().Msg("configuring object ...")
		time.Sleep(time.Millisecond * 200)
		if err := obj.Configure(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to configure object")
			ae.Add(err)
		} else {
			configuredObjects[addr] = obj
			log.Debug().Msg("configured object")
		}
	}
	s.configuredObjects = configuredObjects
	return ae.AsError()
}

// Run all required topics until the given context is cancelled.
func (s *service) Run(ctx context.Context) error {
	defer func() {
		s.log.Debug().Msg("Run Objects ended")
	}()
	if len(s.configuredObjects) == 0 {
		s.log.Warn().Msg("no configured objects, just waiting for context to be cancelled")
		<-ctx.Done()
	} else {
		g, ctx := errgroup.WithContext(ctx)

		// Run all objects & object types.
		visitedTypes := make(map[*ObjectType]struct{})
		for addr, obj := range s.configuredObjects {
			// Run the object itself
			addr := addr // Bring range variables in scope
			obj := obj
			g.Go(func() error {
				s.log.Debug().Str("address", string(addr)).Msg("Running object")
				if err := obj.Run(ctx, s.requestService, s.actuals, s.moduleID); err != nil {
					return err
				}
				return nil
			})

			// Run the message loop for the type of object (if not running already)
			if objType := obj.Type(); objType != nil {
				if _, found := visitedTypes[objType]; found {
					// Type already running
					continue
				}
				visitedTypes[objType] = struct{}{}
				if objType.Run != nil {
					g.Go(func() error {
						s.log.Debug().Msg("starting object type")
						if err := objType.Run(ctx, s.log, s.requestService, s.actuals, s, s.moduleID); err != nil {
							return err
						}
						return nil
					})
				}
			}
		}
		if err := g.Wait(); err != nil && ctx.Err() == nil {
			s.log.Warn().Err(err).Msg("Run Objects failed")
			return err
		}
	}
	return nil
}

// Set the given power state
func (s *service) SetPower(ctx context.Context, req *model.PowerState) error {
	log := s.log
	wg := sync.WaitGroup{}
	var result utils.SyncError
	for _, obj := range s.configuredObjects {
		wg.Add(1)
		// Run the object itself
		go func(obj Object) {
			defer wg.Done()
			if err := obj.ProcessPowerMessage(ctx, *req); err != nil {
				log.Info().Err(err).Msg("Object failed to process PowerMessage")
				result.Add(err)
			}
		}(obj)
	}
	wg.Wait()
	return result.AsError()
}

// Set the given output state
func (s *service) SetOutput(ctx context.Context, req *model.Output) error {
	return s.requestService.SetOutput(ctx, req)
}

// Set the given switch state
func (s *service) SetSwitch(ctx context.Context, req *model.Switch) error {
	return s.requestService.SetSwitch(ctx, req)
}
