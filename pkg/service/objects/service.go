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
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	aerr "github.com/ewoutp/go-aggregate-error"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/binkynet/BinkyNet/apis/util"
	api "github.com/binkynet/BinkyNet/apis/v1"
	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/pkg/service/devices"
	"github.com/binkynet/LocalWorker/pkg/service/intf"
	utils "github.com/binkynet/LocalWorker/pkg/service/util"
)

// Service contains the API that is exposed by the object service.
type Service interface {
	// ObjectByAddress returns the object with given address.
	// Returns: Object, IsGlobal, error
	ObjectByAddress(address model.ObjectAddress) (Object, bool, bool)
	// Configure is called once to put all objects in the desired state.
	Configure(ctx context.Context) error
	// Run all required topics until the given context is cancelled.
	Run(ctx context.Context, nwControlClient model.NetworkControlServiceClient) error
	intf.RequestService
}

type service struct {
	startTime         time.Time
	moduleID          string
	devService        devices.Service
	objects           map[model.ObjectAddress]Object
	configuredObjects map[model.ObjectAddress]Object
	programVersion    string
	metricsPort       int
	grpcPort          int
	log               zerolog.Logger
	requestService    requestService
}

const (
	// How often do we want to log sending ping messages
	pingLogInterval = time.Minute
)

// NewService instantiates a new Service and Object's for the given
// object configurations.
func NewService(moduleID string, programVersion string, metricsPort, grpcPort int, configs []*model.Object, devService devices.Service, log zerolog.Logger) (Service, error) {
	s := &service{
		startTime:         time.Now(),
		moduleID:          moduleID,
		devService:        devService,
		objects:           make(map[model.ObjectAddress]Object),
		configuredObjects: make(map[model.ObjectAddress]Object),
		programVersion:    programVersion,
		metricsPort:       metricsPort,
		grpcPort:          grpcPort,
		log:               log.With().Str("component", "object-service").Logger(),
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
		case model.ObjectTypeMagneticSwitch:
			obj, err = newMagneticSwitch(moduleID, id, address, *c, log, devService)
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
	objectsCreatedTotal.Set(float64(len(s.objects)))
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
	objectsConfiguredTotal.Set(float64(len(configuredObjects)))
	return ae.AsError()
}

// Run all required topics until the given context is cancelled.
func (s *service) Run(ctx context.Context, nwControlClient model.NetworkControlServiceClient) error {
	defer func() {
		s.log.Debug().Msg("Run Objects ended")
	}()

	// Do nothing if we do not have configured objects
	if len(s.configuredObjects) == 0 {
		s.log.Warn().Msg("no configured objects, just waiting for context to be cancelled")
		<-ctx.Done()
		return nil
	}

	// Create request/status services
	requests := newRequestService(s.log)
	s.requestService = requests
	statuses := newStatusService(s.log)

	g, ctx := errgroup.WithContext(ctx)
	// Run requests
	g.Go(func() error { return requests.Run(ctx, s.moduleID, nwControlClient) })
	// Run statuses
	g.Go(func() error { return statuses.Run(ctx, nwControlClient) })
	// Keep sending ping messages
	g.Go(func() error { s.sendPingMessages(ctx, nwControlClient); return nil })
	// Receive power messages
	g.Go(func() error { s.receivePowerMessages(ctx, nwControlClient); return nil })

	// Run all objects & object types.
	visitedTypes := make(map[ObjectType]struct{})
	var runningObjects, runningObjectTypes int32
	for addr, obj := range s.configuredObjects {
		// Run the object itself
		addr := addr // Bring range variables in scope
		obj := obj
		g.Go(func() error {
			atomic.AddInt32(&runningObjects, 1)
			log := s.log.With().
				Str("address", string(addr)).
				Str("objType", obj.Type().String()).
				Logger()
			defer func() {
				atomic.AddInt32(&runningObjects, -1)
				log.Debug().Msg("Stopped running object")
			}()
			log.Debug().Msg("Running object")
			if err := obj.Run(ctx, requests, statuses, s.moduleID); err != nil {
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
			g.Go(func() error {
				atomic.AddInt32(&runningObjectTypes, 1)
				log := s.log.With().Str("objType", objType.String()).Logger()
				defer func() {
					atomic.AddInt32(&runningObjectTypes, -1)
					log.Debug().Msg("Stopped running object type")
				}()
				log.Debug().Msg("Running object type")
				if err := objType.Run(ctx, log, requests, statuses, s, s.moduleID); err != nil {
					return err
				}
				return nil
			})
		}
	}

	g.Go(func() error {
		<-ctx.Done()
		for {
			objs := atomic.LoadInt32(&runningObjects)
			objTypes := atomic.LoadInt32(&runningObjectTypes)
			if objs == 0 && objTypes == 0 {
				s.log.Debug().Msg("No more running objects & object types")
				return nil
			}
			s.log.Debug().
				Int32("running_objects", objs).
				Int32("running_object_types", objTypes).
				Msg("Still running objects")
			time.Sleep(time.Second * 2)
		}
	})

	if err := g.Wait(); err != nil && ctx.Err() == nil {
		s.log.Warn().Err(err).Msg("Run Objects failed")
		return err
	}

	return nil
}

// Set the requested power state
func (s *service) SetPowerRequest(ctx context.Context, msg *api.PowerState) error {
	log := s.log
	log.Debug().Bool("enabled", msg.GetEnabled()).Msg("Receiver power request")
	req := *msg
	for _, obj := range s.configuredObjects {
		// Run the object itself
		go func(obj Object, msg api.PowerState) {
			if err := obj.ProcessPowerMessage(ctx, msg); err != nil {
				log.Info().Err(err).Msg("Object failed to process PowerMessage")
			}
		}(obj, req)
	}
	return nil
}

// Set the requested output state
func (s *service) SetOutputRequest(ctx context.Context, msg *api.Output) error {
	if rs := s.requestService; rs != nil {
		return rs.SetOutputRequest(ctx, msg)
	}
	return fmt.Errorf("not ready yet")
}

// Set the requested switch state
func (s *service) SetSwitchRequest(ctx context.Context, msg *api.Switch) error {
	if rs := s.requestService; rs != nil {
		return rs.SetSwitchRequest(ctx, msg)
	}
	return fmt.Errorf("not ready yet")
}

// sendPingMessages keeps sending ping messages until the given context is canceled.
func (s *service) sendPingMessages(ctx context.Context, nwControlClient model.NetworkControlServiceClient) {
	log := s.log
	log.Info().Msg("Sending ping messages")
	defer func() {
		log.Info().Msg("Stopped sending ping messages")
	}()
	msg := model.LocalWorker{
		Id: s.moduleID,
		Actual: &model.LocalWorkerInfo{
			Id:                       s.moduleID,
			Description:              "Local worker",
			Version:                  s.programVersion,
			Uptime:                   int64(time.Since(s.startTime).Seconds()),
			MetricsPort:              int32(s.metricsPort),
			MetricsSecure:            false,
			LocalWorkerServicePort:   int32(s.grpcPort),
			LocalWorkerServiceSecure: false,
			SupportsReset:            true,
			SupportsSetLocRequest:    false,
			SupportsSetPowerRequest:  true,
			SupportsSetOutputRequest: true,
			SupportsSetSwitchRequest: true,
		},
	}
	lastPingLog := time.Now()
	for {
		// Send ping
		msg.Actual.Uptime = int64(time.Since(s.startTime).Seconds())
		msg.Actual.ConfiguredDeviceIds = s.devService.GetConfiguredDeviceIDs()
		msg.Actual.ConfiguredObjectIds = s.getConfiguredObjectIDs()
		msg.Actual.UnconfiguredDeviceIds = s.devService.GetUnconfiguredDeviceIDs()
		msg.Actual.UnconfiguredObjectIds = s.getUnconfiguredObjectIDs()
		delay := time.Second * 5
		if _, err := nwControlClient.SetLocalWorkerActual(ctx, &msg); err != nil && ctx.Err() == nil {
			log.Info().Err(err).Msg("Failed to SetLocalWorkerActual")
			delay = time.Second * 3
		} else {
			if time.Since(lastPingLog) > pingLogInterval {
				log.Debug().Msg("Ping sent")
				lastPingLog = time.Now()
			}
		}

		// Wait
		select {
		case <-time.After(delay):
			// Continue
		case <-ctx.Done():
			// Context canceled
			return
		}
	}
}

// getConfiguredObjectIDs builds a list of all IDs of configured objects
func (s *service) getConfiguredObjectIDs() []string {
	confObjs := s.configuredObjects
	result := make([]string, 0, len(confObjs))
	for k := range confObjs {
		result = append(result, string(k))
	}
	sort.Strings(result)
	return result
}

// getUnconfiguredObjectIDs builds a list of all IDs of unconfigured objects
func (s *service) getUnconfiguredObjectIDs() []string {
	allObjs := s.objects
	result := make([]string, 0, len(allObjs))
	for id := range allObjs {
		if _, found := s.configuredObjects[id]; !found {
			result = append(result, string(id))
		}
	}
	sort.Strings(result)
	return result
}

// Run subscribes to the intended topic and process incoming messages
// until the given context is cancelled.
func (s *service) receivePowerMessages(ctx context.Context, nwControlClient model.NetworkControlServiceClient) error {
	log := s.log
	once := func() error {
		stream, err := nwControlClient.WatchPower(ctx, &model.WatchOptions{
			WatchRequestChanges: true,
		})
		if err != nil {
			log.Error().Err(err).Msg("Failed to request power messages")
			return err
		}
		defer stream.CloseSend()
		for {
			msg, err := stream.Recv()
			if util.IsStreamClosed(err) || ctx.Err() != nil {
				return nil
			} else if err != nil {
				log.Warn().Err(err).Msg("Recv failed")
				return err
			}
			// Process power request
			log.Debug().Bool("enabled", msg.GetRequest().GetEnabled()).Msg("Receiver power request")
			req := *msg.GetRequest()
			for _, obj := range s.configuredObjects {
				// Run the object itself
				go func(obj Object, msg api.PowerState) {
					if err := obj.ProcessPowerMessage(ctx, msg); err != nil {
						log.Info().Err(err).Msg("Object failed to process PowerMessage")
					}
				}(obj, req)
			}
		}
	}
	return utils.UntilCanceled(ctx, log, "receivePowerMessages", once)
}
