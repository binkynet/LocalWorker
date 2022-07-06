// Copyright 2021 Ewout Prangsma
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

package util

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// UntilCanceled continues to call the given callback
// until the given context is canceled
func UntilCanceled(ctx context.Context, log zerolog.Logger, description string, cb func() error) error {
	delay := time.Millisecond * 10
	for {
		if ctx.Err() != nil {
			// Context canceled
			return nil
		}
		if err := cb(); err != nil {
			log.Warn().Err(err).Msgf("%s failed", description)
			delay = time.Duration(float64(delay) * 1.5)
			if delay > time.Second*5 {
				delay = time.Second * 5
			}
		} else {
			delay = time.Millisecond * 10
		}
		select {
		case <-ctx.Done():
			// Context canceled
			log.Info().Msgf("Stopping %s; context canceled", description)
			return nil
		case <-time.After(delay):
			// Continue
		}
	}

}
