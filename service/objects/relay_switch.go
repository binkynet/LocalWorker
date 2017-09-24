package objects

import (
	"context"
	"sync"
	"time"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/BinkyNet/mq"
	"github.com/binkynet/LocalWorker/service/devices"
	"github.com/binkynet/LocalWorker/service/mqtt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	_ switchAPI = &relaySwitch{}
)

type relaySwitch struct {
	mutex            sync.Mutex
	log              zerolog.Logger
	config           model.Object
	address          string
	straight         relaySwitchDirection
	off              relaySwitchDirection
	disableAllNeeded bool
	disableAllTime   time.Time
}

type relaySwitchDirection struct {
	device devices.GPIO
	pin    int
}

func (rsd relaySwitchDirection) activateRelay(ctx context.Context) error {
	if err := rsd.device.Set(ctx, rsd.pin, true); err != nil {
		return maskAny(err)
	}
	return nil
}

func (rsd relaySwitchDirection) deactivateRelay(ctx context.Context) error {
	if err := rsd.device.Set(ctx, rsd.pin, false); err != nil {
		return maskAny(err)
	}
	return nil
}

// newRelaySwitch creates a new relay-switch object for the given configuration.
func newRelaySwitch(address string, config model.Object, log zerolog.Logger, devService devices.Service) (Object, error) {
	if config.Type != model.ObjectTypeRelaySwitch {
		return nil, errors.Wrapf(model.ValidationError, "Invalid object type '%s'", config.Type)
	}
	straightPin, err := getSinglePin(config, model.PinNameStraightRelay)
	if err != nil {
		return nil, maskAny(err)
	}
	straightDev, err := getGPIOForPin(straightPin, devService)
	if err != nil {
		return nil, errors.Wrapf(err, "%s: (pin %s in object %s)", err.Error(), model.PinNameStraightRelay, config.ID)
	}
	offPin, err := getSinglePin(config, model.PinNameOffRelay)
	if err != nil {
		return nil, maskAny(err)
	}
	offDev, err := getGPIOForPin(offPin, devService)
	if err != nil {
		return nil, errors.Wrapf(err, "%s: (pin %s in object %s)", err.Error(), model.PinNameOffRelay, config.ID)
	}
	return &relaySwitch{
		log:      log,
		config:   config,
		address:  address,
		straight: relaySwitchDirection{straightDev, straightPin.Pin},
		off:      relaySwitchDirection{offDev, offPin.Pin},
	}, nil
}

// Return the type of this object.
func (o *relaySwitch) Type() *ObjectType {
	return switchType
}

// Configure is called once to put the object in the desired state.
func (o *relaySwitch) Configure(ctx context.Context) error {
	if err := o.straight.device.SetDirection(ctx, o.straight.pin, devices.PinDirectionOutput); err != nil {
		return maskAny(err)
	}
	if err := o.straight.deactivateRelay(ctx); err != nil {
		return maskAny(err)
	}
	if err := o.off.device.SetDirection(ctx, o.off.pin, devices.PinDirectionOutput); err != nil {
		return maskAny(err)
	}
	if err := o.off.deactivateRelay(ctx); err != nil {
		return maskAny(err)
	}
	return nil
}

// Run the object until the given context is cancelled.
func (o *relaySwitch) Run(ctx context.Context, mqttService mqtt.Service, topicPrefix string) error {
	for {
		o.mutex.Lock()
		if o.disableAllNeeded && time.Now().After(o.disableAllTime) {
			hasErrors := false
			if err := o.straight.deactivateRelay(ctx); err != nil {
				o.log.Warn().Err(err).Msg("Straight relay did not de-activate")
				hasErrors = true
			}
			if err := o.off.deactivateRelay(ctx); err != nil {
				o.log.Warn().Err(err).Msg("Off relay did not de-activate")
				hasErrors = true
			}
			if !hasErrors {
				o.disableAllNeeded = false
			}
		}
		o.mutex.Unlock()
		select {
		case <-time.After(time.Millisecond * 10):
			// Continue
		case <-ctx.Done():
			if o.disableAllNeeded {
				// Do one more loop to disable relays
				o.disableAllTime = time.Now().Add(-time.Second)
			} else {
				return nil
			}
		}
	}
}

// ProcessMessage acts upons a given request.
func (o *relaySwitch) ProcessMessage(ctx context.Context, r mq.SwitchRequest) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	log := o.log.With().Str("direction", string(r.Direction)).Logger()
	log.Debug().Msg("got request")

	switch r.Direction {
	case mq.SwitchDirectionStraight:
		if err := o.off.deactivateRelay(ctx); err != nil {
			return maskAny(err)
		}
		if err := o.straight.activateRelay(ctx); err != nil {
			return maskAny(err)
		}
	case mq.SwitchDirectionOff:
		if err := o.straight.deactivateRelay(ctx); err != nil {
			return maskAny(err)
		}
		if err := o.off.activateRelay(ctx); err != nil {
			return maskAny(err)
		}
	}
	o.disableAllNeeded = true
	o.disableAllTime = time.Now().Add(time.Second)

	return nil
}
