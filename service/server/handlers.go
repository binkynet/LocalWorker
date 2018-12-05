package server

import (
	"net/http"

	discoveryAPI "github.com/binkynet/BinkyNet/discovery"
	"github.com/julienschmidt/httprouter"
)

func (s *server) notFound(w http.ResponseWriter, r *http.Request) {
	s.requestLog.Warn().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("path not found")
	http.NotFound(w, r)
}

func (s *server) handleEnvironment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	var input discoveryAPI.WorkerEnvironment
	if err := parseBody(r, &input); err != nil {
		handleError(w, err)
	} else {
		err := s.api.Environment(ctx, input)
		if err != nil {
			handleError(w, err)
		} else {
			sendJSON(w, http.StatusOK, nil)
		}
	}
}

func (s *server) handleReload(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	err := s.api.Reload(ctx)
	if err != nil {
		handleError(w, err)
	} else {
		sendJSON(w, http.StatusOK, nil)
	}
}

func (s *server) handleShutdown(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	err := s.api.Shutdown(ctx)
	if err != nil {
		handleError(w, err)
	} else {
		sendJSON(w, http.StatusOK, nil)
	}
}

func (s *server) handleEnableMQTTLogging(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	s.api.EnableMQTTLogger(true)
	sendJSON(w, http.StatusOK, nil)
}

func (s *server) handleDisableMQTTLogging(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	s.api.EnableMQTTLogger(false)
	sendJSON(w, http.StatusOK, nil)
}
