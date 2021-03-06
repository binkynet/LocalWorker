//    Copyright 2021 Ewout Prangsma
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package service

import (
	"context"
	"time"

	"github.com/binkynet/BinkyNet/apis/util"
	api "github.com/binkynet/BinkyNet/apis/v1"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/rs/zerolog"
)

// runLoadConfig keeps requesting the worker configuration and puts
// config changes in configChanged channel.
func (s *service) runLoadConfig(ctx context.Context,
	log zerolog.Logger,
	lwConfigClient api.LocalWorkerConfigServiceClient,
	lwControlClient api.LocalWorkerControlServiceClient,
	configChanged chan *api.LocalWorkerConfig,
	stopWorker chan struct{}) error {

	// Prepare log
	log = log.With().Str("component", "config-reader").Logger()

	loadConfigStream := func(log zerolog.Logger) error {
		uptime := int64(time.Since(s.startedAt).Seconds())
		confStream, err := lwConfigClient.GetConfig(ctx, &api.LocalWorkerInfo{
			Id:          s.hostID,
			Description: "Local worker",
			Version:     s.ProgramVersion,
			Uptime:      uptime,
		}, grpc_retry.WithMax(3))
		if err != nil {
			log.Debug().Err(err).Msg("GetConfig failed.")
			return err
		}
		defer confStream.CloseSend()
		for {
			// Read configuration
			conf, err := confStream.Recv()
			if util.IsStreamClosed(err) || ctx.Err() != nil {
				return nil
			} else if err != nil {
				log.Error().Err(err).Msg("Failed to read configuration")
				return nil
			}
			log.Debug().Msg("Received new configuration")
			select {
			case configChanged <- conf:
				// Continue
			case <-ctx.Done():
				// Context canceled
				return nil
			}
		}
	}

	// Keep requesting configuration in stream
	recentErrors := 0
	for {
		delay := time.Second * 5
		if err := loadConfigStream(log); err != nil {
			log.Warn().Err(err).Msg("loadConfigStream failed")
			recentErrors++
			delay = time.Second
		} else {
			recentErrors = 0
		}
		if recentErrors > 10 {
			// Too many recent errors, stop the worker
			log.Debug().Msg("Stopping worker because of too many recent errors")
			stopWorker <- struct{}{}
			return nil
		}
		select {
		case <-ctx.Done():
			// Context canceled
			return nil
		case <-time.After(delay):
			// Retry
		}
	}
}
