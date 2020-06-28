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
	"golang.org/x/sync/errgroup"

	"github.com/binkynet/BinkyNet/apis/util"
	api "github.com/binkynet/BinkyNet/apis/v1"
)

// RequestService is used by object types to receive requests from the network master.
type requestService struct {
	log            zerolog.Logger
	outputRequests *pubsub.PubSub
	sensorRequests *pubsub.PubSub
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

// Run the service until the given context is canceled
func (s *requestService) Run(ctx context.Context, lwControlClient api.LocalWorkerControlServiceClient) error {
	log := s.log
	g, ctx := errgroup.WithContext(ctx)
	// Receive output requests
	g.Go(func() error {
		once := func() error {
			server, err := lwControlClient.GetOutputRequests(ctx, &api.OutputRequestsOptions{
				ManualConfirm: true,
			})
			if err != nil {
				return err
			}
			msg, err := server.Recv()
			if util.IsStreamClosed(err) {
				return nil
			} else if err != nil {
				log.Warn().Err(err).Msg("Recv(Output) failed")
			} else {
				s.outputRequests.Pub(*msg)
			}
			return nil
		}
		return untilCanceled(ctx, log, "receiveOutputRequests", once)
	})
	// Receive switch requests
	g.Go(func() error {
		once := func() error {
			server, err := lwControlClient.GetSwitchRequests(ctx, &api.SwitchRequestsOptions{
				ManualConfirm: true,
			})
			if err != nil {
				return err
			}
			msg, err := server.Recv()
			if util.IsStreamClosed(err) {
				return nil
			} else if err != nil {
				log.Warn().Err(err).Msg("Recv(Switch) failed")
			} else {
				s.switchRequests.Pub(*msg)
			}
			return nil
		}
		return untilCanceled(ctx, log, "receiveSwitchRequests", once)
	})
	return g.Wait()
}

func (s *requestService) RegisterOutputRequestReceiver(cb func(api.Output) error) context.CancelFunc {
	s.outputRequests.Sub(cb)
	return func() {
		s.outputRequests.Leave(cb)
	}
}

func (s *requestService) RegisterSwitchRequestReceiver(cb func(api.Switch) error) context.CancelFunc {
	s.switchRequests.Sub(cb)
	return func() {
		s.switchRequests.Leave(cb)
	}
}
