//    Copyright 2017-2022 Ewout Prangsma
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
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "github.com/binkynet/BinkyNet/apis/v1"
)

// Reset the local worker
func (s *service) Reset(context.Context, *api.Empty) (*api.Empty, error) {
	log := s.Logger
	go func() {
		log.Warn().Msg("About to reset process")
		time.Sleep(time.Second * 5)
		// Do actual exit
		log.Warn().Msg("Resetting process")
		os.Exit(1)
	}()
	return &api.Empty{}, nil
}

// Set the requested power state
func (s *service) SetPowerRequest(ctx context.Context, msg *api.PowerState) (*api.Empty, error) {
	setPowerRequestTotal.Inc()
	log := s.Logger
	s.mutex.Lock()
	grs := s.getRequestService
	s.mutex.Unlock()

	var err error
	if grs != nil {
		if rs := grs.GetRequestService(); rs != nil {
			err = rs.SetPowerRequest(ctx, msg)
		} else {
			err = fmt.Errorf("request service not available")
			log.Error().Msg("request service not available in SetPowerRequest")
		}
	} else {
		err = fmt.Errorf("get request service not available")
		log.Error().Msg("get request service not available in SetPowerRequest")
	}
	return &api.Empty{}, err
}

// Set the requested loc state
func (s *service) SetLocRequest(context.Context, *api.Loc) (*api.Empty, error) {
	setLocRequestTotal.Inc()
	return nil, status.Errorf(codes.Unimplemented, "not implemented")
}

// Set the requested output state
func (s *service) SetOutputRequest(ctx context.Context, msg *api.Output) (*api.Empty, error) {
	setOutputRequestTotal.WithLabelValues(string(msg.GetAddress())).Inc()
	log := s.Logger
	s.mutex.Lock()
	grs := s.getRequestService
	s.mutex.Unlock()

	var err error
	if grs != nil {
		if rs := grs.GetRequestService(); rs != nil {
			err = rs.SetOutputRequest(ctx, msg)
		} else {
			err = fmt.Errorf("request service not available")
			log.Error().Msg("request service not available in SetOutputRequest")
		}
	} else {
		err = fmt.Errorf("get request service not available")
		log.Error().Msg("get request service not available in SetOutputRequest")
	}
	return &api.Empty{}, err
}

// Set the requested switch state
func (s *service) SetSwitchRequest(ctx context.Context, msg *api.Switch) (*api.Empty, error) {
	setSwitchRequestTotal.WithLabelValues(string(msg.GetAddress())).Inc()
	log := s.Logger
	s.mutex.Lock()
	grs := s.getRequestService
	s.mutex.Unlock()

	var err error
	if grs != nil {
		if rs := grs.GetRequestService(); rs != nil {
			err = rs.SetSwitchRequest(ctx, msg)
		} else {
			err = fmt.Errorf("request service not available")
			log.Error().Msg("request service not available in SetSwitchRequest")
		}
	} else {
		err = fmt.Errorf("get request service not available")
		log.Error().Msg("get request service not available in SetSwitchRequest")
	}
	return &api.Empty{}, err
}
