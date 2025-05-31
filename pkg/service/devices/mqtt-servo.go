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
	mutex             sync.RWMutex
	onActive          func()
	topicPrefix       string
	mqttClientID      string
	mqttBrokerAddress string

	states        map[string]pwmState
	client        mqttapi.Client
	stateTopics   map[model.DeviceIndex]string
	commandTopics map[model.DeviceIndex]string
}

type pwmState struct {
	OnValue, OffValue uint32
	Enabled           bool
	Payload           string
	LastPayloadSent   time.Time
}

const (
	pwmPayloadExpiration = time.Minute * 15 // Should be 15s
)

// Returns true when the payload has either changed or is expired.
func (s pwmState) UpdateNeeded(payload string) bool {
	return s.Payload != payload || time.Since(s.LastPayloadSent) > pwmPayloadExpiration
}

// newMQTTServo creates a virtual MQTT servo device with given config.
func newMQTTServo(log zerolog.Logger, id model.DeviceID, onActive func(), moduleID, topicPrefix, mqttBrokerAddress string) (PWM, error) {
	servo := &mqttServo{
		log: log.With().
			Str("device_id", string(id)).
			Logger(),
		onActive:          onActive,
		topicPrefix:       topicPrefix,
		mqttClientID:      fmt.Sprintf("%s-%s", moduleID, id),
		mqttBrokerAddress: mqttBrokerAddress,
		states:            make(map[string]pwmState),
		stateTopics:       make(map[model.DeviceIndex]string),
		commandTopics:     make(map[model.DeviceIndex]string),
	}
	return servo, nil
}

// Configure is called once to put the device in the desired state.
func (d *mqttServo) Configure(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Prepare MQTT client options
	opts := defaultMQTTClientOptions(d.mqttBrokerAddress, d.mqttClientID)

	// Connect client
	d.client = mqttapi.NewClient(opts)
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

	if token := d.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to mqtt: %w", token.Error())
	}

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

// Check if a publish of given payload for given state key is needed.
// Mutex MUST NOT be locked when calling this method.
func (d *mqttServo) isPublishNeeded(stateKey, payload string) bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.states[stateKey].UpdateNeeded(payload)
}

// SetPWM the output at given index (1...) to the given value
func (d *mqttServo) SetPWM(ctx context.Context, pin model.DeviceIndex, onValue, offValue uint32, enabled bool) error {
	d.mutex.RLock()
	topic, topicFound := d.commandTopics[pin]
	d.mutex.RUnlock()

	level := (200.0 * (float64(offValue) / float64(d.MaxPWMValue()))) - 100.0
	if topicFound {
		stateKey := strings.TrimSuffix(strings.TrimPrefix(topic, d.topicPrefix), "/command")
		payload := strconv.FormatFloat(level, 'f', 3, 32)
		if d.isPublishNeeded(stateKey, payload) {
			ts := time.Now()
			retain := true
			d.log.Debug().
				Str("topic", topic).
				Str("payload", payload).
				Int("pin", int(pin)).
				Msg("Publishing payload to MQTT command...")
			token := d.client.Publish(topic, 0, retain, payload)
			if !token.WaitTimeout(mqttPublishTimeout) {
				d.log.Error().Err(token.Error()).
					Str("topic", topic).
					Str("payload", payload).
					Int("pin", int(pin)).
					Msg("failed to deliver MQTT command in time")
			} else {
				d.mutex.Lock()
				defer d.mutex.Unlock()
				d.states[stateKey] = pwmState{
					OnValue:         onValue,
					OffValue:        offValue,
					Enabled:         enabled,
					Payload:         payload,
					LastPayloadSent: ts,
				}
				d.log.Debug().
					Str("topic", topic).
					Str("payload", payload).
					Int("pin", int(pin)).
					Msg("Deliver payload to MQTT command in time")
			}
		}
	} else {
		d.log.Debug().
			Int("pin", int(pin)).
			Int("command_topics", len(d.commandTopics)).
			Int("state_topics", len(d.stateTopics)).
			Msg("Command Topic for MQTT servo message pin not found")
	}

	return nil
}

// GetPWM the output at given index (1...)
// Returns onValue,offValue,enabled,error
func (d *mqttServo) GetPWM(ctx context.Context, pin model.DeviceIndex) (uint32, uint32, bool, error) {
	topic := d.stateTopics[pin]
	stateKey := strings.TrimSuffix(strings.TrimPrefix(topic, d.topicPrefix), "/state")
	if state, ok := d.states[stateKey]; ok {
		return state.OnValue, state.OffValue, state.Enabled, nil
	}
	return 0, 0, false, fmt.Errorf("no state found for pin %d", pin)
}

// Sets the state topic to use for the pin of the device with given index
// Empty topics are ignored.
func (d *mqttServo) SetStateTopic(index model.DeviceIndex, topic string) error {
	if topic == "" {
		return nil
	}
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if index < 1 || int(index) > d.PWMPinCount() {
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

// Sets the command topic to use for the pin of the device with given index
// Empty topics are ignored.
func (d *mqttServo) SetCommandTopic(index model.DeviceIndex, topic string) error {
	if topic == "" {
		return nil
	}
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if index < 1 || int(index) > d.PWMPinCount() {
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
