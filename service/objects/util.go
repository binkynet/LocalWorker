package objects

import (
	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/LocalWorker/service/devices"
	"github.com/pkg/errors"
)

// getSinglePin looks up the pin with given name in the given configurable.
// If not found, an error is returned.
// If multiple pins are found, an error is returned.
func getSinglePin(config model.Object, pinName string) (model.Pin, error) {
	pins, ok := config.Pins[pinName]
	if !ok {
		return model.Pin{}, errors.Wrapf(model.ValidationError, "Pin '%s' not found in object '%s'", pinName, config.ID)
	}
	if len(pins) != 1 {
		return model.Pin{}, errors.Wrapf(model.ValidationError, "Pin '%s' must have 1 pin in object '%s', got %d", pinName, config.ID, len(pins))
	}
	return pins[0], nil
}

// getGPIOForPin looks up the device for the given pin.
// If device not found, an error is returned.
// If device is not a GPIO, an error is returned.
// If pin is not in pin-range of device, an error is returned.
func getGPIOForPin(pin model.Pin, devService devices.Service) (devices.GPIO, error) {
	device, ok := devService.DeviceByID(pin.DeviceID)
	if !ok {
		return nil, errors.Wrapf(model.ValidationError, "Device '%s' not found", pin.DeviceID)
	}
	gpio, ok := device.(devices.GPIO)
	if !ok {
		return nil, errors.Wrapf(model.ValidationError, "Device '%s' is not a GPIO", pin.DeviceID)
	}
	pinNr := pin.Pin
	if pinNr < 1 || pinNr > gpio.PinCount() {
		return nil, errors.Wrapf(model.ValidationError, "Pin %d is out of range for device '%s'", pinNr, pin.DeviceID)
	}
	return gpio, nil
}
