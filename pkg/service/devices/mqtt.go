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

type mqtt struct {
	log               zerolog.Logger
	mutex             sync.Mutex
	onActive          func()
	config            model.Device
	topicPrefix       string
	mqttClientID      string
	mqttBrokerAddress string

	states    map[string]string
	direction []PinDirection
	client    mqttapi.Client
}

const (
	mqttPinCount       = 256
	mqttPublishTimeout = time.Millisecond * 200
)

// newMQTT creates a virtual MQTT device with given config.
func newMQTT(log zerolog.Logger, config model.Device, onActive func(), moduleID, mqttBrokerAddress string) (GPIO, error) {
	if config.Type != model.DeviceTypeMQTT {
		return nil, model.InvalidArgument("Invalid device type '%s'", string(config.Type))
	}
	topicPrefix := strings.TrimSuffix(config.Address, "/") + "/"
	return &mqtt{
		log:               log,
		onActive:          onActive,
		config:            config,
		topicPrefix:       topicPrefix,
		mqttClientID:      fmt.Sprintf("%s-%s", moduleID, config.Id),
		mqttBrokerAddress: mqttBrokerAddress,
		states:            make(map[string]string),
		direction:         make([]PinDirection, mqttPinCount),
	}, nil
}

// Configure is called once to put the device in the desired state.
func (d *mqtt) Configure(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Prepare MQTT client options
	opts := mqttapi.NewClientOptions().
		AddBroker("tcp://" + d.mqttBrokerAddress).
		SetClientID(d.mqttClientID)
	opts.SetKeepAlive(2 * time.Second)
	opts.SetPingTimeout(1 * time.Second)
	opts.SetOrderMatters(false)
	opts.SetDefaultPublishHandler(func(c mqttapi.Client, m mqttapi.Message) {
		// Ignore messages when no subscription match
	})

	// Connect client
	d.client = mqttapi.NewClient(opts)
	if token := d.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to mqtt: %w", token.Error())
	}
	if token := d.client.Subscribe(d.topicPrefix+"#", 0, d.onMessage); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to '%s': %w", d.topicPrefix+"#", token.Error())
	}

	d.onActive()
	return nil
}

// Close brings the device back to a safe state.
func (d *mqtt) Close(ctx context.Context) error {
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
func (d *mqtt) onMessage(client mqttapi.Client, msg mqttapi.Message) {
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
func (d *mqtt) PinCount() uint {
	return mqttPinCount
}

// Set the direction of the pin at given index (1...)
func (d *mqtt) SetDirection(ctx context.Context, index model.DeviceIndex, direction PinDirection) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.direction[index-1] = direction
	return nil
}

// Get the direction of the pin at given index (1...)
func (d *mqtt) GetDirection(ctx context.Context, index model.DeviceIndex) (PinDirection, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.direction[index-1], nil
}

// Set the pin at given index (1...) to the given value
func (d *mqtt) Set(ctx context.Context, pin model.DeviceIndex, value bool) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.direction[pin-1] == PinDirectionInput {
		// Setting inputs is not supported
		return nil
	}

	topic := fmt.Sprintf("%spin%d/command", d.topicPrefix, pin)
	payload := formatBool(value)
	token := d.client.Publish(topic, 0, false, payload)
	if !token.WaitTimeout(mqttPublishTimeout) {
		d.log.Error().Err(token.Error()).
			Str("topic", topic).
			Str("payload", payload).
			Msg("failed to deliver MQTT command in time")
	}

	return nil
}

// Set the pin at given index (1...)
func (d *mqtt) Get(ctx context.Context, pin model.DeviceIndex) (bool, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	state := d.states[fmt.Sprintf("pin%d", pin)]
	result, _ := parseBool(state)
	return result, nil
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
