//    Copyright 2024 Ewout Prangsma
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

package ncs

import (
	"github.com/binkynet/LocalWorker/pkg/metrics"
)

const (
	subSystem = "ncs"
)

var (
	// Total number of changed configurations received
	configurationChangesTotal = metrics.MustRegisterCounter(subSystem,
		"configuration_changes_total",
		"Total number of changed configurations received")
	// ID of current NCS
	currentNcsIDGauge = metrics.MustRegisterGauge(subSystem,
		"ncs_id",
		"ID of current network control service")
	// ID of current worker
	currentWorkerIDGauge = metrics.MustRegisterGauge(subSystem,
		"worker_id",
		"ID of current worker")
	// Total number of workers, ever created
	workerCountTotal = metrics.MustRegisterCounter(subSystem,
		"worker_count_total",
		"Total number of workers created")
)
