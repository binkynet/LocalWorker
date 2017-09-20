package objects

import (
	"context"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/BinkyNet/mq"
	"github.com/binkynet/LocalWorker/service/devices"
	"github.com/binkynet/LocalWorker/service/mqtt"
	"github.com/pkg/errors"
)

var (
	binaryOutputType = &ObjectType{
		TopicSuffix: mq.BinaryOutputRequest{}.TopicSuffix(),
		NextMessage: func(ctx context.Context, subscription mqtt.Subscription, service Service) error {
			var msg mq.BinaryOutputRequest
			if err := subscription.NextMsg(ctx, &msg); err != nil {
				return maskAny(err)
			}
			return nil
		},
	}
)

type binaryOutput struct {
	config       model.Object
	outputDevice devices.GPIO
	pin          int
}

// newBinaryOutput creates a new binary-output object for the given configuration.
func newBinaryOutput(config model.Object, devService devices.Service) (Object, error) {
	if config.Type != model.ObjectTypeBinaryOutput {
		return nil, errors.Wrapf(model.ValidationError, "Invalid object type '%s'", config.Type)
	}
	pins, ok := config.Pins[model.PinNameOutput]
	if !ok {
		return nil, errors.Wrapf(model.ValidationError, "Pin '%s' not found in object '%s'", model.PinNameOutput, config.ID)
	}
	if len(pins) != 1 {
		return nil, errors.Wrapf(model.ValidationError, "Pin '%s' must have 1 pin in object '%s', got %d", model.PinNameOutput, config.ID, len(pins))
	}
	device, ok := devService.DeviceByID(pins[0].DeviceID)
	if !ok {
		return nil, errors.Wrapf(model.ValidationError, "Device '%s' not found in object '%s'", pins[0].DeviceID, config.ID)
	}
	gpio, ok := device.(devices.GPIO)
	if !ok {
		return nil, errors.Wrapf(model.ValidationError, "Device '%s' in object '%s' is not a GPIO", pins[0].DeviceID, config.ID)
	}
	pin := pins[0].Pin
	if pin < 1 || pin > gpio.PinCount() {
		return nil, errors.Wrapf(model.ValidationError, "Pin '%s' in object '%s' is out of range. Got %d. Range [1..%d]", model.PinNameOutput, config.ID, pin, gpio.PinCount())
	}
	return &binaryOutput{
		config:       config,
		outputDevice: gpio,
		pin:          pin,
	}, nil
}

// Return the type of this object.
func (o *binaryOutput) Type() *ObjectType {
	return binaryOutputType
}

// Configure is called once to put the object in the desired state.
func (o *binaryOutput) Configure(ctx context.Context) error {
	if err := o.outputDevice.SetDirection(ctx, o.pin, devices.PinDirectionOutput); err != nil {
		return maskAny(err)
	}
	return nil
}
