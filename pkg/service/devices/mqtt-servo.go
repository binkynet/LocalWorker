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
	"strconv"
	"strings"
	"sync"
	"time"

	mqttapi "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog"

	model "github.com/binkynet/BinkyNet/apis/v1"
)

type mqttServo struct {
	log               zerolog.Logger
	mutex             sync.Mutex
	onActive          func()
	config            model.Device
	topicPrefix       string
	mqttClientID      string
	mqttBrokerAddress string

	states map[string]pwmState
	client mqttapi.Client
}

type pwmState struct {
	OnValue, OffValue uint32
	Enabled           bool
}

// newMQTTServo creates a virtual MQTT servo device with given config.
func newMQTTServo(log zerolog.Logger, config model.Device, onActive func(), moduleID, mqttBrokerAddress string) (PWM, error) {
	if config.Type != model.DeviceTypeMQTTServo {
		return nil, model.InvalidArgument("Invalid device type '%s'", string(config.Type))
	}
	topicPrefix := strings.TrimSuffix(config.Address, "/") + "/"
	return &mqttServo{
		log:               log,
		onActive:          onActive,
		config:            config,
		topicPrefix:       topicPrefix,
		mqttClientID:      fmt.Sprintf("%s-%s", moduleID, config.Id),
		mqttBrokerAddress: mqttBrokerAddress,
		states:            make(map[string]pwmState),
	}, nil
}

// Configure is called once to put the device in the desired state.
func (d *mqttServo) Configure(ctx context.Context) error {
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
func (d *mqttServo) Close(ctx context.Context) error {
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
func (d *mqttServo) onMessage(client mqttapi.Client, msg mqttapi.Message) {
	topic := strings.TrimPrefix(msg.Topic(), d.topicPrefix)
	if strings.HasSuffix(topic, "/state") {
		// Valid state message
	} else {
		// Not a valid message
		return
	}
	stateKey := strings.TrimSuffix(strings.TrimPrefix(topic, d.topicPrefix), "/state")

	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.states[stateKey] = pwmState{
		OnValue:  0,
		OffValue: 1.0,  // TODO
		Enabled:  true, // TODO
	}
}

// PWMPinCount returns the number of PWM output pins of the device
func (d *mqttServo) PWMPinCount() int {
	return mqttPinCount
}

// MaxPWMValue returns the maximum valid value for onValue or offValue.
func (d *mqttServo) MaxPWMValue() uint32 {
	return 4095
}

// SetPWM the output at given index (1...) to the given value
func (d *mqttServo) SetPWM(ctx context.Context, pin model.DeviceIndex, onValue, offValue uint32, enabled bool) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	level := (200.0 * (float64(offValue) / float64(d.MaxPWMValue()))) - 100.0
	topic := fmt.Sprintf("%spin%d/command", d.topicPrefix, pin)
	payload := strconv.FormatFloat(level, 'f', 3, 32)
	token := d.client.Publish(topic, 0, false, payload)
	if !token.WaitTimeout(mqttPublishTimeout) {
		d.log.Error().Err(token.Error()).
			Str("topic", topic).
			Str("payload", payload).
			Msg("failed to deliver MQTT command in time")
	} else {
		stateKey := strings.TrimSuffix(strings.TrimPrefix(topic, d.topicPrefix), "/command")
		d.states[stateKey] = pwmState{
			OnValue:  onValue,
			OffValue: offValue,
			Enabled:  enabled,
		}
	}

	return nil

}

// GetPWM the output at given index (1...)
// Returns onValue,offValue,enabled,error
func (d *mqttServo) GetPWM(ctx context.Context, pin model.DeviceIndex) (uint32, uint32, bool, error) {
	stateKey := fmt.Sprintf("pin%d", pin)
	if state, ok := d.states[stateKey]; ok {
		return state.OnValue, state.OffValue, state.Enabled, nil
	}
	return 0, 0, false, fmt.Errorf("no state found for pin %d", pin)
}
