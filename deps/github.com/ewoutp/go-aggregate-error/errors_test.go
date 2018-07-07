package errors

import (
	"errors"
	"testing"
)

func TestEmpty(t *testing.T) {
	var ae AggregateError
	if !ae.IsEmpty() {
		t.Error("Must be empty")
	}
}

func TestAsErrorEmpty(t *testing.T) {
	var ae AggregateError
	if ae.AsError() != nil {
		t.Error("Must be nil")
	}
}

func TestAsError1(t *testing.T) {
	var ae AggregateError
	err := errors.New("Error1")
	ae.Add(err)
	if ae.AsError() != err {
		t.Error("Must be equal to err")
	}
}

func TestAsError2(t *testing.T) {
	var ae AggregateError
	ae.Add(errors.New("Error1"))
	ae.Add(errors.New("Error2"))
	if ae.AsError() != &ae {
		t.Error("Must be equal to ae")
	}
}

func TestAdd(t *testing.T) {
	var ae AggregateError
	ae.Add(errors.New("Error1"))
	if ae.IsEmpty() {
		t.Error("Must not be empty")
	}
}

func TestAddNil(t *testing.T) {
	var ae AggregateError
	ae.Add(nil)
	if !ae.IsEmpty() {
		t.Error("Must be empty")
	}
}

func TestError1(t *testing.T) {
	var ae AggregateError
	ae.Add(errors.New("Error1"))
	str := ae.Error()
	expected := "Error1"
	if str != expected {
		t.Errorf("Expected %s, got %s", expected, str)
	}
}

func TestError2(t *testing.T) {
	var ae AggregateError
	ae.Add(errors.New("Error1"))
	ae.Add(errors.New("Error2"))
	str := ae.Error()
	expected := "Error1, Error2"
	if str != expected {
		t.Errorf("Expected %s, got %s", expected, str)
	}
}
