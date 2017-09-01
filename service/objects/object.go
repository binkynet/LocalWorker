package objects

import "context"

// Object contains the API supported by all types of objects.
type Object interface {
	// Configure is called once to put the object in the desired state.
	Configure(ctx context.Context) error
}
