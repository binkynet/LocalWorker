package objects

import (
	"context"

	"github.com/binkynet/BinkyNet/mq"
	"github.com/binkynet/LocalWorker/service/mqtt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	switchType = &ObjectType{
		TopicSuffix: mq.SwitchRequest{}.TopicSuffix(),
		NextMessage: func(ctx context.Context, log zerolog.Logger, subscription mqtt.Subscription, service Service) error {
			var msg mq.SwitchRequest
			if err := subscription.NextMsg(ctx, &msg); err != nil {
				log.Debug().Err(err).Msg("NextMsg failed")
				return maskAny(err)
			}
			log = log.With().Str("address", msg.Address).Logger()
			//log.Debug().Msg("got message")
			if obj, found := service.ObjectByID(msg.Address); found {
				if x, ok := obj.(switchAPI); ok {
					if err := x.ProcessMessage(ctx, msg); err != nil {
						return maskAny(err)
					}
				} else {
					return errors.Errorf("Expected object of type switchAPI")
				}
			} else {
				log.Debug().Msg("object not found")
			}
			return nil
		},
	}
)

type switchAPI interface {
	// ProcessMessage acts upons a given request.
	ProcessMessage(ctx context.Context, r mq.SwitchRequest) error
}
