package model

import "github.com/pkg/errors"

// Object holds the base info for each type of real world object.
type Object struct {
	// Unique ID of the object
	ID string `json:"id"`
	// Type of object
	Type ObjectType `json:"type"`
	// Connection pins used by this object
	// The keys used in this map are specific to the type of object.
	Pins map[string][]Pin `json:"pins,omitempty"`
}

// ObjectType identifies a type of real world objects.
type ObjectType string

const (
	ObjectTypeBinaryFeedback ObjectType = "binary-feedback"
	ObjectTypeBinaryOutput   ObjectType = "binary-output"
	ObjectTypeServoSwitch    ObjectType = "servo-switch"
)

// ObjectTypeInfo holds builtin information for a type of objects.
type ObjectTypeInfo struct {
	Type     ObjectType
	PinNames []string
}

const (
	PinNameFeedback = "feedback"
	PinNameOutput   = "output"
	PinNameServo    = "servo"
)

var (
	objectTypeInfos = []ObjectTypeInfo{
		ObjectTypeInfo{
			Type:     ObjectTypeBinaryFeedback,
			PinNames: []string{PinNameFeedback},
		},
		ObjectTypeInfo{
			Type:     ObjectTypeBinaryOutput,
			PinNames: []string{PinNameOutput},
		},
		ObjectTypeInfo{
			Type:     ObjectTypeServoSwitch,
			PinNames: []string{PinNameServo},
		},
	}
)

// Validate the given type, returning nil on ok,
// or an error upon validation issues.
func (t ObjectType) Validate() error {
	for _, typeInfo := range objectTypeInfos {
		if typeInfo.Type == t {
			return nil
		}
	}
	return errors.Wrapf(ValidationError, "invalid object type '%s'", string(t))
}

// Validate the given configuration, returning nil on ok,
// or an error upon validation issues.
func (o Object) Validate() error {
	if o.ID == "" {
		return errors.Wrap(ValidationError, "ID is empty")
	}
	if err := o.Type.Validate(); err != nil {
		return errors.Wrapf(ValidationError, "Error in Type of '%s': %s", o.ID, err.Error())
	}
	// TODO validate pins
	return nil
}
