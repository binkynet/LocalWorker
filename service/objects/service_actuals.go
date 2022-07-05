// Copyright 2022 Ewout Prangsma
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

package objects

import (
	api "github.com/binkynet/BinkyNet/apis/v1"
)

// Channels where actual status updates are sent to
type ServiceActuals struct {
	OutputActuals chan api.Output
	PowerActuals  chan api.PowerState
	SensorActuals chan api.Sensor
	SwitchActuals chan api.Switch
}

func (s *ServiceActuals) PublishOutputActual(msg api.Output) {
	s.OutputActuals <- msg
}

func (s *ServiceActuals) PublishPowerActual(msg api.PowerState) {
	s.PowerActuals <- msg
}

func (s *ServiceActuals) PublishSensorActual(msg api.Sensor) {
	s.SensorActuals <- msg
}

func (s *ServiceActuals) PublishSwitchActual(msg api.Switch) {
	s.SwitchActuals <- msg
}
