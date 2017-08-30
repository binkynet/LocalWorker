package model

import (
	"github.com/pkg/errors"
)

var (
	ValidationError = errors.New("validation failed")
	maskAny         = errors.WithStack
)
