// Copyright 2025 Ewout Prangsma
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

package devices

import (
	"context"
	"fmt"
	"strings"

	mqttapi "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog"

	model "github.com/binkynet/BinkyNet/apis/v1"
)

type mqttStatusMonitor struct {
	log               zerolog.Logger
	onActive          func()
	onStatusChange    func(Status)
	topicPrefix       string
	mqttClientID      string
	mqttBrokerAddress string

	client     mqttapi.Client
	lastStatus Status
}

// newMQTTStatusMonitor creates an MQTT status monitor.
func newMQTTStatusMonitor(log zerolog.Logger, id model.DeviceID, onActive func(), onStatusChange func(Status), moduleID, topicPrefix, mqttBrokerAddress string) (*mqttStatusMonitor, error) {
	servo := &mqttStatusMonitor{
		log: log.With().
			Str("device_id", string(id)).
			Logger(),
		onActive:          onActive,
		onStatusChange:    onStatusChange,
		topicPrefix:       topicPrefix,
		mqttClientID:      fmt.Sprintf("%s-%s", moduleID, id),
		mqttBrokerAddress: mqttBrokerAddress,
	}
	return servo, nil
}

// Configure is called once to put the device in the desired state.
func (d *mqttStatusMonitor) Configure(ctx context.Context) error {
	// Prepare MQTT client options
	topic := strings.TrimSuffix(d.topicPrefix, "/") + "/status"
	opts := defaultMQTTClientOptions(d.mqttBrokerAddress, d.mqttClientID)

	// Prepare logger
	log := d.log.With().Str("topic", topic).Logger()

	// Connect client
	opts.SetOnConnectHandler(func(c mqttapi.Client) {
		log.Debug().Msg("Connected to MQTT")
		if token := d.client.Subscribe(topic, 0, d.onMessage); token.Wait() && token.Error() != nil {
			log.Error().Err(token.Error()).
				Msgf("failed to subscribe to '%s'", topic)
			c.Disconnect(500)
		} else {
			log.Debug().Msgf("Subscribed to MQTT topic '%s'", topic)
			d.onActive()
		}
	})

	log.Debug().Msg("Connecting to MQTT...")
	d.client = mqttapi.NewClient(opts)
	if token := d.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to mqtt: %w", token.Error())
	}

	return nil
}

// Close brings the device back to a safe state.
func (d *mqttStatusMonitor) Close(ctx context.Context) error {
	if c := d.client; c != nil {
		d.client = nil
		c.Disconnect(250)
	}

	// Restore all to defaults
	d.onActive()
	return nil
}

// Receive messages
func (d *mqttStatusMonitor) onMessage(client mqttapi.Client, msg mqttapi.Message) {
	topic := msg.Topic()
	payload := strings.TrimSpace(string(msg.Payload()))
	d.log.Debug().
		Str("topic", topic).
		Str("payload", payload).
		Msg("received status message")

	status := StatusUnknown
	switch payload {
	case "online":
		status = StatusOnline
	case "offline":
		status = StatusOffline
	default:
		d.log.Warn().
			Str("payload", payload).
			Msg("Unknown payload in status message")
		return
	}

	if d.lastStatus != status {
		d.lastStatus = status
		d.onStatusChange(status)
	}
}
