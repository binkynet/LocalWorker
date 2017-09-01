package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"

	discoveryAPI "github.com/binkynet/BinkyNet/discovery"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
)

type Server interface {
	// Run the HTTP server until the given context is cancelled.
	Run(ctx context.Context) error
}

type API interface {
	// Called to relay environment information.
	Environment(ctx context.Context, input discoveryAPI.WorkerEnvironment) error
	// Called to force a complete reload of the worker.
	Reload(ctx context.Context) error
}

type Config struct {
	Host string
	Port int
}

func (c Config) createTLSConfig() (*tls.Config, error) {
	return nil, nil
}

// NewServer creates a new server
func NewServer(conf Config, api API, log zerolog.Logger) (Server, error) {
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
	api        API
}

// Run the HTTP server until the given context is cancelled.
func (s *server) Run(ctx context.Context) error {
	mux := httprouter.New()
	mux.NotFound = http.HandlerFunc(s.notFound)
	mux.POST("/environment", s.handleEnvironment)
	mux.DELETE("/environment", s.handleReload)

	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	tlsConfig, err := s.Config.createTLSConfig()
	if err != nil {
		return maskAny(err)
	}

	serverErrors := make(chan error)
	go func() {
		defer close(serverErrors)
		if tlsConfig != nil {
			s.log.Info().Msgf("Listening on %s using TLS", addr)
			httpServer.TLSConfig = tlsConfig
			tlsConfig.BuildNameToCertificate()
			if err := httpServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				serverErrors <- maskAny(err)
			}
		} else {
			s.log.Info().Msgf("Listening on %s", addr)
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				serverErrors <- maskAny(err)
			}
		}
	}()

	select {
	case err := <-serverErrors:
		return maskAny(err)
	case <-ctx.Done():
		// Close server
		s.log.Debug().Msg("Closing server...")
		httpServer.Close()
		return nil
	}
}

func handleError(w http.ResponseWriter, err error) {
	writeError(w, http.StatusInternalServerError, err.Error())
}

func writeError(w http.ResponseWriter, status int, message string) {
	if message == "" {
		message = "Unknown error"
	}
	errResp := struct {
		Error string `json:"error"`
	}{
		Error: message,
	}
	b, _ := json.Marshal(errResp)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(b)
}

func parseBody(r *http.Request, data interface{}) error {
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return maskAny(err)
	}
	if err := json.Unmarshal(body, data); err != nil {
		return maskAny(err)
	}
	return nil
}

// sendJSON encodes given body as JSON and sends it to the given writer with given HTTP status.
func sendJSON(w http.ResponseWriter, status int, body interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body == nil {
		w.Write([]byte("{}"))
	} else {
		encoder := json.NewEncoder(w)
		if err := encoder.Encode(body); err != nil {
			return maskAny(err)
		}
	}
	return nil
}
