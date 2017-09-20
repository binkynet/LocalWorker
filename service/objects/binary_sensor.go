package objects

import (
	"context"
	"path"
	"time"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/BinkyNet/mq"
	"github.com/binkynet/LocalWorker/service/devices"
	"github.com/binkynet/LocalWorker/service/mqtt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	binarySensorType = &ObjectType{
		TopicSuffix: "",
		NextMessage: nil,
	}
)

type binarySensor struct {
	log         zerolog.Logger
	config      model.Object
	address     string
	inputDevice devices.GPIO
	pin         int
}

// newBinarySensor creates a new binary-sensor object for the given configuration.
func newBinarySensor(address string, config model.Object, log zerolog.Logger, devService devices.Service) (Object, error) {
	if config.Type != model.ObjectTypeBinarySensor {
		return nil, errors.Wrapf(model.ValidationError, "Invalid object type '%s'", config.Type)
	}
	pins, ok := config.Pins[model.PinNameSensor]
	if !ok {
		return nil, errors.Wrapf(model.ValidationError, "Pin '%s' not found in object '%s'", model.PinNameSensor, config.ID)
	}
	if len(pins) != 1 {
		return nil, errors.Wrapf(model.ValidationError, "Pin '%s' must have 1 pin in object '%s', got %d", model.PinNameSensor, config.ID, len(pins))
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
		return nil, errors.Wrapf(model.ValidationError, "Pin '%s' in object '%s' is out of range. Got %d. Range [1..%d]", model.PinNameSensor, config.ID, pin, gpio.PinCount())
	}
	return &binarySensor{
		log:         log,
		config:      config,
		address:     address,
		inputDevice: gpio,
		pin:         pin,
	}, nil
}

// Return the type of this object.
func (o *binarySensor) Type() *ObjectType {
	return binarySensorType
}

// Configure is called once to put the object in the desired state.
func (o *binarySensor) Configure(ctx context.Context) error {
	if err := o.inputDevice.SetDirection(ctx, o.pin, devices.PinDirectionInput); err != nil {
		return maskAny(err)
	}
	return nil
}

// Run the object until the given context is cancelled.
func (o *binarySensor) Run(ctx context.Context, mqttService mqtt.Service, topicPrefix string) error {
	lastValue := false
	changes := 0
	recentErrors := 0
	log := o.log
	for {
		delay := time.Millisecond * 10

		// Read state
		value, err := o.inputDevice.Get(ctx, o.pin)
		if err != nil {
			// Try again soon
			if recentErrors == 0 {
				o.log.Info().Err(err).Msg("Get value failed")
			}
			recentErrors++
		} else {
			recentErrors = 0
			if lastValue != value || changes == 0 {
				// Send feedback data
				log = log.With().Bool("value", value).Logger()
				log.Debug().Msg("change detected")
				msg := mq.BinaryOutputFeedback{
					Address: o.address,
					Value:   value,
				}
				topic := path.Join(topicPrefix, msg.TopicSuffix())
				lctx, cancel := context.WithTimeout(ctx, time.Millisecond*250)
				if err := mqttService.Publish(lctx, msg, topic, mqtt.QosAsLeastOnce); err != nil {
					o.log.Debug().Err(err).Msg("Publish failed")
				}
				cancel()
				log.Debug().Str("topic", topic).Msg("change published")
				lastValue = value
				changes++
			}
		}

		// Wait a bit
		select {
		case <-time.After(delay):
			// Continue
		case <-ctx.Done():
			// Context cancelled
			return nil
		}
	}
	return nil
}
