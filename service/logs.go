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
	"sync"

	api "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/rs/zerolog"
)

// LogWriter is a destination of the zerolog logger, used to send logs to the network
type LogWriter struct {
	mutex   sync.RWMutex
	input   chan logMessage
	outputs []chan logMessage
}

// NewLogWriter creates a new log writer
func NewLogWriter() *LogWriter {
	return &LogWriter{
		input: make(chan logMessage, 64),
	}
}

// logMessage is captured in the LogWriter
type logMessage struct {
	Level zerolog.Level
	Msg   []byte
}

// Write a message
func (w *LogWriter) Write(p []byte) (n int, err error) {
	return w.WriteLevel(zerolog.NoLevel, p)
}

// WriteLevel writes a message at given level
func (w *LogWriter) WriteLevel(l zerolog.Level, p []byte) (n int, err error) {
	return len(p), nil
}

// Run the writer until the given context is canceled
func (w *LogWriter) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Context canceled
			return
		case msg := <-w.input:
			// Message received
			w.mutex.RLock()
			outputs := w.outputs
			w.mutex.RUnlock()
			for _, output := range outputs {
				select {
				case output <- msg:
					// Ok message delivered
				default:
					// Message cannot be delivered
					<-output // Remove message
					output <- msg
				}
			}
		}
	}
}

// Subscribe to messages
func (w *LogWriter) Subscribe() (<-chan logMessage, context.CancelFunc) {
	output := make(chan logMessage, 64)
	w.mutex.Lock()
	defer w.mutex.Unlock()
	w.outputs = append(w.outputs, output)

	cancel := func() {
		w.mutex.Lock()
		defer w.mutex.Unlock()

		for idx, c := range w.outputs {
			if c == output {
				w.outputs = append(w.outputs[:idx], w.outputs[idx+1:]...)
				return
			}
		}
	}

	return output, cancel
}

// Record a log message.
func (s *service) GetLogs(req *api.GetLogsRequest, server api.LogProviderService_GetLogsServer) error {
	messages, cancel := s.LogWriter.Subscribe()
	defer cancel()

	createLevel := func(level zerolog.Level) api.LogLevel {
		switch level {
		case zerolog.DebugLevel:
			return api.LogLevel_DEBUG
		case zerolog.InfoLevel:
			return api.LogLevel_INFO
		case zerolog.WarnLevel:
			return api.LogLevel_WARNING
		case zerolog.ErrorLevel:
			return api.LogLevel_ERROR
		case zerolog.FatalLevel:
			return api.LogLevel_FATAL
		default:
			return api.LogLevel_INFO
		}
	}

	for msg := range messages {
		if err := server.Send(&api.LogEntry{
			Message: string(msg.Msg),
			Level:   createLevel(msg.Level),
		}); err != nil {
			return err
		}
	}

	return nil
}
