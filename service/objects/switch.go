// Copyright 2020 Ewout Prangsma
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author Ewout Prangsma
//

package objects

import (
	"context"

	model "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	switchType = &ObjectType{
		Run: func(ctx context.Context, log zerolog.Logger, requests RequestService, actuals *ServiceActuals, service Service, moduleID string) error {
			cancel := requests.RegisterSwitchRequestReceiver(func(msg model.Switch) error {
				log := log.With().Str("address", string(msg.Address)).Logger()
				//log.Debug().Msg("got message")
				if obj, isGlobal, found := service.ObjectByAddress(msg.Address); found {
					if x, ok := obj.(switchAPI); ok {
						if err := x.ProcessMessage(ctx, msg); err != nil {
							return err
						}
					} else {
						return errors.Errorf("Expected object of type switchAPI")
					}
				} else if !isGlobal {
					log.Debug().Msg("object not found")
				}
				return nil
			})
			defer cancel()
			<-ctx.Done()
			return nil
		},
	}
)

type switchAPI interface {
	// ProcessMessage acts upons a given request.
	ProcessMessage(ctx context.Context, r model.Switch) error
}
