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
		"i2c_execute_total",
		"Total number of times I2CBus.Execute is called",
		"address")
	// Total number of times I2CBus.Execute failed
	i2cExecuteErrorCounters = metrics.MustRegisterCounterVec(subSystem,
		"i2c_execute_error_total",
		"Total number of times I2CBus.Execute failed",
		"address")
	i2cRecoveryAttemptsTotal = metrics.MustRegisterCounter(subSystem,
		"i2c_recovery_attempts_total",
		"Total number of times and I2C bus recovery was attempted")
	i2cRecoverySucceededTotal = metrics.MustRegisterCounter(subSystem,
		"i2c_recovery_succeeded_total",
		"Total number of times and I2C bus recovery succeeded")
	i2cRecoveryFailedTotal = metrics.MustRegisterCounter(subSystem,
		"i2c_recovery_failed_total",
		"Total number of times and I2C bus recovery failed")
	i2cRecoverySkippedTotal = metrics.MustRegisterCounter(subSystem,
		"i2c_recovery_skipped_total",
		"Total number of times and I2C bus recovery skipped")
)
