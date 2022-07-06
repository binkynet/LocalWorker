// Copyright 2020 Ewout Prangsma
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
	"context"

	api "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/rs/zerolog"
)

// Object contains the API supported by all types of objects.
type Object interface {
	// Return the type of this object.
	Type() ObjectType
	// Configure is called once to put the object in the desired state.
	Configure(ctx context.Context) error
	// Run the object until the given context is cancelled.
	Run(ctx context.Context, requests RequestService, statuses StatusService, moduleID string) error
	// ProcessPowerMessage acts upons a given power message.
	ProcessPowerMessage(ctx context.Context, m api.PowerState) error
}

// ObjectType contains the API supported a specific type of object.
// There will be a single instances of a specific ObjecType that is used by all Object instances.
type ObjectType interface {
	// Return the name of this object type
	String() string
	// Run the object until the given context is cancelled.
	Run(ctx context.Context, log zerolog.Logger, requests RequestService, statuses StatusService, service Service, moduleID string) error
}

// RequestService is used by object types to receive requests from the network master.
type RequestService interface {
	RegisterOutputRequestReceiver(func(api.Output) error) context.CancelFunc
	RegisterSwitchRequestReceiver(func(api.Switch) error) context.CancelFunc
}

// StatusService is used by object types to report their status to the network master.
type StatusService interface {
	PublishOutputActual(api.Output)
	PublishSensorActual(api.Sensor)
	PublishSwitchActual(api.Switch)
}
