package devices

import "context"

// GPIO contains the API that is supported by all general purpose I/O devices.
type GPIO interface {
	Device
	// PinCount returns the number of pins of the device
	PinCount() int
	// Set the direction of the pin at given index (1...)
	SetDirection(ctx context.Context, pin int, direction PinDirection) error
	// Get the direction of the pin at given index (1...)
	GetDirection(ctx context.Context, pin int) (PinDirection, error)
	// Set the pin at given index (1...) to the given value
	Set(ctx context.Context, pin int, value bool) error
	// Set the pin at given index (1...)
	Get(ctx context.Context, pin int) (bool, error)
}

type PinDirection byte

const (
	PinDirectionInput PinDirection = iota
	PinDirectionOutput
)
