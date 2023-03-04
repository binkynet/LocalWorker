//    Copyright 2023 Ewout Prangsma
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

package bridge

import (
	"github.com/binkynet/LocalWorker/pkg/metrics"
)

const (
	subSystem = "bridge"
)

var (
	// Total number of times I2CBus.Execute is called
	i2cExecuteCounters = metrics.MustRegisterCounterVec(subSystem,
		"execute_total",
		"Total number of times I2CBus.Execute is called",
		"address")
	// Total number of times I2CBus.Execute failed
	i2cExecuteErrorCounters = metrics.MustRegisterCounterVec(subSystem,
		"execute_error_total",
		"Total number of times I2CBus.Execute failed",
		"address")
)
