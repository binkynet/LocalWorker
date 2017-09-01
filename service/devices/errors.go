package devices

import "github.com/pkg/errors"

var (
	InvalidDirectionError = errors.New("invalid direction")
	IsInvalidDirection    = isErrorFunc(InvalidDirectionError)
	InvalidPinError       = errors.New("invalid pin")
	IsInvalidPin          = isErrorFunc(InvalidPinError)

	maskAny = errors.WithStack
)

func isErrorFunc(typeOfError error) func(err error) bool {
	return func(err error) bool {
		return err == typeOfError || errors.Cause(err) == typeOfError
	}
}
