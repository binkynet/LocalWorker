// Copyright 2022 Ewout Prangsma
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
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	api "github.com/binkynet/BinkyNet/apis/v1"
)

type Server interface {
	// Run the HTTP server until the given context is cancelled.
	Run(ctx context.Context) error
}

// Service ('s) that we offer
type Service interface {
	api.LocalWorkerServiceServer
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
	api.RegisterLocalWorkerServiceServer(grpcSrv, s.api)
	// Register reflection service on gRPC server.
	reflection.Register(grpcSrv)

	nctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g, nctx := errgroup.WithContext(nctx)
	g.Go(func() error {
		if err := grpcSrv.Serve(grpcLis); err != nil {
			s.log.Warn().Err(err).Msg("failed to serve GRPC")
			return err
		}
		return nil
	})
	g.Go(func() error {
		return api.RegisterServiceEntry(nctx, api.ServiceTypeLocalWorker, api.ServiceInfo{
			ApiVersion: "v1",
			ApiPort:    int32(s.GRPCPort),
			Secure:     false,
		})
	})
	g.Go(func() error {
		// Wait for content cancellation
		select {
		case <-ctx.Done():
			// Stop
		case <-nctx.Done():
			// Stop
		}
		// Close server
		s.log.Debug().Msg("Closing server...")
		grpcSrv.GracefulStop()
		cancel()
		return nil
	})
	if err := g.Wait(); err != nil {
		s.log.Debug().Err(err).Msg("Wait failed")
		return err
	}
	return nil
}
