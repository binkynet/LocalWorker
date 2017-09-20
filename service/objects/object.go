package objects

import (
	"context"
	"path"

	"github.com/binkynet/LocalWorker/service/mqtt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Object contains the API supported by all types of objects.
type Object interface {
	// Return the type of this object.
	Type() *ObjectType
	// Configure is called once to put the object in the desired state.
	Configure(ctx context.Context) error
	// Run the object until the given context is cancelled.
	Run(ctx context.Context) error
}

// ObjectType contains the API supported a specific type of object.
// There will be a single instances of a specific ObjecType that is used by all Object instances.
type ObjectType struct {
	// Return the suffix of the topic name to listen on for messages
	TopicSuffix string
	// NextMessage waits for the next message on given subscription
	// and processes it.
	NextMessage func(ctx context.Context, log zerolog.Logger, subscription mqtt.Subscription, service Service) error
}

// Run subscribes to the intended topic and process incoming messages
// until the given context is cancelled.
func (t *ObjectType) Run(ctx context.Context, log zerolog.Logger, mqttService mqtt.Service, topicPrefix string, service Service) error {
	topic := path.Join(topicPrefix, t.TopicSuffix)
	log = log.With().
		Str("topic", topic).
		Logger()
	subscription, err := mqttService.Subscribe(ctx, topic, mqtt.QosAsLeastOnce)
	if err != nil {
		log.Error().Err(err).Msg("Failed to subscribe to MQTT topic")
		return maskAny(err)
	}
	log.Debug().Msg("subscribed to MQTT topic")
	defer subscription.Close()

	for {
		// Wait for next message and process it
		if err := t.NextMessage(ctx, log, subscription, service); err != nil {
			if errors.Cause(err) == context.Canceled {
				return nil
			}
			log.Error().Err(err).Msg("Failed to get next message")
			return maskAny(err)
		}
	}
}
