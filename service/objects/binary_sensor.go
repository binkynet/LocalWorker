package objects

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/BinkyNet/mqp"
	"github.com/binkynet/BinkyNet/mqtt"
	"github.com/binkynet/LocalWorker/service/devices"
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
	address     mqp.ObjectAddress
	inputDevice devices.GPIO
	pin         model.DeviceIndex
	sendNow     int32
}

// newBinarySensor creates a new binary-sensor object for the given configuration.
func newBinarySensor(oid model.ObjectID, address mqp.ObjectAddress, config model.Object, log zerolog.Logger, devService devices.Service) (Object, error) {
	if config.Type != model.ObjectTypeBinarySensor {
		return nil, errors.Wrapf(model.ValidationError, "Invalid object type '%s'", config.Type)
	}
	conn, ok := config.Connections[model.ConnectionNameSensor]
	if !ok {
		return nil, errors.Wrapf(model.ValidationError, "Pin '%s' not found in object '%s'", model.ConnectionNameSensor, oid)
	}
	if len(conn.Pins) != 1 {
		return nil, errors.Wrapf(model.ValidationError, "Pin '%s' must have 1 pin in object '%s', got %d", model.ConnectionNameSensor, oid, len(conn.Pins))
	}
	device, ok := devService.DeviceByID(conn.Pins[0].DeviceID)
	if !ok {
		return nil, errors.Wrapf(model.ValidationError, "Device '%s' not found in object '%s'", conn.Pins[0].DeviceID, oid)
	}
	gpio, ok := device.(devices.GPIO)
	if !ok {
		return nil, errors.Wrapf(model.ValidationError, "Device '%s' in object '%s' is not a GPIO", conn.Pins[0].DeviceID, oid)
	}
	pin := conn.Pins[0].Index
	if pin < 1 || uint(pin) > gpio.PinCount() {
		return nil, errors.Wrapf(model.ValidationError, "Pin '%s' in object '%s' is out of range. Got %d. Range [1..%d]", model.ConnectionNameSensor, oid, pin, gpio.PinCount())
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
func (o *binarySensor) Run(ctx context.Context, mqttService mqtt.Service, topicPrefix, moduleID string) error {
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
				log.Info().Err(err).Msg("Get value failed")
			}
			recentErrors++
		} else {
			recentErrors = 0
			force := atomic.CompareAndSwapInt32(&o.sendNow, 1, 0)
			if force || lastValue != value || changes == 0 {
				// Send feedback data
				log = log.With().Bool("value", value).Logger()
				log.Debug().Bool("force", force).Msg("change detected")
				msg := mqp.BinaryMessage{
					ObjectMessageBase: mqp.ObjectMessageBase{
						MessageBase: mqp.MessageBase{
							Mode: mqp.MessageModeActual,
						},
						Address: o.address,
					},
					Value: value,
				}
				topic := mqp.CreateObjectTopic(topicPrefix, moduleID, msg)
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
}

// ProcessPowerMessage acts upons a given power message.
func (o *binarySensor) ProcessPowerMessage(ctx context.Context, m mqp.PowerMessage) error {
	if m.Active && m.IsRequest() {
		atomic.StoreInt32(&o.sendNow, 1)
	}
	return nil
}
