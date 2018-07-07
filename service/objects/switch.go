package objects

import (
	"context"

	"github.com/binkynet/BinkyNet/mqp"
	"github.com/binkynet/BinkyNet/mqtt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	switchType = &ObjectType{
		TopicSuffix: mqp.SwitchMessage{}.TopicSuffix(),
		NextMessage: func(ctx context.Context, log zerolog.Logger, subscription mqtt.Subscription, service Service) error {
			var msg mqp.SwitchMessage
			msgID, err := subscription.NextMsg(ctx, &msg)
			if err != nil {
				log.Debug().Err(err).Msg("NextMsg failed")
				return maskAny(err)
			}
			if msg.IsRequest() {
				log = log.With().Int("msg-id", msgID).Str("address", string(msg.Address)).Logger()
				//log.Debug().Msg("got message")
				if obj, found := service.ObjectByAddress(msg.Address); found {
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
			} else {
				log.Debug().Msg("ignoring non-request message")
			}
			return nil
		},
	}
)

type switchAPI interface {
	// ProcessMessage acts upons a given request.
	ProcessMessage(ctx context.Context, r mqp.SwitchMessage) error
}
