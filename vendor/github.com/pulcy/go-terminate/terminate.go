// Copyright (c) 2016 Pulcy.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package terminate

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

type Terminator struct {
	ExitCode    int
	OsExitDelay time.Duration

	signalCounter uint32
	logger        func(template string, args ...interface{})
	onClosing     func()
}

// NewTerminator creates and initialized a new Terminator.
// Call ListenSignals on the returned instances to perform its magic.
func NewTerminator(logger func(template string, args ...interface{}), onClosing func()) *Terminator {
	return &Terminator{
		ExitCode:    0,
		OsExitDelay: time.Second * 3,

		signalCounter: 0,
		logger:        logger,
		onClosing:     onClosing,
	}
}

// close closes this service in a timely manor.
func (t *Terminator) close() {
	// Interrupt the process when closing is requested twice.
	if atomic.AddUint32(&t.signalCounter, 1) >= 2 {
		t.exitProcess()
	}

	if t.onClosing != nil {
		t.logger("preparing to shutdown")
		t.onClosing()
	}
	t.logger("shutting down server in %s", t.OsExitDelay.String())
	time.Sleep(t.OsExitDelay)

	t.exitProcess()
}

// exitProcess terminates this process with exit code 1.
func (t *Terminator) exitProcess() {
	t.logger("shutting down server")
	os.Exit(t.ExitCode)
}

// ListenSignals waits for incoming OS signals and acts upon them
func (t *Terminator) ListenSignals() {
	// Set up channel on which to send signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	// Block until a signal is received.
	for {
		select {
		case sig := <-c:
			t.logger("received signal %s", sig)
			go t.close()
		}
	}
}
