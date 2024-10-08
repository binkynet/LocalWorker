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

type outputType string

const outputTypeInstance outputType = "outputType"

func (t outputType) String() string {
	return string(t)
}

func (outputType) Run(ctx context.Context, log zerolog.Logger, requests RequestService, statuses StatusService, service Service, moduleID string) error {
	cancel := requests.RegisterOutputRequestReceiver(func(msg model.Output) error {
		log := log.With().Str("address", string(msg.Address)).Logger()
		//log.Debug().Msg("got message")
		if obj, isGlobal, found := service.ObjectByAddress(msg.Address); found {
			if x, ok := obj.(outputAPI); ok {
				// Process message
				if err := x.ProcessMessage(ctx, msg); err != nil {
					return err
				}
				// Set metrics
				x.UpdateMetrics(msg)
			} else {
				return errors.Errorf("Expected object of type outputAPI")
			}
		} else if !isGlobal {
			log.Debug().Msg("output object not found")
		}
		return nil
	})
	defer cancel()
	<-ctx.Done()
	return nil
}

type outputAPI interface {
	// ProcessMessage acts upons a given request.
	ProcessMessage(ctx context.Context, r model.Output) error
	// Update metrics
	UpdateMetrics(model.Output)
}
