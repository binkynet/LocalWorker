// Copyright 2022 Ewout Prangsma
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

// ADC contains the API that is supported by all analog/digital conversion devices.
type ADC interface {
	Device
	// PinCount returns the number of pins of the device
	PinCount() uint
	// Set the analog value at given index (1...)
	Get(ctx context.Context, index model.DeviceIndex) (int, error)
}
