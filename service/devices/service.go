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

package devices

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	aerr "github.com/ewoutp/go-aggregate-error"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/binkynet/BinkyNet/apis/util"
	model "github.com/binkynet/BinkyNet/apis/v1"

	"github.com/binkynet/LocalWorker/service/bridge"
	utils "github.com/binkynet/LocalWorker/service/util"
)

// Service contains the API that is exposed by the device service.
type Service interface {
	// DeviceByID returns the device with given ID.
	// Return false if not found
	DeviceByID(id model.DeviceID) (Device, bool)
	// Configure is called once to put all devices in the desired state.
	Configure(ctx context.Context) error
	// Run the service until the given context is canceled.
	Run(ctx context.Context, nwControlClient model.NetworkControlServiceClient) error
	// Close brings all devices back to a safe state.
	Close() error
}

type service struct {
	hardwareID        string
	moduleID          string
	programVersion    string
	log               zerolog.Logger
	devices           map[model.DeviceID]Device
	configuredDevices map[model.DeviceID]Device
	bus               bridge.I2CBus
	bAPI              bridge.API
	activeCount       uint32
}

// NewService instantiates a new Service and Device's for the given
// device configurations.
func NewService(hardwareID, moduleID, programVersion string, configs []*model.Device, bAPI bridge.API, bus bridge.I2CBus, log zerolog.Logger) (Service, error) {
	s := &service{
		hardwareID:        hardwareID,
		moduleID:          moduleID,
		programVersion:    programVersion,
		log:               log.With().Str("component", "device-service").Logger(),
		devices:           make(map[model.DeviceID]Device),
		configuredDevices: make(map[model.DeviceID]Device),
		bus:               bus,
		bAPI:              bAPI,
	}
	for _, c := range configs {
		var dev Device
		var err error
		switch c.Type {
		case model.DeviceTypeMCP23008:
			dev, err = newMcp23008(*c, bus, s.onActive)
		case model.DeviceTypeMCP23017:
			dev, err = newMcp23017(*c, bus, s.onActive)
		case model.DeviceTypePCA9685:
			dev, err = newPCA9685(*c, bus, s.onActive)
		case model.DeviceTypePCF8574:
			dev, err = newPCF8574(*c, bus, s.onActive)
		default:
			return nil, model.InvalidArgument("Unsupported device type '%s'", c.Type)
		}
		if err != nil {
			return nil, err
		}
		s.devices[c.Id] = dev
	}
	return s, nil
}

// DeviceByID returns the device with given ID.
// Return false if not found or not configured.
func (s *service) DeviceByID(id model.DeviceID) (Device, bool) {
	dev, ok := s.configuredDevices[id]
	return dev, ok
}

// Configure is called once to put all devices in the desired state.
func (s *service) Configure(ctx context.Context) error {
	log := s.log
	var ae aerr.AggregateError
	configuredDevices := make(map[model.DeviceID]Device)
	for id, d := range s.devices {
		log := log.With().Str("device-id", string(id)).Logger()
		log.Debug().Msg("configuring device...")
		if err := d.Configure(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to configure device")
			ae.Add(err)
			log.Debug().Err(err).Msg("Failed to configure device (debug)")
		} else {
			configuredDevices[id] = d
			log.Debug().Msg("configured device")
		}
	}
	s.configuredDevices = configuredDevices
	log.Info().Int("count", len(configuredDevices)).Msg("Configured devices")
	return ae.AsError()
}

// Run the service until the given context is canceled.
func (s *service) Run(ctx context.Context, nwControlClient model.NetworkControlServiceClient) error {
	g, ctx := errgroup.WithContext(ctx)
	//g.Go(func() error { return s.receiveDiscoverMessages(ctx, nwControlClient) })
	g.Go(func() error { return s.runActiveNotify(ctx) })
	return g.Wait()
}

// Close brings all devices back to a safe state.
func (s *service) Close() error {
	var ae aerr.AggregateError
	for _, d := range s.devices {
		if err := d.Close(); err != nil {
			ae.Add(err)
		}
	}
	return ae.AsError()
}

// onActive is called when a device change is activated.
func (s *service) onActive() {
	atomic.AddUint32(&s.activeCount, 1)
}

// runActiveNotify updates the blinking status when a device has become active
func (s *service) runActiveNotify(ctx context.Context) error {
	lastActiveCount := uint32(0)
	count := 0
	for {
		select {
		case <-ctx.Done():
			// Context canceled
			return nil
		case <-time.After(time.Second / 10):
			newActiveCount := atomic.LoadUint32(&s.activeCount)
			if newActiveCount != lastActiveCount {
				lastActiveCount = newActiveCount
				s.bAPI.BlinkRedLED(time.Second / 10)
				count = 0
			} else if count < 20 {
				count++
			} else {
				count = 0
				s.bAPI.SetRedLED(false)
			}
		}
	}
}

// Run subscribed to discover messages and processed them
// until the given context is cancelled.
func (s *service) receiveDiscoverMessages(ctx context.Context, lwControlClient model.LocalWorkerControlServiceClient) error {
	log := s.log
	once := func() error {
		log.Debug().Msg("Opening GetDiscoverRequests stream...")
		stream, err := lwControlClient.GetDiscoverRequests(ctx, &model.LocalWorkerInfo{
			Id: s.hardwareID,
		})
		if err != nil {
			log.Error().Err(err).Msg("Failed to request discover messages")
			return err
		}
		defer stream.CloseSend()
		for {
			log.Debug().Msg("Waiting to receive discover request...")
			_, err := stream.Recv()
			if util.IsStreamClosed(err) || ctx.Err() != nil {
				log.Info().Msg("Stream closed or context canceled")
				return nil
			} else if err != nil {
				log.Warn().Err(err).Msg("Recv failed")
				return err
			}
			// Process discover request
			log.Debug().Msg("Received discover request")

			addrs := s.bus.DetectSlaveAddresses()
			result := &model.DiscoverResult{
				Id: s.moduleID,
			}
			for _, addr := range addrs {
				result.Addresses = append(result.Addresses, fmt.Sprintf("0x%x", addr))
			}
			log.Info().Strs("addresses", result.GetAddresses()).Msg("Discovered addresses")
			if _, err := lwControlClient.SetDiscoverResult(ctx, result); err != nil {
				log.Warn().Err(err).Msg("SetDiscoverResult failed")
				return err
			}
		}
	}
	return utils.UntilCanceled(ctx, log, "receiveDiscoverMessages", once)
}
