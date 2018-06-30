package devices

import "context"

// PWM contains the API that is supported by all pulse width modulation devices.
type PWM interface {
	Device
	// OutputCount returns the number of outputs of the device
	OutputCount() int
	// MaxValue returns the maximum valid value for onValue or offValue.
	MaxValue() int
	// Set the output at given index (1...) to the given value
	Set(ctx context.Context, output int, onValue, offValue int) error
	// Get the output at given index (1...)
	// Returns onValue,offValue,error
	Get(ctx context.Context, output int) (int, int, error)
}
