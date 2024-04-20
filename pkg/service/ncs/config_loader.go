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

package ncs

import (
	"context"
	"time"

	"github.com/binkynet/BinkyNet/apis/util"
	api "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/rs/zerolog"
)

// runLoadConfig keeps requesting the worker configuration and puts
// config changes in configChanged channel.
func (ncs *networkControlService) runLoadConfig(ctx context.Context,
	configChanged chan *api.LocalWorkerConfig) {

	// Prepare log
	log := ncs.log.With().Str("component", "config-reader").Logger()

	loadConfigStream := func(log zerolog.Logger) error {
		log.Debug().Msg("Calling WatchLocalWorkers...")
		confStream, err := ncs.nwControlClient.WatchLocalWorkers(ctx, &api.WatchOptions{
			WatchRequestChanges: true,
			ModuleId:            ncs.hostID,
		})
		if err != nil {
			log.Debug().Err(err).Msg("GetConfig failed.")
			return err
		}
		defer confStream.CloseSend()
		var lastConfHash string
		for {
			// Read configuration
			log.Debug().Msg("Calling WatchLocalWorkers.Recv...")
			lw, err := confStream.Recv()
			log.Debug().Err(err).Msg("Called WatchLocalWorkers.Recv.")
			if util.IsStreamClosed(err) || ctx.Err() != nil {
				log.Debug().Err(err).Msg("WatchLocalWorkers stream closed.")
				return nil
			} else if err != nil {
				log.Error().Err(err).Msg("WatchLocalWorkers failed to read configuration")
				return err
			}
			conf := lw.GetRequest()
			hash := conf.GetHash()
			if hash == lastConfHash {
				log.Debug().
					Str("hash", hash).
					Str("lastHash", lastConfHash).
					Msg("Received identical configuration")
			} else {
				log.Debug().
					Str("hash", hash).
					Msg("Received new configuration")
				lastConfHash = hash
				if ut := conf.GetUnixtime(); ut != 0 {
					timeUnix := time.Now().Unix()
					offset := ut - timeUnix
					select {
					case ncs.timeOffsetChanges <- offset:
						// Continue
					case <-ctx.Done():
						// Context canceled
						return nil
					}
				}
				configurationChangesTotal.Inc()
				select {
				case configChanged <- conf:
					// Continue
				case <-ctx.Done():
					// Context canceled
					return nil
				}
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
			return
		}
		select {
		case <-ctx.Done():
			// Context canceled
			return
		case <-time.After(delay):
			// Retry
		}
	}
}
