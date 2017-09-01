package netmanager

import (
	"github.com/pkg/errors"
	restkit "github.com/pulcy/rest-kit"
)

var (
	maskAny = errors.WithStack
)

func init() {
	restkit.WithStack = errors.WithStack
	restkit.Cause = errors.Cause
}
