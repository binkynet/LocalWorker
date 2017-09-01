//    Copyright 2017 Ewout Prangsma
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

package mqtt

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"sync"

	"github.com/rs/zerolog"
	"github.com/yosssi/gmq/mqtt"
	"github.com/yosssi/gmq/mqtt/client"
)

const (
	// QosAtMostOnce represents "QoS 0: At most once delivery".
	QosAtMostOnce = mqtt.QoS0
	// QosAsLeastOnce represents "QoS 1: At least once delivery".
	QosAsLeastOnce = mqtt.QoS1
	//QosExactlyOnce represents "QoS 2: Exactly once delivery".
	QosExactlyOnce = mqtt.QoS2
)

type Config struct {
	Host     string
	Port     int
	UserName string
	Password string
	ClientID string
}

// Service contains the API exposed by the MQTT service.
type Service interface {
	// Close the service
	Close() error
	// Publish a JSON encoded message into a topic.
	Publish(ctx context.Context, msg interface{}, topic string, qos byte) error
}

// NewService instantiates a new MQTT service.
func NewService(config Config, logger zerolog.Logger) (Service, error) {
	// Create an MQTT Client.
	cli := client.New(&client.Options{
		// Define the processing of the error handler.
		ErrorHandler: func(err error) {
			logger.Error().Err(err).Msg("MQTT error")
		},
	})

	return &service{
		Config: config,
		client: cli,
	}, nil
}

type service struct {
	Config
	mutex     sync.Mutex
	client    *client.Client
	connected bool
}

// Close the service
func (s *service) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.connected {
		s.client.Disconnect()
		s.connected = false
	}

	s.client.Terminate()
	return nil
}

// connect opens a connection.
func (s *service) connect() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.connected {
		return nil
	}
	// Connect to the MQTT Server.
	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	if err := s.client.Connect(&client.ConnectOptions{
		Network:  "tcp",
		Address:  addr,
		ClientID: []byte(s.ClientID),
	}); err != nil {
		return maskAny(err)
	}
	s.connected = true
	return nil
}

// Publish a JSON encoded message into a topic.
func (s *service) Publish(ctx context.Context, msg interface{}, topic string, qos byte) error {
	encodedMsg, err := json.Marshal(msg)
	if err != nil {
		return maskAny(err)
	}
	if err := s.client.Publish(&client.PublishOptions{
		QoS:       qos,
		TopicName: []byte(topic),
		Message:   encodedMsg,
	}); err != nil {
		return maskAny(err)
	}
	return nil
}
