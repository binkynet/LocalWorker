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

	"github.com/mattn/go-pubsub"
	"github.com/rs/zerolog"

	api "github.com/binkynet/BinkyNet/apis/v1"
	model "github.com/binkynet/BinkyNet/apis/v1"
)

// RequestService is used by object types to receive requests from the network master.
type requestService struct {
	log            zerolog.Logger
	outputRequests *pubsub.PubSub
	switchRequests *pubsub.PubSub
}

// newRequestService creates a new RequestService.
func newRequestService(log zerolog.Logger) *requestService {
	return &requestService{
		log:            log,
		outputRequests: pubsub.New(),
		switchRequests: pubsub.New(),
	}
}

// Set the given output state
func (s *requestService) SetOutput(ctx context.Context, req *model.Output) error {
	s.outputRequests.Pub(*req)
	return nil
}

// Set the given switch state
func (s *requestService) SetSwitch(ctx context.Context, req *model.Switch) error {
	s.switchRequests.Pub(*req)
	return nil
}

func (s *requestService) RegisterOutputRequestReceiver(cb func(api.Output) error) context.CancelFunc {
	wcb := func(x api.Output) {
		if err := cb(x); err != nil {
			s.log.Warn().Err(err).Msg("Output processing error")
		}
	}
	s.outputRequests.Sub(wcb)
	return func() {
		s.outputRequests.Leave(wcb)
	}
}

func (s *requestService) RegisterSwitchRequestReceiver(cb func(api.Switch) error) context.CancelFunc {
	wcb := func(x api.Switch) {
		if err := cb(x); err != nil {
			s.log.Warn().Err(err).Msg("Switch processing error")
		}
	}
	s.switchRequests.Sub(wcb)
	return func() {
		s.switchRequests.Leave(wcb)
	}
}
