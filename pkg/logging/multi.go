// Copyright 2018 Ewout Prangsma
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

package logging

import (
	"io"
)

type multiWriter struct {
	writers []io.Writer
}

// NewMultiWriter creates a new output for logs and can add outputs
// on the fly.
func NewMultiWriter(writers ...io.Writer) io.Writer {
	l := &multiWriter{
		writers: writers,
	}
	return l
}

func (l *multiWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	err = nil
	for _, w := range l.writers {
		n, err = w.Write(p)
	}
	return n, err
}
