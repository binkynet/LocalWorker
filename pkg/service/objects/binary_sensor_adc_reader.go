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

package objects

import (
	"context"
	"time"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/pkg/service/devices"
	"github.com/rs/zerolog"
)

// binarySensorADCReader is a binary sensor reader that reads a value
// from an ADC device.
type binarySensorADCReader struct {
	log             zerolog.Logger
	inputDevice     devices.ADC
	pin             model.DeviceIndex
	threshold       int
	debug           bool
	lastAnalogValue int
}

// newAnalogSensor creates a new analog-sensor object for the given configuration.
func newBinarySensorADCReader(log zerolog.Logger, adc devices.ADC, pin model.DeviceIndex, threshold int, debug bool) (binarySensorReader, error) {
	return &binarySensorADCReader{
		log:         log,
		inputDevice: adc,
		pin:         pin,
		threshold:   threshold,
		debug:       debug,
	}, nil
}

// Configure is called once to put the object in the desired state.
func (o *binarySensorADCReader) Configure(ctx context.Context) error {
	return nil
}

// Read the current sensor value
func (o *binarySensorADCReader) Read(ctx context.Context) (bool, error) {
	// Read state
	lctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	analogValue, err := o.inputDevice.Get(lctx, o.pin)
	if err != nil {
		return false, err
	}
	// Convert analog value to binary value
	value := analogValue >= o.threshold
	// Debug log changes (if any)
	if o.debug && analogValue != o.lastAnalogValue {
		o.log.Debug().
			Int("current_analog", analogValue).
			Int("last_analog", o.lastAnalogValue).
			Msg("Analog value changed")
		o.lastAnalogValue = analogValue
	}
	return value, nil
}
