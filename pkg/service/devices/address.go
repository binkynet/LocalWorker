// Copyright 2020 Ewout Prangsma
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

package devices

import (
	"strconv"
	"strings"
)

// parseAddress parses a string containing a numeric address.
func parseAddress(addr string) (int, error) {
	if strings.HasPrefix(addr, "0x") || strings.HasPrefix(addr, "0X") {
		addr = addr[2:]
		result, err := strconv.ParseUint(addr, 16, 32)
		if err != nil {
			return 0, err
		}
		return int(result), nil
	}
	result, err := strconv.ParseUint(addr, 10, 32)
	if err != nil {
		return 0, err
	}
	return int(result), nil
}
