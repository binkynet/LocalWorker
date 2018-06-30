//    Copyright 2018 Ewout Prangsma
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

package environment

import (
	"strings"

	"github.com/rs/zerolog"
	"golang.org/x/sys/unix"
)

// AutoDetectBridgeType detects the default bridge type based on the environment.
func AutoDetectBridgeType(log zerolog.Logger) string {
	var name unix.Utsname
	if err := unix.Uname(&name); err != nil {
		// Fallback to RPI
		return "rpi"
	}
	release := strings.TrimSpace(string(name.Release[:]))
	if strings.Contains(release, "sunxi") {
		return "opz"
	}
	return "rpi"
}
