//    Copyright 2022 Ewout Prangsma
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package util

import (
	"sync"

	"go.uber.org/multierr"
)

type SyncError struct {
	err   error
	mutex sync.Mutex
}

// Add the given error to the sync error.
func (se *SyncError) Add(err error) {
	se.mutex.Lock()
	defer se.mutex.Unlock()
	multierr.AppendInto(&se.err, err)
}

// Returns the combined error (if any)
func (se *SyncError) AsError() error {
	se.mutex.Lock()
	defer se.mutex.Unlock()
	return se.err
}
