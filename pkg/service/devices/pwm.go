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

	model "github.com/binkynet/BinkyNet/apis/v1"
)

// PWM contains the API that is supported by all pulse width modulation devices.
type PWM interface {
	Device
	// PWMPinCount returns the number of PWM output pins of the device
	PWMPinCount() int
	// MaxPWMValue returns the maximum valid value for onValue or offValue.
	MaxPWMValue() uint32
	// SetPWM the output at given index (1...) to the given value
	SetPWM(ctx context.Context, output model.DeviceIndex, onValue, offValue uint32, enabled bool) error
	// GetPWM the output at given index (1...)
	// Returns onValue,offValue,enabled,error
	GetPWM(ctx context.Context, output model.DeviceIndex) (uint32, uint32, bool, error)
}
