// Copyright 2023 Ewout Prangsma
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
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// Config for the HTTP server.
type Config struct {
	// Host interface to listen on
	Host string
	// Port to listen on for HTTP requests
	HTTPPort int
}

// Server runs the HTTP server for the service.
type Server struct {
	Config
	log zerolog.Logger
}

// New configures a new Server.
func New(cfg Config, log zerolog.Logger) (*Server, error) {
	return &Server{
		Config: cfg,
		log:    log,
	}, nil
}

// Run the server until the given context is canceled.
func (s *Server) Run(ctx context.Context) error {
	// Prepare HTTP listener
	log := s.log
	httpAddr := net.JoinHostPort(s.Host, strconv.Itoa(s.HTTPPort))
	httpLis, err := net.Listen("tcp", httpAddr)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to listen on address %s", httpAddr)
	}

	// Prepare HTTP server
	httpRouter := echo.New()
	httpRouter.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	httpSrv := http.Server{
		Handler: httpRouter,
	}

	// Serve apis
	log.Debug().Str("address", httpAddr).Msg("Serving HTTP")
	go func() {
		if err := httpSrv.Serve(httpLis); err != nil {
			log.Fatal().Err(err).Msg("failed to serve HTTP server")
		}
		log.Debug().Str("address", httpAddr).Msg("Done Serving HTTP")
	}()

	// Wait until context closed
	<-ctx.Done()

	log.Info().Msg("Closing GRPC server")
	httpSrv.Shutdown(context.Background())

	return nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "OK")
}
