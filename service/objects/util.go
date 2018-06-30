package objects

import (
	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/LocalWorker/service/devices"
	"github.com/pkg/errors"
)

// getSinglePin looks up the pin with given name in the given configurable.
// If not found, an error is returned.
// If multiple pins are found, an error is returned.
func getSinglePin(oid model.ObjectID, config model.Object, connectionName model.ConnectionName) (model.DevicePin, error) {
	pins, ok := config.Connections[connectionName]
	if !ok {
		return model.DevicePin{}, errors.Wrapf(model.ValidationError, "Connection '%s' not found in object '%s'", connectionName, oid)
	}
	if len(pins) != 1 {
		return model.DevicePin{}, errors.Wrapf(model.ValidationError, "Connection '%s' must have 1 pin in object '%s', got %d", connectionName, oid, len(pins))
	}
	return pins[0], nil
}

// getGPIOForPin looks up the device for the given pin.
// If device not found, an error is returned.
// If device is not a GPIO, an error is returned.
// If pin is not in pin-range of device, an error is returned.
func getGPIOForPin(pin model.DevicePin, devService devices.Service) (devices.GPIO, error) {
	device, ok := devService.DeviceByID(pin.DeviceID)
	if !ok {
		return nil, errors.Wrapf(model.ValidationError, "Device '%s' not found", pin.DeviceID)
	}
	gpio, ok := device.(devices.GPIO)
	if !ok {
		return nil, errors.Wrapf(model.ValidationError, "Device '%s' is not a GPIO", pin.DeviceID)
	}
	pinNr := pin.Index
	if pinNr < 1 || uint(pinNr) > gpio.PinCount() {
		return nil, errors.Wrapf(model.ValidationError, "Pin %d is out of range for device '%s'", pinNr, pin.DeviceID)
	}
	return gpio, nil
}
