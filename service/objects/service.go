package objects

import (
	"context"
	"path"
	"time"

	aerr "github.com/ewoutp/go-aggregate-error"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/BinkyNet/mqp"
	"github.com/binkynet/BinkyNet/mqtt"
	"github.com/binkynet/LocalWorker/service/devices"
	"github.com/pkg/errors"
)

// Service contains the API that is exposed by the object service.
type Service interface {
	// ObjectByAddress returns the object with given address.
	// Return false if not found
	ObjectByAddress(address mqp.ObjectAddress) (Object, bool)
	// Configure is called once to put all objects in the desired state.
	Configure(ctx context.Context) error
	// Run all required topics until the given context is cancelled.
	Run(ctx context.Context, mqttService mqtt.Service) error
}

type service struct {
	startTime         time.Time
	moduleID          string
	objects           map[mqp.ObjectAddress]Object
	configuredObjects map[mqp.ObjectAddress]Object
	topicPrefix       string
	programVersion    string
	log               zerolog.Logger
}

// NewService instantiates a new Service and Object's for the given
// object configurations.
func NewService(moduleID string, programVersion string, configs map[model.ObjectID]model.Object, topicPrefix string, devService devices.Service, log zerolog.Logger) (Service, error) {
	s := &service{
		startTime:         time.Now(),
		moduleID:          moduleID,
		objects:           make(map[mqp.ObjectAddress]Object),
		configuredObjects: make(map[mqp.ObjectAddress]Object),
		topicPrefix:       topicPrefix,
		programVersion:    programVersion,
		log:               log.With().Str("component", "object-service").Logger(),
	}
	for id, c := range configs {
		var obj Object
		var err error
		address := mqp.JoinModuleLocal(moduleID, string(id))
		log = log.With().
			Str("address", string(address)).
			Str("type", string(c.Type)).
			Logger()
		log.Debug().Msg("creating object...")
		switch c.Type {
		case model.ObjectTypeBinarySensor:
			obj, err = newBinarySensor(moduleID, id, address, c, log, devService)
		case model.ObjectTypeBinaryOutput:
			obj, err = newBinaryOutput(moduleID, id, address, c, log, devService)
		case model.ObjectTypeRelaySwitch:
			obj, err = newRelaySwitch(moduleID, id, address, c, log, devService)
		case model.ObjectTypeServoSwitch:
			obj, err = newServoSwitch(moduleID, id, address, c, log, devService)
		default:
			err = errors.Wrapf(model.ValidationError, "Unsupported object type '%s'", c.Type)
		}
		if err != nil {
			log.Error().Err(err).Msg("Failed to create object")
			//return nil, maskAny(err)
		} else {
			s.objects[address] = obj
		}
	}
	s.log.Debug().Msgf("created %d objects", len(s.objects))
	return s, nil
}

// ObjectByAddress returns the object with given object address.
// Return false if not found or not configured.
func (s *service) ObjectByAddress(address mqp.ObjectAddress) (Object, bool) {
	dev, ok := s.configuredObjects[address]
	return dev, ok
}

// Configure is called once to put all objects in the desired state.
func (s *service) Configure(ctx context.Context) error {
	var ae aerr.AggregateError
	configuredObjects := make(map[mqp.ObjectAddress]Object)
	for addr, d := range s.objects {
		time.Sleep(time.Millisecond * 200)
		if err := d.Configure(ctx); err != nil {
			s.log.Error().Err(err).Str("address", string(addr)).Msg("Failed to configure object")
			ae.Add(maskAny(err))
		} else {
			s.log.Debug().Str("address", string(addr)).Msg("configured object")
			configuredObjects[addr] = d
		}
	}
	s.configuredObjects = configuredObjects
	return ae.AsError()
}

// Run all required topics until the given context is cancelled.
func (s *service) Run(ctx context.Context, mqttService mqtt.Service) error {
	defer func() {
		s.log.Debug().Msg("Run Objects ended")
	}()
	if len(s.configuredObjects) == 0 {
		s.log.Warn().Msg("no configured objects, just waiting for context to be cancelled")
		<-ctx.Done()
	} else {
		g, ctx := errgroup.WithContext(ctx)
		// Keep sending ping messages
		g.Go(func() error { s.sendPingMessages(ctx, mqttService); return nil })

		// Receive power messages
		g.Go(func() error { s.receivePowerMessages(ctx, mqttService); return nil })

		// Run all objects & object types.
		visitedTypes := make(map[*ObjectType]struct{})
		for addr, obj := range s.configuredObjects {
			// Run the object itself
			addr := addr // Bring range variables in scope
			obj := obj
			g.Go(func() error {
				s.log.Debug().Str("address", string(addr)).Msg("Running object")
				if err := obj.Run(ctx, mqttService, s.topicPrefix, s.moduleID); err != nil {
					return maskAny(err)
				}
				return nil
			})

			// Run the message loop for the type of object (if not running already)
			objType := obj.Type()
			if _, found := visitedTypes[objType]; found {
				// Type already running
				continue
			}
			visitedTypes[objType] = struct{}{}
			g.Go(func() error {
				s.log.Debug().Str("type", s.topicPrefix).Msg("starting object type")
				if err := objType.Run(ctx, s.log, mqttService, s.topicPrefix, s.moduleID, s); err != nil {
					return maskAny(err)
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			s.log.Warn().Err(err).Msg("Run Objects failed")
			return maskAny(err)
		}
	}
	return nil
}

// sendPingMessages keeps sending ping messages until the given context is canceled.
func (s *service) sendPingMessages(ctx context.Context, mqttService mqtt.Service) {
	topic := mqp.CreateGlobalTopic(s.topicPrefix, mqp.PingMessage{})
	log := s.log.With().Str("topic", topic).Logger()
	log.Info().Str("topic", topic).Msg("Sending ping messages")
	defer func() {
		log.Info().Str("topic", topic).Msg("Stopped sending ping messages")
	}()
	for {
		// Send ping
		msg := mqp.PingMessage{
			GlobalMessageBase: mqp.NewGlobalMessageBase(s.moduleID, mqp.MessageModeActual),
			ProtocolVersion:   mqp.ProtocolVersion,
			Version:           s.programVersion,
			Uptime:            int(time.Since(s.startTime).Seconds()),
		}
		delay := time.Second * 15
		if err := mqttService.Publish(ctx, msg, topic, mqtt.QosDefault); err != nil {
			log.Info().Err(err).Msg("Failed to send ping message")
			delay = time.Second * 5
		}

		// Wait
		select {
		case <-time.After(delay):
			// Continue
		case <-ctx.Done():
			// Context canceled
			return
		}
	}
}

// Run subscribes to the intended topic and process incoming messages
// until the given context is cancelled.
func (s *service) receivePowerMessages(ctx context.Context, mqttService mqtt.Service) error {
	topic := path.Join(s.topicPrefix, "global/power")
	log := s.log.With().
		Str("topic", topic).
		Logger()
	subscription, err := mqttService.Subscribe(ctx, topic, mqtt.QosDefault)
	if err != nil {
		log.Error().Err(err).Msg("Failed to subscribe to MQTT topic")
		return maskAny(err)
	}
	log.Debug().Msg("subscribed to MQTT topic")
	defer subscription.Close()

	for {
		// Wait for next message and process it
		var msg mqp.PowerMessage
		msgID, err := subscription.NextMsg(ctx, &msg)
		if err != nil {
			if errors.Cause(err) == context.Canceled {
				return nil
			}
			log.Debug().Err(err).Msg("NextMsg failed")
		} else if msg.IsRequest() {
			// Process power request
			log.Debug().Int("msg-id", msgID).Bool("active", msg.Active).Msg("Receiver power request")
			for _, obj := range s.configuredObjects {
				// Run the object itself
				go func(obj Object) {
					if err := obj.ProcessPowerMessage(ctx, msg); err != nil {
						log.Info().Err(err).Msg("Object failed to process PowerMessage")
					}
				}(obj)
			}
		}
	}
}
