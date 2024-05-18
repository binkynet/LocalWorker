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
	"net/http/pprof"
	"strconv"

	api "github.com/binkynet/BinkyNet/apis/v1"
	"github.com/binkynet/BinkyNet/sshlog"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Config for the HTTP server.
type Config struct {
	// Host interface to listen on
	Host string
	// Port to listen on for HTTP requests
	HTTPPort int
	// Port to listen on for SSH requests
	SSHPort int
	// Port to listen on for GRPC requests
	GRPCPort int
}

// Server runs the HTTP server for the service.
type Server struct {
	Config
	log       zerolog.Logger
	ui        UI
	service   Service
	sshLogger *sshlog.SSHLogger
}

type UI interface {
	// You can wire any Bubble Tea model up to the middleware with a function that
	// handles the incoming ssh.Session. Here we just grab the terminal info and
	// pass it to the new model. You can also return tea.ProgramOptions (such as
	// tea.WithAltScreen) on a session by session basis.
	Handler(s ssh.Session) (tea.Model, []tea.ProgramOption)
}

// Implementation of the GRPC service.
type Service interface {
	api.LocalWorkerServiceServer
}

// New configures a new Server.
func New(cfg Config, log zerolog.Logger, sshLogger *sshlog.SSHLogger, ui UI, service Service) (*Server, error) {
	return &Server{
		Config:    cfg,
		log:       log,
		ui:        ui,
		service:   service,
		sshLogger: sshLogger,
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
	httpRouter.GET("/debug/pprof/*", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	httpSrv := http.Server{
		Handler: httpRouter,
	}

	// Prepare GRPC listener
	grpcAddr := net.JoinHostPort(s.Host, strconv.Itoa(s.GRPCPort))
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to listen on address %s", grpcAddr)
	}

	// Prepare GRPC server
	grpcSrv := grpc.NewServer(
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	)
	api.RegisterLocalWorkerServiceServer(grpcSrv, s.service)
	// Register reflection service on gRPC server.
	reflection.Register(grpcSrv)

	// Prepare SSH server
	sshAddr := net.JoinHostPort(s.Host, strconv.Itoa(s.SSHPort))
	sshServer, err := wish.NewServer(
		// The address the server will listen to.
		wish.WithAddress(sshAddr),

		// The SSH server need its own keys, this will create a keypair in the
		// given path if it doesn't exist yet.
		// By default, it will create an ED25519 key.
		wish.WithHostKeyPath(".ssh/id_ed25519"),

		// Middlewares do something on a ssh.Session, and then call the next
		// middleware in the stack.
		wish.WithMiddleware(
			bubbletea.Middleware(s.sshLogger.TeaHandler),
			// The last item in the chain is the first to be called.
			activeterm.Middleware(),
			logging.Middleware(),
		),
	)
	if err != nil {
		return fmt.Errorf("could not start SSH server: %w", err)
	}

	// Serve apis
	log.Debug().Str("address", httpAddr).Msg("Serving HTTP")
	go func() {
		if err := httpSrv.Serve(httpLis); err != nil {
			log.Fatal().Err(err).Msg("failed to serve HTTP server")
		}
		log.Debug().Str("address", httpAddr).Msg("Done Serving HTTP")
	}()
	log.Debug().Str("address", grpcAddr).Msg("Serving GRPC")
	go func() {
		if err := grpcSrv.Serve(grpcLis); err != nil {
			log.Fatal().Err(err).Msg("failed to serve GRPC server")
		}
		log.Debug().Str("address", httpAddr).Msg("Done Serving GRPC")
	}()
	// Serve UI
	log.Debug().Str("address", sshAddr).Msg("Serving SSH")
	go func() {
		if err = sshServer.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Msg("failed to serve SSH server")
		}
		log.Debug().Str("address", httpAddr).Msg("Done Serving SSH")
	}()

	// Wait until context closed
	<-ctx.Done()

	log.Info().Msg("Closing servers")
	httpSrv.Shutdown(context.Background())
	grpcSrv.GracefulStop()
	sshServer.Shutdown(context.Background())

	return nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "OK")
}
