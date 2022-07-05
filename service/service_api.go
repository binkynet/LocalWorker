//    Copyright 2022 Ewout Prangsma
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
	"io"
	"time"

	api "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/LocalWorker/service/worker"
)

// Gets the features supported by this local worker
func (s *service) GetFeatures(ctx context.Context, req *api.GetFeaturesRequest) (*api.Features, error) {
	return &api.Features{
		Power:    true,
		Locs:     false,
		Outputs:  true,
		Sensors:  true,
		Switches: true,
		Clock:    false,
	}, nil
}

// Describe is used to fetch the a stream if identyable information of a local worker.
func (s *service) Describe(req *api.DescribeRequest, server api.LocalWorkerService_DescribeServer) error {
	log := s.Logger
	ctx := server.Context()
	info := api.LocalWorkerInfo{
		Id:          s.hostID,
		Description: s.hostID,
		Version:     s.ProgramVersion,
	}
	for {
		// Update structure
		info.Uptime = int64(time.Since(s.startedAt).Seconds())
		if w := s.workerRunner.GetWorker(); w != nil {
			info.ConfigHash = w.GetConfigHash()
		} else {
			info.ConfigHash = ""
		}
		// Send info
		if err := server.Send(&info); err != nil {
			log.Info().Err(err).Msg("Describe.Send failed")
			return err
		}
		select {
		case <-time.After(time.Second * 3):
			// Continue
		case <-ctx.Done():
			// Context canceled
			return ctx.Err()
		}
	}
}

// Configure is used to configure a local worker.
func (s *service) Configure(ctx context.Context, req *api.LocalWorkerConfig) (*api.Empty, error) {
	select {
	case s.configChanged <- req:
		// Ok
		return &api.Empty{}, nil
	case <-time.After(time.Second * 30):
		// Timeout
		return nil, fmt.Errorf("Timeout in configure")
	}
}

// DiscoverDevices is used by the netmanager to request a discovery by
// the local worker.
// The local worker in turn responds the found devices.
func (s *service) DiscoverDevices(ctx context.Context, req *api.DiscoverDevicesRequest) (*api.DiscoverDevicesResult, error) {
	worker := s.workerRunner.GetWorker()
	if worker == nil {
		return nil, fmt.Errorf("No worker yet")
	}
	return worker.DiscoverDevices(ctx, req)
}

// SetPowerRequests is used to change the power state of the local worker.
func (s *service) SetPowerRequests(server api.LocalWorkerService_SetPowerRequestsServer) error {
	ctx := server.Context()
	var lastWorker worker.Service
	for {
		msg, err := server.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("SetPowerRequests.Recv failed: %w", err)
		}
		worker := s.workerRunner.GetWorker()
		if worker == nil {
			return fmt.Errorf("No worker yet")
		}
		if lastWorker != nil && lastWorker != worker {
			return fmt.Errorf("Worker changed")
		}
		lastWorker = worker
		if err := worker.SetPower(ctx, msg); err != nil {
			return err
		}
	}
}

// GetPowerActuals is used to fetch a stream of actual power statuses
// from the local workers, to the network master.
func (s *service) GetPowerActuals(req *api.GetPowerActualsRequest, server api.LocalWorkerService_GetPowerActualsServer) error {
	ctx := server.Context()
	for {
		select {
		case msg := <-s.actuals.PowerActuals:
			if err := server.Send(&msg); err != nil {
				s.Logger.Debug().Err(err).Msg("GetPowerActuals.Send failed")
				return err
			}
		case <-ctx.Done():
			// Context canceled
			return nil
		}
	}
}

// SetLocRequests is used to send a stream of loc requests from the network
// master to the local worker.
// Note: Loc.actual field is not set.
func (s *service) SetLocRequests(server api.LocalWorkerService_SetLocRequestsServer) error {
	for {
		_, err := server.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("SetLocRequests.Recv failed: %w", err)
		}
		// Ignore loc messages
	}
}

// GetLocActuals is used to send a stream of actual loc statuses from
// the local worker to the network master.
// Note: Loc.request field must be set to the latest request.
func (s *service) GetLocActuals(req *api.GetLocActualsRequest, server api.LocalWorkerService_GetLocActualsServer) error {
	return nil
}

// GetSensorActuals is used to send a stream of actual sensor statuses
// from the local worker to the network master.
func (s *service) GetSensorActuals(req *api.GetSensorActualsRequest, server api.LocalWorkerService_GetSensorActualsServer) error {
	ctx := server.Context()
	for {
		select {
		case msg := <-s.actuals.SensorActuals:
			if err := server.Send(&msg); err != nil {
				s.Logger.Debug().Err(err).Msg("GetSensorActuals.Send failed")
				return err
			}
		case <-ctx.Done():
			// Context canceled
			return nil
		}
	}
}

// SetOutputRequests is used to get a stream of output requests from the network
// master to the local worker.
func (s *service) SetOutputRequests(server api.LocalWorkerService_SetOutputRequestsServer) error {
	ctx := server.Context()
	var lastWorker worker.Service
	for {
		msg, err := server.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("SetOutputRequests.Recv failed: %w", err)
		}
		worker := s.workerRunner.GetWorker()
		if worker == nil {
			return fmt.Errorf("No worker yet")
		}
		if lastWorker != nil && lastWorker != worker {
			return fmt.Errorf("Worker changed")
		}
		lastWorker = worker
		if err := worker.SetOutput(ctx, msg); err != nil {
			return err
		}
	}
}

// GetOutputActuals is used to send a stream of actual output statuses from
// the local worker to the network master.
func (s *service) GetOutputActuals(req *api.GetOutputActualsRequest, server api.LocalWorkerService_GetOutputActualsServer) error {
	ctx := server.Context()
	for {
		select {
		case msg := <-s.actuals.OutputActuals:
			if err := server.Send(&msg); err != nil {
				s.Logger.Debug().Err(err).Msg("GetOutputActuals.Send failed")
				return err
			}
		case <-ctx.Done():
			// Context canceled
			return nil
		}
	}
}

// SetSwitchRequests is used to get a stream of switch requests from the network
// master to the local worker.
func (s *service) SetSwitchRequests(server api.LocalWorkerService_SetSwitchRequestsServer) error {
	ctx := server.Context()
	var lastWorker worker.Service
	for {
		msg, err := server.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("SetSwitchRequests.Recv failed: %w", err)
		}
		worker := s.workerRunner.GetWorker()
		if worker == nil {
			return fmt.Errorf("No worker yet")
		}
		if lastWorker != nil && lastWorker != worker {
			return fmt.Errorf("Worker changed")
		}
		lastWorker = worker
		if err := worker.SetSwitch(ctx, msg); err != nil {
			return err
		}
	}
}

// GetSwitchActuals is used to send a stream of actual switch statuses from
// the local worker to the network master.
func (s *service) GetSwitchActuals(req *api.GetSwitchActualsRequest, server api.LocalWorkerService_GetSwitchActualsServer) error {
	ctx := server.Context()
	for {
		select {
		case msg := <-s.actuals.SwitchActuals:
			if err := server.Send(&msg); err != nil {
				s.Logger.Debug().Err(err).Msg("GetSwitchActuals.Send failed")
				return err
			}
		case <-ctx.Done():
			// Context canceled
			return nil
		}
	}
}

// SetClock is used to send a stream of current time of day from the network
// master to the local worker.
func (s *service) SetClock(server api.LocalWorkerService_SetClockServer) error {
	// Ignore for now
	for {
		_, err := server.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
	}
}
