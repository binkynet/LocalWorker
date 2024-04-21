package intf

import (
	"context"

	api "github.com/binkynet/BinkyNet/apis/v1"
)

type RequestService interface {
	// Set the requested power state
	SetPowerRequest(context.Context, *api.PowerState) error
	// Set the requested output state
	SetOutputRequest(context.Context, *api.Output) error
	// Set the requested switch state
	SetSwitchRequest(context.Context, *api.Switch) error
	// Set the requested device discovery state
	SetDeviceDiscoveryRequest(context.Context, *api.DeviceDiscovery) error
}

type GetRequestService interface {
	GetRequestService() RequestService
}
