// Copyright (c) 2017 Epracom Advies.
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

// +build !no_errgo

package restkit

import (
	"fmt"
	"regexp"
	"testing"
)

func TestErrorLocation(t *testing.T) {
	e := WithStack(fmt.Errorf("foo"))
	msg := fmt.Sprintf("%#v", e)
	expectedPattern := "[{[a-z/]+/errors_juju_errgo_test.go:[0-9]+: } {foo}]"
	if match, err := regexp.MatchString(expectedPattern, msg); err != nil {
		t.Fatalf("Failed to match: %#v", err)
	} else if !match {
		t.Errorf("Expected pattern '%s', got '%s'", expectedPattern, msg)
	}
}
