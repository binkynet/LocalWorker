package devices

import "context"

// Device contains the API that is supported by all types of devices.
type Device interface {
	// Configure is called once to put the device in the desired state.
	Configure(ctx context.Context) error
	// Close brings the device back to a safe state.
	Close() error
}
