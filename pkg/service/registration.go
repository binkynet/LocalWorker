//    Copyright 2017 Ewout Prangsma
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

package service

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"net"
	"runtime"
	"sort"
	"strings"
)

// create a host ID based on network hardware addresses.
func createHostID() (string, error) {
	if content, err := ioutil.ReadFile("/etc/machine-id"); err == nil {
		content = []byte(strings.TrimSpace(string(content)))
		id := fmt.Sprintf("%x", sha1.Sum(content))
		return id[:10], nil
	}

	ifs, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	list := make([]string, 0, len(ifs))
	for _, v := range ifs {
		f := v.Flags
		if f&net.FlagUp != 0 && f&net.FlagLoopback == 0 {
			fmt.Printf("Using intf %s with addr %s\n", v.Name, v.HardwareAddr.String())
			h := v.HardwareAddr.String()
			if len(h) > 0 {
				list = append(list, h)
			}
		}
	}
	sort.Strings(list) // sort host IDs
	list = append(list, runtime.GOOS, runtime.GOARCH)
	data := []byte(strings.Join(list, ","))
	id := fmt.Sprintf("%x", sha1.Sum(data))
	return id[:10], nil
}

func firstIPv4(addrs []net.Addr) net.IP {
	for _, x := range addrs {
		if ipn, ok := x.(*net.IPNet); ok {
			if result := ipn.IP.To4(); result != nil {
				return result
			}
		}
	}
	return nil
}
