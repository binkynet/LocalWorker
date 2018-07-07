package errors

import (
	"strings"
	"sync"
)

type AggregateError struct {
	list  []error
	mutex sync.Mutex
}

// Add adds given given error to my list of errors and returns this.
// If the given error is nil, nothing changes.
func (this *AggregateError) Add(err error) *AggregateError {
	if err == nil {
		return this
	}
	this.mutex.Lock()
	defer this.mutex.Unlock()
	this.list = append(this.list, err)
	return this
}

// IsEmpty returns true if there are no errors added to me, false otherwise.
func (this *AggregateError) IsEmpty() bool {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	return len(this.list) == 0
}

// AsError returns this instance if errors have been added to this instance, nil otherwise
func (this *AggregateError) AsError() error {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	switch len(this.list) {
	case 0:
		return nil
	case 1:
		return this.list[0]
	default:
		return this
	}
}

// Error returns the combined error messages.
func (this *AggregateError) Error() string {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	if len(this.list) == 0 {
		return ""
	}
	messages := make([]string, 0, len(this.list))
	for _, x := range this.list {
		messages = append(messages, x.Error())
	}
	return strings.Join(messages, ", ")
}
