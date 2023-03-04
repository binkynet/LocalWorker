//    Copyright 2022 Ewout Prangsma
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
	"fmt"

	api "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/BinkyNet/loki"

	"github.com/rs/zerolog"
)

type lokiLogger struct {
	logger *loki.LokiLogger
}

type LokiLogger interface {
	zerolog.LevelWriter
	Run(ctx context.Context, log zerolog.Logger, job string, lokiChanges chan api.ServiceInfo, timeOffsetChanges chan int64)
}

func NewLokiLogger() LokiLogger {
	return &lokiLogger{}
}

var _ LokiLogger = &lokiLogger{}

func (ll *lokiLogger) Write(p []byte) (n int, err error) {
	return ll.WriteLevel(zerolog.InfoLevel, p)
}

func (ll *lokiLogger) WriteLevel(level zerolog.Level, p []byte) (n int, err error) {
	if logger := ll.logger; logger != nil {
		return logger.WriteLevel(level, p)
	}
	return len(p), nil
}

func (ll *lokiLogger) Run(ctx context.Context, log zerolog.Logger, job string, lokiChanges chan api.ServiceInfo, timeOffsetChanges chan int64) {
	currentURL := ""
	var lastLogger *loki.LokiLogger
	timeOffset := int64(0)
	for {
		select {
		case offset := <-timeOffsetChanges:
			timeOffset = offset
			if lastLogger != nil {
				lastLogger.SetTimeoffset(offset)
			}
		case info := <-lokiChanges:
			if info.ApiAddress != "" {
				url := fmt.Sprintf("http://%s:%d", info.ApiAddress, info.ApiPort)
				if url != currentURL {
					currentURL = url
					logger, err := loki.NewLokiLogger(url, job, timeOffset)
					if err != nil {
						log.Error().Err(err).Msg("NewLokiLogger failed")
						continue
					}
					lastLogger = logger
					ll.logger = logger
					log.Info().
						Str("url", url).
						Str("job", job).
						Msg("Got New Loki logger")
				}
			}
			// TODO
		case <-ctx.Done():
			// Context canceled
			return
		}
	}
}
