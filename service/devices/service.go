package devices

import (
	"context"
	"fmt"
	"path"
	"time"

	aerr "github.com/ewoutp/go-aggregate-error"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/BinkyNet/mqtt"
	"github.com/binkynet/LocalWorker/service/bridge"
	"github.com/pkg/errors"
)

// Service contains the API that is exposed by the device service.
type Service interface {
	// DeviceByID returns the device with given ID.
	// Return false if not found
	DeviceByID(id model.DeviceID) (Device, bool)
	// Configure is called once to put all devices in the desired state.
	Configure(ctx context.Context) error
	// Run the service until the given context is canceled.
	Run(ctx context.Context, mqttService mqtt.Service, topicPrefix string) error
	// Close brings all devices back to a safe state.
	Close() error
}

type service struct {
	log               zerolog.Logger
	devices           map[model.DeviceID]Device
	configuredDevices map[model.DeviceID]Device
	bus               *bridge.I2CBus
}

// NewService instantiates a new Service and Device's for the given
// device configurations.
func NewService(configs map[model.DeviceID]model.Device, bus *bridge.I2CBus, log zerolog.Logger) (Service, error) {
	s := &service{
		log:               log.With().Str("component", "device-service").Logger(),
		devices:           make(map[model.DeviceID]Device),
		configuredDevices: make(map[model.DeviceID]Device),
		bus:               bus,
	}
	for id, c := range configs {
		var dev Device
		var err error
		switch c.Type {
		case model.DeviceTypeMCP23008:
			dev, err = newMcp23008(c, bus)
		case model.DeviceTypeMCP23017:
			dev, err = newMcp23017(c, bus)
		case model.DeviceTypePCA9685:
			dev, err = newPCA9685(c, bus)
		default:
			return nil, errors.Wrapf(model.ValidationError, "Unsupported device type '%s'", c.Type)
		}
		if err != nil {
			return nil, maskAny(err)
		}
		s.devices[id] = dev
	}
	return s, nil
}

// DeviceByID returns the device with given ID.
// Return false if not found or not configured.
func (s *service) DeviceByID(id model.DeviceID) (Device, bool) {
	dev, ok := s.configuredDevices[id]
	return dev, ok
}

// Configure is called once to put all devices in the desired state.
func (s *service) Configure(ctx context.Context) error {
	var ae aerr.AggregateError
	configuredDevices := make(map[model.DeviceID]Device)
	for id, d := range s.devices {
		log.Debug().
			Str("id", string(id)).
			Msg("Configuring device")
		if err := d.Configure(ctx); err != nil {
			log.Warn().
				Err(err).
				Str("id", string(id)).
				Msg("Failed to configure device")
			ae.Add(maskAny(err))
		} else {
			configuredDevices[id] = d
		}
	}
	s.configuredDevices = configuredDevices
	log.Info().Int("count", len(configuredDevices)).Msg("Configured devices")
	return ae.AsError()
}

// Run the service until the given context is canceled.
func (s *service) Run(ctx context.Context, mqttService mqtt.Service, topicPrefix string) error {
	msg := struct {
		Slaves []string `json:"slave-addresses"`
	}{}
	topic := path.Join(topicPrefix, "bus")
	s.log.Debug().Str("topic", topic).Msg("Broadcasting bus addresses...")
	for {
		// Poll slave addresses
		addrs := s.bus.DetectSlaveAddresses()
		msg.Slaves = nil
		for _, addr := range addrs {
			msg.Slaves = append(msg.Slaves, fmt.Sprintf("0x%0x", addr))
		}
		mqttService.Publish(ctx, msg, topic, mqtt.QosDefault)

		select {
		case <-time.After(time.Minute):
			// Continue
		case <-ctx.Done():
			// Context canceled
			return nil
		}
	}
}

// Close brings all devices back to a safe state.
func (s *service) Close() error {
	var ae aerr.AggregateError
	for _, d := range s.devices {
		if err := d.Close(); err != nil {
			ae.Add(maskAny(err))
		}
	}
	return ae.AsError()
}
