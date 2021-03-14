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

package util

import (
	"net"
	"strconv"
	"time"

	api "github.com/binkynet/BinkyNet/apis/v1"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// DialConn prepares a connection to given service.
func DialConn(info *api.ServiceInfo) (*grpc.ClientConn, error) {
	address := net.JoinHostPort(info.GetApiAddress(), strconv.Itoa(int(info.GetApiPort())))
	var opts []grpc.DialOption
	if !info.Secure {
		opts = append(opts, grpc.WithInsecure())
	}
	callOpts := []grpc_retry.CallOption{
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(50 * time.Millisecond)),
		grpc_retry.WithMax(3),
		grpc_retry.WithCodes(codes.Unavailable),
	}
	opts = append(opts,
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(callOpts...)),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(callOpts...)),
	)
	return grpc.Dial(address, opts...)
}
