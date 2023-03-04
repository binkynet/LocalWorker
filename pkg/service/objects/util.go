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

package objects

import (
	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/pkg/service/devices"
)

// getSinglePin looks up the pin with given name in the given configurable.
// If not found, an error is returned.
// If multiple pins are found, an error is returned.
func getSinglePin(oid model.ObjectID, config model.Object, connectionName model.ConnectionName) (model.Connection, model.DevicePin, error) {
	conn, ok := config.ConnectionByName(connectionName)
	if !ok {
		return model.Connection{}, model.DevicePin{}, model.InvalidArgument("Connection '%s' not found in object '%s'", connectionName, oid)
	}
	if len(conn.Pins) != 1 {
		return model.Connection{}, model.DevicePin{}, model.InvalidArgument("Connection '%s' must have 1 pin in object '%s', got %d", connectionName, oid, len(conn.Pins))
	}
	return *conn, *conn.Pins[0], nil
}

// getGPIOForPin looks up the device for the given pin.
// If device not found, an error is returned.
// If device is not a GPIO, an error is returned.
// If pin is not in pin-range of device, an error is returned.
func getGPIOForPin(pin model.DevicePin, devService devices.Service) (devices.GPIO, error) {
	device, ok := devService.DeviceByID(pin.DeviceId)
	if !ok {
		return nil, model.InvalidArgument("Device '%s' not found", pin.DeviceId)
	}
	gpio, ok := device.(devices.GPIO)
	if !ok {
		return nil, model.InvalidArgument("Device '%s' is not a GPIO", pin.DeviceId)
	}
	pinNr := pin.Index
	if pinNr < 1 || uint(pinNr) > gpio.PinCount() {
		return nil, model.InvalidArgument("Pin %d is out of range for device '%s'", pinNr, pin.DeviceId)
	}
	return gpio, nil
}

// getPWMForPin looks up the device for the given pin.
// If device not found, an error is returned.
// If device is not a PWM, an error is returned.
// If pin is not in pin-range of device, an error is returned.
func getPWMForPin(pin model.DevicePin, devService devices.Service) (devices.PWM, error) {
	device, ok := devService.DeviceByID(pin.DeviceId)
	if !ok {
		return nil, model.InvalidArgument("Device '%s' not found", pin.DeviceId)
	}
	pwm, ok := device.(devices.PWM)
	if !ok {
		return nil, model.InvalidArgument("Device '%s' is not a PWM", pin.DeviceId)
	}
	pinNr := pin.Index
	if pinNr < 1 || int(pinNr) > pwm.OutputCount() {
		return nil, model.InvalidArgument("Pin %d is out of range for device '%s'", pinNr, pin.DeviceId)
	}
	return pwm, nil
}

// absInt returns the absolute value of the given int.
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// maxInt returns the maximum of the given integers.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// minInt returns the minimum of the given integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func boolToInt32(v bool) int32 {
	if v {
		return 1
	}
	return 0
}

func int32ToBool(v int32) bool {
	return v != 0
}
