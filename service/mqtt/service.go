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

	"github.com/binkynet/BinkyNet/model"
	"github.com/rs/zerolog"
)

type Config struct {
	Host     string
	Port     int
	UserName string
	Password string
}

// Service contains the API exposed by the MQTT service.
type Service interface {
	// Close the service
	Close() error
	// RequestConfiguration sends a request to ask for the configuration of this worker.
	RequestConfiguration(ctx context.Context) (model.LocalConfiguration, error)
}

// NewService instantiates a new MQTT service.
func NewService(config Config, logger zerolog.Logger) (Service, error) {
	return &service{}, nil
}

type service struct {
}

// Close the service
func (s *service) Close() error {
	return nil
}

// RequestConfiguration sends a request to ask for the configuration of this worker.
// The given callback is called when the configuration is received.
func (s *service) RequestConfiguration(ctx context.Context) (model.LocalConfiguration, error) {
	// TODO
	return model.LocalConfiguration{}, nil
}
