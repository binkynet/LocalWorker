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
	// Total number of changed configurations received
	configurationChangesTotal = metrics.MustRegisterCounter(subSystem,
		"configuration_changes_total",
		"Total number of changed configurations received")
	// ID of current environment
	currentEnvironmentIDGauge = metrics.MustRegisterGauge(subSystem,
		"environment_id",
		"ID of current environment")
	// ID of current worker
	currentWorkerIDGauge = metrics.MustRegisterGauge(subSystem,
		"worker_id",
		"ID of current worker")
	// Total number of changes in loki service detected
	lokiServiceChangesTotal = metrics.MustRegisterCounter(subSystem,
		"loki_service_changes_total",
		"Total number of changes in loki service detected")
	// Total number of changes in network control service detected
	networkControlServiceChangesTotal = metrics.MustRegisterCounter(subSystem,
		"network_control_service_changes_total",
		"Total number of changes in network control service detected")
	// Total number of workers created
	workerCountTotal = metrics.MustRegisterCounter(subSystem,
		"worker_count_total",
		"Total number of workers created")
)
