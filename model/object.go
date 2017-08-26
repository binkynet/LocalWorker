package model

// Object holds the base info for each type of real world object.
type Object struct {
	ID   string
	Type ObjectType
}

type ObjectType string
