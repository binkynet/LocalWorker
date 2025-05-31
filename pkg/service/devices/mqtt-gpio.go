// Copyright 2024 Ewout Prangsma
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
	"sync"
	"time"

	mqttapi "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog"

	model "github.com/binkynet/BinkyNet/apis/v1"
)

type mqttGPIO struct {
	log               zerolog.Logger
	mutex             sync.Mutex
	onActive          func()
	topicPrefix       string
	mqttClientID      string
	mqttBrokerAddress string

	states        map[string]string
	direction     []PinDirection
	client        mqttapi.Client
	stateTopics   map[model.DeviceIndex]string
	commandTopics map[model.DeviceIndex]string
}

const (
	mqttPinCount       = 256
	mqttPublishTimeout = time.Millisecond * 200
)

// newMQTTGPIO creates a virtual MQTT gpio device with given config.
func newMQTTGPIO(log zerolog.Logger, id model.DeviceID, onActive func(), moduleID, topicPrefix, mqttBrokerAddress string) (GPIO, error) {
	//	topicPrefix := strings.TrimSuffix(config.Address, "/") + "/"
	gpio := &mqttGPIO{
		log:               log,
		onActive:          onActive,
		topicPrefix:       topicPrefix,
		mqttClientID:      fmt.Sprintf("%s-%s", moduleID, id),
		mqttBrokerAddress: mqttBrokerAddress,
		states:            make(map[string]string),
		direction:         make([]PinDirection, mqttPinCount),
		stateTopics:       make(map[model.DeviceIndex]string),
		commandTopics:     make(map[model.DeviceIndex]string),
	}
	for pin := model.DeviceIndex(1); uint(pin) <= gpio.PinCount(); pin++ {
		gpio.stateTopics[pin] = fmt.Sprintf("%spin%d/state", topicPrefix, pin)
		gpio.commandTopics[pin] = fmt.Sprintf("%spin%d/command", topicPrefix, pin)
	}
	return gpio, nil
}

// Configure is called once to put the device in the desired state.
func (d *mqttGPIO) Configure(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Prepare MQTT client options
	opts := defaultMQTTClientOptions(d.mqttBrokerAddress, d.mqttClientID)
	opts.SetOnConnectHandler(func(c mqttapi.Client) {
		d.log.Debug().Msg("Connected to MQTT")
		topic := d.topicPrefix + "#"
		if token := d.client.Subscribe(topic, 0, d.onMessage); token.Wait() && token.Error() != nil {
			d.log.Error().Err(token.Error()).
				Msgf("failed to subscribe to '%s'", topic)
			c.Disconnect(500)
		} else {
			d.log.Debug().Msgf("Subscribed to MQTT topic '%s'", topic)
			d.onActive()
		}
	})

	// Connect client
	d.client = mqttapi.NewClient(opts)
	if token := d.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to mqtt: %w", token.Error())
	}

	return nil
}

// Close brings the device back to a safe state.
func (d *mqttGPIO) Close(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.client != nil {
		d.client.Disconnect(250)
		d.client = nil
	}

	// Restore all to defaults
	d.onActive()
	return nil
}

// Receive messages
func (d *mqttGPIO) onMessage(client mqttapi.Client, msg mqttapi.Message) {
	topic := strings.TrimPrefix(msg.Topic(), d.topicPrefix)
	if strings.HasSuffix(topic, "/state") {
		// Valid state message
		topic = strings.TrimSuffix(topic, "/state")
	} else {
		// Not a valid message
		return
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.states[topic] = string(msg.Payload())
}

// PinCount returns the number of pins of the device
func (d *mqttGPIO) PinCount() uint {
	return mqttPinCount
}

// Set the direction of the pin at given index (1...)
func (d *mqttGPIO) SetDirection(ctx context.Context, index model.DeviceIndex, direction PinDirection) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.direction[index-1] = direction
	return nil
}

// Get the direction of the pin at given index (1...)
func (d *mqttGPIO) GetDirection(ctx context.Context, index model.DeviceIndex) (PinDirection, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.direction[index-1], nil
}

// Set the pin at given index (1...) to the given value
func (d *mqttGPIO) Set(ctx context.Context, pin model.DeviceIndex, value bool) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.direction[pin-1] == PinDirectionInput {
		// Setting inputs is not supported
		return nil
	}

	topic := d.commandTopics[pin]
	payload := formatBool(value)
	retain := true
	token := d.client.Publish(topic, 0, retain, payload)
	if !token.WaitTimeout(mqttPublishTimeout) {
		d.log.Error().Err(token.Error()).
			Str("topic", topic).
			Str("payload", payload).
			Msg("failed to deliver MQTT command in time")
	}

	return nil
}

// Set the pin at given index (1...)
func (d *mqttGPIO) Get(ctx context.Context, pin model.DeviceIndex) (bool, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	topic := d.stateTopics[pin]
	stateKey := strings.TrimSuffix(strings.TrimPrefix(topic, d.topicPrefix), "/state")
	state := d.states[stateKey]
	result, _ := parseBool(state)
	return result, nil
}

// Sets the state topic to use for the pin of the device with given index
// Empty topics are ignored.
func (d *mqttGPIO) SetStateTopic(index model.DeviceIndex, topic string) error {
	if topic == "" {
		return nil
	}
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if index < 1 || uint(index) > d.PinCount() {
		return fmt.Errorf("invalid index %d", index)
	}
	if !strings.HasPrefix(topic, d.topicPrefix) {
		return fmt.Errorf("topic '%s' is missing prefix '%s' at index %d", topic, d.topicPrefix, index)
	}
	if !strings.HasSuffix(topic, "/state") {
		return fmt.Errorf("topic '%s' is missing suffix '/state' at index %d", topic, index)
	}
	d.stateTopics[index] = topic
	return nil
}

// Sets the command topic to use for the pin of the device with given index.
// Empty topics are ignored.
func (d *mqttGPIO) SetCommandTopic(index model.DeviceIndex, topic string) error {
	if topic == "" {
		return nil
	}
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if index < 1 || uint(index) > d.PinCount() {
		return fmt.Errorf("invalid index %d", index)
	}
	if !strings.HasPrefix(topic, d.topicPrefix) {
		return fmt.Errorf("topic '%s' is missing prefix '%s' at index %d", topic, d.topicPrefix, index)
	}
	if !strings.HasSuffix(topic, "/command") {
		return fmt.Errorf("topic '%s' is missing suffix '/command' at index %d", topic, index)
	}
	d.commandTopics[index] = topic
	return nil
}

// Parse a string into a bool
func parseBool(str string) (bool, error) {
	str = strings.ToLower(str)
	switch str {
	case "1", "t", "true", "on", "yes":
		return true, nil
	case "0", "f", "false", "off", "no":
		return false, nil
	}
	return false, fmt.Errorf("invalid bool value '%s'", str)
}

// format a bool as string
func formatBool(v bool) string {
	if v {
		return "ON"
	}
	return "OFF"
}
