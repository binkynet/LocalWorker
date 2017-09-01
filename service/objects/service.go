package objects

import (
	"context"

	aerr "github.com/ewoutp/go-aggregate-error"

	"github.com/binkynet/BinkyNet/model"
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
	// Subscribe to all required topics
	Subscribe(mqttService mqtt.Service) error
	// Close all subscriptions
	Close() error
}

type service struct {
	objects           map[string]Object
	configuredObjects map[string]Object
}

// NewService instantiates a new Service and Object's for the given
// object configurations.
func NewService(configs []model.Object) (Service, error) {
	s := &service{
		objects:           make(map[string]Object),
		configuredObjects: make(map[string]Object),
	}
	for _, c := range configs {
		var obj Object
		var err error
		switch c.Type {
		default:
			return nil, errors.Wrapf(model.ValidationError, "Unsupported object type '%s'", c.Type)
		}
		if err != nil {
			return nil, maskAny(err)
		}
		s.objects[c.ID] = obj
	}
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
			ae.Add(maskAny(err))
		} else {
			configuredObjects[id] = d
		}
	}
	s.configuredObjects = configuredObjects
	return ae.AsError()
}

// Subscribe to all required topics
func (s *service) Subscribe(mqttService mqtt.Service) error {
	// TODO
	return nil
}

// Close all subscriptions
func (s *service) Close() error {
	// TODO
	return nil
}
