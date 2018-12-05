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
	"fmt"
	"io"
	"sync"

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
	select {
	case l.queue <- p:
		return len(p), nil
	default:
		return 0, fmt.Errorf("Queue full")
	}
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

func (l *mqttLogger) run(ctx context.Context) {
	for {
		select {
		case msg := <-l.queue:
			if l.enable {
				l.mutex.Lock()
				mqttService := l.mqttService
				topic := l.topic
				l.mutex.Unlock()
				if topic != "" && mqttService != nil {
					mqttService.Publish(ctx, msg, topic, mqtt.QosDefault)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
