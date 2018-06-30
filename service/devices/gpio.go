package devices

import (
	"context"

	"github.com/binkynet/BinkyNet/model"
)

// GPIO contains the API that is supported by all general purpose I/O devices.
type GPIO interface {
	Device
	// PinCount returns the number of pins of the device
	PinCount() uint
	// Set the direction of the pin at given index (1...)
	SetDirection(ctx context.Context, index model.DeviceIndex, direction PinDirection) error
	// Get the direction of the pin at given index (1...)
	GetDirection(ctx context.Context, index model.DeviceIndex) (PinDirection, error)
	// Set the pin at given index (1...) to the given value
	Set(ctx context.Context, index model.DeviceIndex, value bool) error
	// Set the pin at given index (1...)
	Get(ctx context.Context, index model.DeviceIndex) (bool, error)
}

type PinDirection byte

const (
	PinDirectionInput PinDirection = iota
	PinDirectionOutput
)
