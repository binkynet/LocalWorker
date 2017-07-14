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

package service

import (
	"context"

	logging "github.com/op/go-logging"

	"github.com/binkynet/LocalWorker/service/mqtt"
)

type Service interface {
	// Run the worker until the given context is cancelled.
	Run(ctx context.Context) error
}

type ServiceDependencies struct {
	Log      *logging.Logger
	MqttConn mqtt.API
}

type service struct {
	ServiceDependencies
}

// NewService creates a Service instance and returns it.
func NewService(deps ServiceDependencies) (Service, error) {
	return &service{
		ServiceDependencies: deps,
	}, nil
}

// Run the worker until the given context is cancelled.
func (s *service) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
