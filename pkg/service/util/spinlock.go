// Copyright 2024 Ewout Prangsma
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author Ewout Prangsma
//

package util

import (
	"runtime"
	"sync/atomic"
)

// Spinlock with exponential backoff.
type SpinLock struct {
	flags uint32
}

// Lock the spinlock.
func (l *SpinLock) Lock() {
	backoff := 1
	for {
		if l.TryLock() {
			// We're locked
			return
		}
		// Backoff
		for x := 0; x < backoff; x++ {
			runtime.Gosched()
		}
		if backoff < 64 {
			backoff *= 2
		}
	}
}

// Try to lock the spinlock.
// Returns true when locked, false otherwise.
func (l *SpinLock) TryLock() bool {
	return atomic.CompareAndSwapUint32(&l.flags, 0, 1)
}

// Unlock the spinlock.
func (l *SpinLock) Unlock() {
	atomic.StoreUint32(&l.flags, 0)
}
