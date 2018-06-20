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
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	discoveryAPI "github.com/binkynet/BinkyNet/discovery"
)

// Run the worker until the given context is cancelled.
func (s *service) registerWorker(ctx context.Context, hostID string, discoveryPort, httpPort int, httpSecure bool) error {
	intfs, err := net.Interfaces()
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to get network interfaces")
		return maskAny(err)
	}
	wg := sync.WaitGroup{}
	errors := make(chan error, len(intfs))
	for _, intf := range intfs {
		flagMask := net.FlagUp | net.FlagBroadcast | net.FlagLoopback
		flagValue := net.FlagUp | net.FlagBroadcast
		if intf.Flags&flagMask == flagValue {
			addrs, err := intf.Addrs()
			if err != nil {
				s.Log.Error().Err(err).Str("interface", intf.Name).Msg("Failed to get interfaces addresses")
				continue
			}
			if localAddr := firstIPv4(addrs); localAddr != nil {
				s.Log.Info().
					Str("interface", intf.Name).
					Str("address", localAddr.String()).
					Msg("Performing registration on interface")
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := s.registerWorkerOnLocalAddr(ctx, hostID, localAddr, discoveryPort, httpPort, httpSecure); err != nil {
						errors <- err
					}
				}()
			}
		}
	}
	wg.Wait()
	select {
	case err := <-errors:
		return maskAny(err)
	default:
		return nil
	}
}

// Run the worker until the given context is cancelled.
func (s *service) registerWorkerOnLocalAddr(ctx context.Context, hostID string, localAddr net.IP, discoveryPort, httpPort int, httpSecure bool) error {
	broadcastIP := net.IPv4(255, 255, 255, 255)
	localUDPAddr := &net.UDPAddr{
		IP: localAddr,
	}
	socket, err := net.DialUDP("udp4", localUDPAddr, &net.UDPAddr{
		IP:   broadcastIP,
		Port: discoveryPort,
	})
	if err != nil {
		s.Log.Debug().Err(err).Msg("Failed to dial discovery endpoint")
		return maskAny(err)
	}
	defer socket.Close()

	msg := discoveryAPI.RegisterWorkerMessage{
		ID:     hostID,
		Port:   httpPort,
		Secure: httpSecure,
	}
	encodedMsg, err := json.Marshal(msg)
	if err != nil {
		return maskAny(err)
	}
	for {
		if _, err := socket.Write(encodedMsg); err != nil {
			s.Log.Error().Err(err).Msg("Failed to send register worker message")
		}
		select {
		case <-time.After(time.Second):
			// Retry
		case <-ctx.Done():
			// Context cancelled
			return nil
		}
	}
}

// create a host ID based on network hardware addresses.
func createHostID() (string, error) {
	if content, err := ioutil.ReadFile("/etc/machine-id"); err == nil {
		content = []byte(strings.TrimSpace(string(content)))
		id := fmt.Sprintf("%x", sha1.Sum(content))
		return id[:10], nil
	}

	ifs, err := net.Interfaces()
	if err != nil {
		return "", maskAny(err)
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
