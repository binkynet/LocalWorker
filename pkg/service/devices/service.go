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
	"sort"
	"strings"
	"sync/atomic"
	"time"

	aerr "github.com/ewoutp/go-aggregate-error"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	api "github.com/binkynet/BinkyNet/apis/v1"
	model "github.com/binkynet/BinkyNet/apis/v1"

	"github.com/binkynet/LocalWorker/pkg/service/bridge"
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
	Close(context.Context) error
	// Get a list of configured device IDs
	GetConfiguredDeviceIDs() []string
	// Get a list of unconfigured device IDs
	GetUnconfiguredDeviceIDs() []string
	// Get router info of all online routers
	GetOnlineRouterInfos() []*api.RouterInfo
	// Get router info of all offline routers
	GetOfflineRouterInfos() []*api.RouterInfo
	// Perform a single device discovery
	PerformDeviceDiscovery(ctx context.Context, req *api.DeviceDiscovery) error
	// Gets current status or a router or local worker.
	// If routerModuleID is empty, status of local worker is returned.
	// Otherwise status of specified router is returned.
	GetStatus(routerModuleID string) Status
}

type Status uint8

const (
	StatusUnknown Status = iota
	StatusOnline
	StatusOffline
)

const (
	statusDevID = api.DeviceID("statmon")
)

type service struct {
	hardwareID        string
	moduleID          string
	routers           []*api.RouterInfo
	programVersion    string
	mqttBrokerAddress string
	log               zerolog.Logger
	devices           map[model.DeviceID]Device
	configuredDevices map[model.DeviceID]Device
	bus               bridge.I2CBus
	bAPI              bridge.API
	activeCount       uint32
	nwControlClient   model.NetworkControlServiceClient
	lastStatuses      map[model.DeviceID]Status
	lastUptimeSeconds map[model.DeviceID]int
	lastIpAddress     map[model.DeviceID]string
}

// NewService instantiates a new Service and Device's for the given
// device configurations.
func NewService(hardwareID, moduleID, programVersion, mqttBrokerAddress string,
	configs []*model.Device, isVirtual bool, routers []*api.RouterInfo,
	bAPI bridge.API, bus bridge.I2CBus, log zerolog.Logger) (Service, error) {
	s := &service{
		hardwareID:        hardwareID,
		moduleID:          moduleID,
		routers:           routers,
		programVersion:    programVersion,
		mqttBrokerAddress: mqttBrokerAddress,
		log:               log.With().Str("component", "device-service").Logger(),
		devices:           make(map[model.DeviceID]Device),
		configuredDevices: make(map[model.DeviceID]Device),
		lastStatuses:      make(map[model.DeviceID]Status),
		lastUptimeSeconds: make(map[model.DeviceID]int),
		lastIpAddress:     map[model.DeviceID]string{},
		bus:               bus,
		bAPI:              bAPI,
	}
	if isVirtual {
		if len(routers) == 0 {
			log := s.log.With().Str("module_id", moduleID).Logger()
			s.lastStatuses[statusDevID] = StatusUnknown
			monitorDev, err := newMQTTStatusMonitor(log, statusDevID, s.onActive,
				func(newStatus Status) {
					log.Debug().Uint8("status", uint8(newStatus)).Msg("Got local worker status change")
					s.lastStatuses[statusDevID] = newStatus
				},
				func(seconds int) {
					log.Debug().Int("uptime", seconds).Msg("Got local worker uptime change")
					s.lastUptimeSeconds[statusDevID] = seconds
				},
				func(ipAddress string) {
					log.Debug().Str("ip_address", ipAddress).Msg("Got local worker ip_address change")
					s.lastIpAddress[statusDevID] = ipAddress
				}, moduleID, defaultMQTTTopicPrefix(moduleID), s.mqttBrokerAddress)
			if err != nil {
				return nil, err
			}
			s.devices[statusDevID] = monitorDev
		} else {
			for _, router := range routers {
				routerModuleID := router.GetModuleId()
				log := s.log.With().Str("router_module_id", routerModuleID).Logger()
				devID := routerStatusDeviceID(routerModuleID)
				s.lastStatuses[devID] = StatusUnknown
				monitorDev, err := newMQTTStatusMonitor(log, devID, s.onActive,
					func(newStatus Status) {
						log.Debug().Uint8("status", uint8(newStatus)).Msg("Got router status change")
						s.lastStatuses[devID] = newStatus
					},
					func(seconds int) {
						log.Debug().Int("uptime", seconds).Msg("Got router uptime change")
						s.lastUptimeSeconds[devID] = seconds
					},
					func(ipAddress string) {
						log.Debug().Str("ip_address", ipAddress).Msg("Got router ip_address change")
						s.lastIpAddress[devID] = ipAddress
					}, moduleID, defaultMQTTTopicPrefix(routerModuleID), s.mqttBrokerAddress)
				if err != nil {
					return nil, err
				}
				s.devices[devID] = monitorDev
			}
		}
	} else {
		s.lastStatuses[statusDevID] = StatusOnline
	}
	for _, c := range configs {
		var dev Device
		var err error
		switch c.Type {
		case model.DeviceTypeGPIO:
			if isVirtual {
				err = fmt.Errorf("no virtual implementation for gpio")
			} else {
				dev, err = newLocalGPIO(*c, bAPI, s.onActive)
			}
		case model.DeviceTypeBinkyCarSensor:
			if isVirtual {
				err = fmt.Errorf("no virtual implementation for binky-car-sensor")
			} else {
				dev, err = newBinkyCarSensor(log, *c, bus, s.onActive)
			}
		case model.DeviceTypeADS1115:
			if isVirtual {
				err = fmt.Errorf("no virtual implementation for ADS1115")
			} else {
				dev, err = newADS1115(*c, bus, s.onActive)
			}
		case model.DeviceTypeMCP23008:
			if isVirtual {
				dev, err = newMQTTGPIO(log, c.GetId(), s.onActive, moduleID, defaultMQTTTopicPrefix(moduleID), s.mqttBrokerAddress)
			} else {
				dev, err = newMcp23008(*c, bus, s.onActive)
			}
		case model.DeviceTypeMCP23017:
			if isVirtual {
				dev, err = newMQTTGPIO(log, c.GetId(), s.onActive, moduleID, defaultMQTTTopicPrefix(moduleID), s.mqttBrokerAddress)
			} else {
				dev, err = newMcp23017(*c, bus, s.onActive)
			}
		case model.DeviceTypePCA9685:
			if isVirtual {
				dev, err = newMQTTServo(log, c.GetId(), s.onActive, moduleID, defaultMQTTTopicPrefix(moduleID), s.mqttBrokerAddress)
			} else {
				dev, err = newPCA9685(*c, bus, s.onActive)
			}
		case model.DeviceTypePCF8574:
			if isVirtual {
				dev, err = newMQTTGPIO(log, c.GetId(), s.onActive, moduleID, defaultMQTTTopicPrefix(moduleID), s.mqttBrokerAddress)
			} else {
				dev, err = newPCF8574(*c, bus, s.onActive)
			}
		case model.DeviceTypeMQTTGPIO:
			dev, err = newMQTTGPIO(log, c.GetId(), s.onActive, moduleID, defaultMQTTTopicPrefix(moduleID), s.mqttBrokerAddress)
		case model.DeviceTypeMQTTServo:
			dev, err = newMQTTServo(log, c.GetId(), s.onActive, moduleID, defaultMQTTTopicPrefix(moduleID), s.mqttBrokerAddress)
		default:
			return nil, model.InvalidArgument("Unsupported device type '%s'", c.Type)
		}
		if err != nil {
			return nil, err
		}
		s.devices[c.Id] = dev
	}
	devicesCreatedTotal.Set(float64(len(s.devices)))
	return s, nil
}

// Create the status device ID key for the router with given module id.
func routerStatusDeviceID(routerModuleID string) model.DeviceID {
	return model.DeviceID(fmt.Sprintf("%s-%s", statusDevID, routerModuleID))
}

// Generate the default MQTT topic prefix for the given device config.
func defaultMQTTTopicPrefix(moduleID string) string {
	return strings.ToLower(fmt.Sprintf("/binky/%s/", moduleID))
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
	devicesConfiguredTotal.Set(float64(len(configuredDevices)))
	return ae.AsError()
}

// Run the service until the given context is canceled.
func (s *service) Run(ctx context.Context, nwControlClient model.NetworkControlServiceClient) error {
	s.nwControlClient = nwControlClient
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return s.runActiveNotify(ctx) })
	return g.Wait()
}

// Close brings all devices back to a safe state.
func (s *service) Close(ctx context.Context) error {
	var ae aerr.AggregateError
	for _, d := range s.devices {
		if err := d.Close(ctx); err != nil {
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

// Perform a single device discovery
func (s *service) PerformDeviceDiscovery(ctx context.Context, req *api.DeviceDiscovery) error {
	log := s.log
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
	req.Actual = result
	if client := s.nwControlClient; client != nil {
		if _, err := client.SetDeviceDiscoveryActual(ctx, req); err != nil {
			log.Warn().Err(err).Msg("SetDeviceDiscoveryActual failed")
			return err
		}
	}
	return nil
}

// Get a list of configured device IDs
func (s *service) GetConfiguredDeviceIDs() []string {
	confDevs := s.configuredDevices
	result := make([]string, 0, len(confDevs))
	for k := range confDevs {
		result = append(result, string(k))
	}
	sort.Strings(result)
	return result
}

// Get a list of unconfigured device IDs
func (s *service) GetUnconfiguredDeviceIDs() []string {
	allDevs := s.devices
	result := make([]string, 0, len(allDevs))
	for id := range allDevs {
		if _, found := s.configuredDevices[id]; !found {
			result = append(result, string(id))
		}
	}
	sort.Strings(result)
	return result
}

// Gets current status or a router or local worker.
// If routerModuleID is empty, status of local worker is returned.
// Otherwise status of specified router is returned.
func (s *service) GetStatus(routerModuleID string) Status {
	if routerModuleID == "" {
		if status, ok := s.lastStatuses[statusDevID]; ok {
			return status
		}
		if len(s.lastStatuses) == 0 {
			return StatusUnknown
		}
		all := lo.Values(s.lastStatuses)
		return lo.Reduce(all, func(a, b Status, _ int) Status {
			if a == StatusOnline || b == StatusOnline {
				return StatusOnline
			}
			if a == StatusOffline || b == StatusOffline {
				return StatusOffline
			}
			return StatusUnknown
		}, all[0])
	}
	devID := routerStatusDeviceID(routerModuleID)
	return s.lastStatuses[devID]
}

// Get router info of all online routers
func (s *service) GetOnlineRouterInfos() []*api.RouterInfo {
	result := make([]*api.RouterInfo, 0, len(s.routers))
	for _, r := range s.routers {
		status := s.GetStatus(r.GetModuleId())
		if status == StatusOnline {
			devID := routerStatusDeviceID(r.GetModuleId())
			r.Uptime = int64(s.lastUptimeSeconds[devID])
			r.IpAddress = s.lastIpAddress[devID]
			result = append(result, r)
		}
	}
	return result
}

// Get router info of all offline routers
func (s *service) GetOfflineRouterInfos() []*api.RouterInfo {
	result := make([]*api.RouterInfo, 0, len(s.routers))
	for _, r := range s.routers {
		status := s.GetStatus(r.GetModuleId())
		if status != StatusOnline {
			r.Uptime = 0
			r.IpAddress = ""
			result = append(result, r)
		}
	}
	return result
}
