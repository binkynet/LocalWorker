package objects

import (
	"context"
	"path"

	aerr "github.com/ewoutp/go-aggregate-error"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/binkynet/BinkyNet/model"
	"github.com/binkynet/LocalWorker/service/devices"
	"github.com/binkynet/LocalWorker/service/mqtt"
	"github.com/pkg/errors"
)

// Service contains the API that is exposed by the object service.
type Service interface {
	// ObjectByID returns the object with given ID.
	// Return false if not found
	ObjectByID(id string) (Object, bool)
	// Configure is called once to put all objects in the desired state.
	Configure(ctx context.Context) error
	// Run all required topics until the given context is cancelled.
	Run(ctx context.Context, mqttService mqtt.Service) error
}

type service struct {
	objects           map[string]Object
	configuredObjects map[string]Object
	topicPrefix       string
	log               zerolog.Logger
}

// NewService instantiates a new Service and Object's for the given
// object configurations.
func NewService(moduleID string, configs []model.Object, topicPrefix string, devService devices.Service, log zerolog.Logger) (Service, error) {
	s := &service{
		objects:           make(map[string]Object),
		configuredObjects: make(map[string]Object),
		topicPrefix:       topicPrefix,
		log:               log,
	}
	for _, c := range configs {
		var obj Object
		var err error
		address := path.Join(moduleID, c.ID)
		log = log.With().
			Str("address", address).
			Str("type", string(c.Type)).
			Logger()
		log.Debug().Msg("creating object...")
		switch c.Type {
		case model.ObjectTypeBinarySensor:
			obj, err = newBinarySensor(address, c, log, devService)
		case model.ObjectTypeBinaryOutput:
			obj, err = newBinaryOutput(address, c, log, devService)
		case model.ObjectTypeRelaySwitch:
			obj, err = newRelaySwitch(address, c, log, devService)
		default:
			return nil, errors.Wrapf(model.ValidationError, "Unsupported object type '%s'", c.Type)
		}
		if err != nil {
			return nil, maskAny(err)
		}
		s.objects[address] = obj
	}
	s.log.Debug().Msgf("created %d objects", len(s.objects))
	return s, nil
}

// ObjectByID returns the object with given ID.
// Return false if not found or not configured.
func (s *service) ObjectByID(id string) (Object, bool) {
	dev, ok := s.configuredObjects[id]
	return dev, ok
}

// Configure is called once to put all objects in the desired state.
func (s *service) Configure(ctx context.Context) error {
	var ae aerr.AggregateError
	configuredObjects := make(map[string]Object)
	for id, d := range s.objects {
		if err := d.Configure(ctx); err != nil {
			s.log.Error().Err(err).Str("id", id).Msg("Failed to configure object")
			ae.Add(maskAny(err))
		} else {
			s.log.Debug().Str("id", id).Msg("configured object")
			configuredObjects[id] = d
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
