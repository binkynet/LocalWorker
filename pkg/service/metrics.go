//    Copyright 2021 Ewout Prangsma
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package service

import (
	"github.com/binkynet/LocalWorker/pkg/metrics"
)

const (
	subSystem = "service"
)

var (
	// Total number of changes in loki service detected
	lokiServiceChangesTotal = metrics.MustRegisterCounter(subSystem,
		"loki_service_changes_total",
		"Total number of changes in loki service detected")
	// Total number of changes in network control service detected
	networkControlServiceChangesTotal = metrics.MustRegisterCounter(subSystem,
		"network_control_service_changes_total",
		"Total number of changes in network control service detected")
	// Total number of SetPowerRequest calls
	setPowerRequestTotal = metrics.MustRegisterCounter(subSystem,
		"service_api_set_power_request_total",
		"Total number of SetPowerRequest calls")
	// Total number of SetLocRequest calls
	setLocRequestTotal = metrics.MustRegisterCounter(subSystem,
		"service_api_set_loc_request_total",
		"Total number of SetLocRequest calls")
	// Total number of SetOutputRequest calls per output ID
	setOutputRequestTotal = metrics.MustRegisterCounterVec(subSystem,
		"service_api_set_output_request_total",
		"Total number of SetOutputRequest calls per ID",
		"id")
	// Total number of SetSwitchRequest calls per switch ID
	setSwitchRequestTotal = metrics.MustRegisterCounterVec(subSystem,
		"service_api_set_switch_request_total",
		"Total number of SetSwitchRequest calls per ID",
		"id")
	// Total number of SetDeviceDiscoveryRequest calls
	setDeviceDiscoveryRequestTotal = metrics.MustRegisterCounter(subSystem,
		"service_api_set_device_discovery_request_total",
		"Total number of SetSwitchRequest calls")
)
