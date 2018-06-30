package objects

import (
	"context"
	"time"

	aerr "github.com/ewoutp/go-aggregate-error"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/BinkyNet/mq"
	"github.com/binkynet/LocalWorker/service/devices"
	"github.com/binkynet/LocalWorker/service/mqtt"
	"github.com/pkg/errors"
)

// Service contains the API that is exposed by the object service.
type Service interface {
	// ObjectByAddress returns the object with given address.
	// Return false if not found
	ObjectByAddress(address mq.ObjectAddress) (Object, bool)
	// Configure is called once to put all objects in the desired state.
	Configure(ctx context.Context) error
	// Run all required topics until the given context is cancelled.
	Run(ctx context.Context, mqttService mqtt.Service) error
}

type service struct {
	objects           map[mq.ObjectAddress]Object
	configuredObjects map[mq.ObjectAddress]Object
	topicPrefix       string
	log               zerolog.Logger
}

// NewService instantiates a new Service and Object's for the given
// object configurations.
func NewService(moduleID string, configs map[model.ObjectID]model.Object, topicPrefix string, devService devices.Service, log zerolog.Logger) (Service, error) {
	s := &service{
		objects:           make(map[mq.ObjectAddress]Object),
		configuredObjects: make(map[mq.ObjectAddress]Object),
		topicPrefix:       topicPrefix,
		log:               log.With().Str("component", "object-service").Logger(),
	}
	for id, c := range configs {
		var obj Object
		var err error
		address := mq.JoinModuleLocal(moduleID, string(id))
		log = log.With().
			Str("address", string(address)).
			Str("type", string(c.Type)).
			Logger()
		log.Debug().Msg("creating object...")
		switch c.Type {
		case model.ObjectTypeBinarySensor:
			obj, err = newBinarySensor(id, address, c, log, devService)
		case model.ObjectTypeBinaryOutput:
			obj, err = newBinaryOutput(id, address, c, log, devService)
		case model.ObjectTypeRelaySwitch:
			obj, err = newRelaySwitch(id, address, c, log, devService)
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
func (s *service) ObjectByAddress(address mq.ObjectAddress) (Object, bool) {
	dev, ok := s.configuredObjects[address]
	return dev, ok
}

// Configure is called once to put all objects in the desired state.
func (s *service) Configure(ctx context.Context) error {
	var ae aerr.AggregateError
	configuredObjects := make(map[mq.ObjectAddress]Object)
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
	if len(s.configuredObjects) == 0 {
		s.log.Warn().Msg("no configured objects, just waiting for context to be cancelled")
		<-ctx.Done()
	} else {
		g, ctx := errgroup.WithContext(ctx)
		visitedTypes := make(map[*ObjectType]struct{})
		for _, obj := range s.configuredObjects {
			// Run the object itself
			g.Go(func() error {
				if err := obj.Run(ctx, mqttService, s.topicPrefix); err != nil {
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
				if err := objType.Run(ctx, s.log, mqttService, s.topicPrefix, s); err != nil {
					return maskAny(err)
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return maskAny(err)
		}
	}
	return nil
}
