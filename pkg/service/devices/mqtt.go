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
	"time"

	model "github.com/binkynet/BinkyNet/apis/v1"
	mqttapi "github.com/eclipse/paho.mqtt.golang"
)

// Device is of type MQTT
type MQTT interface {
	// Sets the state topic to use for the pin of the device with given index
	SetStateTopic(index model.DeviceIndex, topic string) error
	// Sets the command topic to use for the pin of the device with given index
	SetCommandTopic(index model.DeviceIndex, topic string) error
}

// Create default options for MQTT clients
func defaultMQTTClientOptions(mqttBrokerAddress, mqttClientID string) *mqttapi.ClientOptions {
	// Prepare MQTT client options
	return mqttapi.NewClientOptions().
		AddBroker("tcp://" + mqttBrokerAddress).
		SetClientID(mqttClientID).
		SetKeepAlive(15 * time.Second).
		SetPingTimeout(1 * time.Second).
		SetOrderMatters(false).
		SetDefaultPublishHandler(func(c mqttapi.Client, m mqttapi.Message) {
			// Ignore messages when no subscription match
		}).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(time.Second * 5).
		SetMaxReconnectInterval(time.Hour)
}
