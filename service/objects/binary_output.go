package objects

import (
	"context"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/BinkyNet/mqp"
	"github.com/binkynet/LocalWorker/service/devices"
	"github.com/binkynet/LocalWorker/service/mqtt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	binaryOutputType = &ObjectType{
		TopicSuffix: mqp.BinaryMessage{}.TopicSuffix(),
		NextMessage: func(ctx context.Context, log zerolog.Logger, subscription mqtt.Subscription, service Service) error {
			var msg mqp.BinaryMessage
			if err := subscription.NextMsg(ctx, &msg); err != nil {
				log.Debug().Err(err).Msg("NextMsg failed")
				return maskAny(err)
			}
			if msg.IsRequest() {
				log = log.With().Str("address", string(msg.Address)).Logger()
				//log.Debug().Msg("got message")
				if obj, found := service.ObjectByAddress(msg.Address); found {
					if x, ok := obj.(*binaryOutput); ok {
						if err := x.ProcessMessage(ctx, msg); err != nil {
							return maskAny(err)
						}
					} else {
						return errors.Errorf("Expected object of type binaryOutput")
					}
				} else {
					log.Debug().Msg("object not found")
				}
			} else {
				log.Debug().Msg("ignoring non-request message")
			}
			return nil
		},
	}
)

type binaryOutput struct {
	log          zerolog.Logger
	config       model.Object
	address      mqp.ObjectAddress
	outputDevice devices.GPIO
	pin          model.DeviceIndex
}

// newBinaryOutput creates a new binary-output object for the given configuration.
func newBinaryOutput(oid model.ObjectID, address mqp.ObjectAddress, config model.Object, log zerolog.Logger, devService devices.Service) (Object, error) {
	if config.Type != model.ObjectTypeBinaryOutput {
		return nil, errors.Wrapf(model.ValidationError, "Invalid object type '%s'", config.Type)
	}
	conn, ok := config.Connections[model.ConnectionNameOutput]
	if !ok {
		return nil, errors.Wrapf(model.ValidationError, "Pin '%s' not found in object '%s'", model.ConnectionNameOutput, oid)
	}
	if len(conn.Pins) != 1 {
		return nil, errors.Wrapf(model.ValidationError, "Pin '%s' must have 1 pin in object '%s', got %d", model.ConnectionNameOutput, oid, len(conn.Pins))
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
		return nil, errors.Wrapf(model.ValidationError, "Pin '%s' in object '%s' is out of range. Got %d. Range [1..%d]", model.ConnectionNameOutput, oid, pin, gpio.PinCount())
	}
	return &binaryOutput{
		log:          log,
		config:       config,
		address:      address,
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
	for i := 0; i < 5; i++ {
		if err := o.outputDevice.SetDirection(ctx, o.pin, devices.PinDirectionOutput); err != nil {
			return maskAny(err)
		}
	}
	return nil
}

// Run the object until the given context is cancelled.
func (o *binaryOutput) Run(ctx context.Context, mqttService mqtt.Service, topicPrefix, moduleID string) error {
	// Nothing to do here
	return nil
}

// ProcessMessage acts upons a given request.
func (o *binaryOutput) ProcessMessage(ctx context.Context, r mqp.BinaryMessage) error {
	log := o.log.With().Bool("value", r.Value).Logger()
	log.Debug().Msg("got request")
	if err := o.outputDevice.Set(ctx, o.pin, r.Value); err != nil {
		log.Debug().Err(err).Msg("GPIO.set failed")
		return maskAny(err)
	}
	return nil
}

// ProcessPowerMessage acts upons a given power message.
func (o *binaryOutput) ProcessPowerMessage(ctx context.Context, m mqp.PowerMessage) error {
	return nil // TODO
}
