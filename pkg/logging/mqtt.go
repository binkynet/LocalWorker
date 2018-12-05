// Copyright 2018 Ewout Prangsma
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

package logging

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/binkynet/BinkyNet/mqtt"
)

type MQTTWriter interface {
	io.Writer
	Enable(enable bool)
	SetDestination(topic string, mqttService mqtt.Service)
}

type mqttLogger struct {
	mutex       sync.Mutex
	queue       chan []byte
	topic       string
	mqttService mqtt.Service
	enable      bool
}

const (
	mqttQueueSize = 512
)

// NewMQTTWriter creates a new MQTT output for logs.
// The MQTT sender is closed when the given context is canceled.
func NewMQTTWriter(ctx context.Context) MQTTWriter {
	l := &mqttLogger{
		queue: make(chan []byte, mqttQueueSize),
	}
	go l.run(ctx)
	return l
}

func (l *mqttLogger) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	for attempt := 0; attempt < 10; attempt++ {
		select {
		case l.queue <- p:
			return len(p), nil
		default:
			// Queue full; Take 1 out and try again
			select {
			case <-l.queue:
				// Continue
			default:
				// Also continue
			}
		}
	}
	// Ignore errors
	return len(p), nil
}

func (l *mqttLogger) Enable(enable bool) {
	l.enable = enable
}

func (l *mqttLogger) SetDestination(topic string, mqttService mqtt.Service) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.topic = topic
	l.mqttService = mqttService
}

type logMsg struct {
	Message string `json:"message"`
}

func (l *mqttLogger) run(ctx context.Context) {
	for {
		l.mutex.Lock()
		mqttService := l.mqttService
		topic := l.topic
		enabled := l.enable
		l.mutex.Unlock()

		if enabled && topic != "" && mqttService != nil {
			select {
			case msg := <-l.queue:
				mqttService.Publish(ctx, logMsg{Message: string(msg)}, topic, mqtt.QosDefault)
			case <-ctx.Done():
				return
			}
		} else {
			select {
			case <-time.After(time.Second):
				// Continue
			case <-ctx.Done():
				return
			}
		}
	}
}
