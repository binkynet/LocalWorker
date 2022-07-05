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

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	api "github.com/binkynet/BinkyNet/apis/v1"
	utils "github.com/binkynet/LocalWorker/service/util"
)

// StatusService is used by object types to report their status to the network master.
type statusService struct {
	log           zerolog.Logger
	outputActuals chan api.Output
	sensorActuals chan api.Sensor
	switchActuals chan api.Switch
}

// newStatusService creates a new StatusService.
func newStatusService(log zerolog.Logger) *statusService {
	return &statusService{
		log:           log,
		outputActuals: make(chan api.Output, 8),
		sensorActuals: make(chan api.Sensor, 8),
		switchActuals: make(chan api.Switch, 8),
	}
}

// Run the service until the given context is canceled
func (s *statusService) Run(ctx context.Context, nwControlClient api.NetworkControlServiceClient) error {
	log := s.log
	g, ctx := errgroup.WithContext(ctx)
	// Send output actuals
	g.Go(func() error {
		once := func() error {
			for {
				select {
				case msg := <-s.outputActuals:
					if _, err := nwControlClient.SetOutputActual(ctx, &msg); err != nil {
						log.Debug().Err(err).Msg("Send(Output) failed")
						return err
					}
				case <-ctx.Done():
					return nil
				}
			}
		}
		return utils.UntilCanceled(ctx, log, "sendOutputActuals", once)
	})
	// Send sensor actuals
	g.Go(func() error {
		once := func() error {
			for {
				select {
				case msg := <-s.sensorActuals:
					if _, err := nwControlClient.SetSensorActual(ctx, &msg); err != nil {
						log.Debug().Err(err).Msg("Send(Sensor) failed")
						return err
					}
				case <-ctx.Done():
					return nil
				}
			}
		}
		return utils.UntilCanceled(ctx, log, "sendSensorActuals", once)
	})
	// Send switch actuals
	g.Go(func() error {
		once := func() error {
			for {
				select {
				case msg := <-s.switchActuals:
					if _, err := nwControlClient.SetSwitchActual(ctx, &msg); err != nil {
						log.Debug().Err(err).Msg("Send(Switch) failed")
						return err
					}
				case <-ctx.Done():
					return nil
				}
			}
		}
		return utils.UntilCanceled(ctx, log, "sendSwitchActuals", once)
	})
	return g.Wait()
}

func (s *statusService) PublishOutputActual(msg api.Output) {
	s.outputActuals <- msg
}

func (s *statusService) PublishSensorActual(msg api.Sensor) {
	s.sensorActuals <- msg
}

func (s *statusService) PublishSwitchActual(msg api.Switch) {
	s.switchActuals <- msg
}
