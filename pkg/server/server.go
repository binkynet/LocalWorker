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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	lm "github.com/charmbracelet/wish/logging"
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
	// Port to listen on for SSH requests
	SSHPort int
}

// Server runs the HTTP server for the service.
type Server struct {
	Config
	log zerolog.Logger
	ui  UI
}

type UI interface {
	// You can wire any Bubble Tea model up to the middleware with a function that
	// handles the incoming ssh.Session. Here we just grab the terminal info and
	// pass it to the new model. You can also return tea.ProgramOptions (such as
	// tea.WithAltScreen) on a session by session basis.
	Handler(s ssh.Session) (tea.Model, []tea.ProgramOption)
}

// New configures a new Server.
func New(cfg Config, log zerolog.Logger, ui UI) (*Server, error) {
	return &Server{
		Config: cfg,
		log:    log,
		ui:     ui,
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

	// Prepare SSH server
	sshAddr := net.JoinHostPort(s.Host, strconv.Itoa(s.SSHPort))
	sshSrv, err := wish.NewServer(
		wish.WithAddress(sshAddr),
		//wish.WithHostKeyPath(".ssh/term_info_ed25519"),
		wish.WithMiddleware(
			bm.Middleware(s.ui.Handler),
			lm.Middleware(),
		),
	)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to listen on address %s", sshAddr)
	}

	// Serve apis
	log.Debug().Str("address", httpAddr).Msg("Serving HTTP")
	go func() {
		if err := httpSrv.Serve(httpLis); err != nil {
			log.Fatal().Err(err).Msg("failed to serve HTTP server")
		}
		log.Debug().Str("address", httpAddr).Msg("Done Serving HTTP")
	}()
	// Serve UI
	log.Debug().Str("address", sshAddr).Msg("Serving SSH")
	go func() {
		if err = sshSrv.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Msg("failed to serve SSH server")
		}
		log.Debug().Str("address", httpAddr).Msg("Done Serving SSH")
	}()

	// Wait until context closed
	<-ctx.Done()

	log.Info().Msg("Closing servers")
	httpSrv.Shutdown(context.Background())
	sshSrv.Shutdown(context.Background())

	return nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "OK")
}
