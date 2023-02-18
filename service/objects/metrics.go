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
	"github.com/binkynet/LocalWorker/pkg/metrics"
)

const (
	subSystem = "objects"
)

var (
	// Number of created objects
	objectsCreatedTotal = metrics.MustRegisterGauge(subSystem,
		"objects_created_total",
		"Number of created objects")

	// Number of configured objects
	objectsConfiguredTotal = metrics.MustRegisterGauge(subSystem,
		"objects_configured_total",
		"Number of configured objects")

	// Binary output metrics
	binaryOutputRequestsTotal = metrics.MustRegisterCounterVec(subSystem,
		"binary_output_requests_total",
		"Number of binary output requests",
		"id")
	binaryOutputRequestGauge = metrics.MustRegisterGaugeVec(subSystem,
		"binary_output_request",
		"Requested value of binary output (0=OFF, 1=ON)",
		"id")

	// Binary sensor metrics
	binarySensorActualGauge = metrics.MustRegisterGaugeVec(subSystem,
		"binary_sensor_actual",
		"Actual value of binary sensor",
		"id")
	// Binary sensor metrics
	binarySensorChangesTotal = metrics.MustRegisterCounterVec(subSystem,
		"binary_sensor_changes_total",
		"Number of times, actual value of binary sensor has changed",
		"id")

	// Switch metrics
	switchDirectionRequestsTotal = metrics.MustRegisterCounterVec(subSystem,
		"switch_direction_requests_total",
		"Number of switch direction requests",
		"id")
	switchDirectionRequestGauge = metrics.MustRegisterGaugeVec(subSystem,
		"switch_direction_request",
		"Requested direction of switch (0=STRAIGHT, 1=OFF)",
		"id")

	// Track inverter metrics
	trackInverterRequestsTotal = metrics.MustRegisterCounterVec(subSystem,
		"track_inverter_requests_total",
		"Number of track inverter requests",
		"id")
	trackInverterRequestGauge = metrics.MustRegisterGaugeVec(subSystem,
		"track_inverter_request",
		"Requested value of track inverter (0=OFF, 1=ON)",
		"id")
)
