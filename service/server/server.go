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

package server

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"strconv"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Server interface {
	// Run the HTTP server until the given context is cancelled.
	Run(ctx context.Context) error
}

// Service ('s) that we offer
type Service interface {
	//api.LogProviderServiceServer
}

type Config struct {
	Host     string
	GRPCPort int
}

func (c Config) createTLSConfig() (*tls.Config, error) {
	return nil, nil
}

// NewServer creates a new server
func NewServer(conf Config, api Service, log zerolog.Logger) (Server, error) {
	return &server{
		Config:     conf,
		log:        log.With().Str("component", "server").Logger(),
		requestLog: log.With().Str("component", "server.requests").Logger(),
		api:        api,
	}, nil
}

type server struct {
	Config
	log        zerolog.Logger
	requestLog zerolog.Logger
	api        Service
}

// Run the HTTP server until the given context is cancelled.
func (s *server) Run(ctx context.Context) error {
	// Create TLS config
	/*tlsConfig, err := s.Config.createTLSConfig()
	if err != nil {
		return err
	}*/

	// Prepare GRPC listener
	grpcAddr := net.JoinHostPort(s.Host, strconv.Itoa(s.GRPCPort))
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen on address %s: %v", grpcAddr, err)
	}

	// Prepare GRPC server
	grpcSrv := grpc.NewServer(
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	)
	//api.RegisterLogProviderServiceServer(grpcSrv, s.api)
	// Register reflection service on gRPC server.
	reflection.Register(grpcSrv)

	go func() {
		if err := grpcSrv.Serve(grpcLis); err != nil {
			log.Fatalf("failed to serve GRPC: %v", err)
		}
	}()
	select {
	case <-ctx.Done():
		// Close server
		s.log.Debug().Msg("Closing server...")
		grpcSrv.GracefulStop()
		return nil
	}
}
