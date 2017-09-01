package server

import (
	"context"
	"encoding/json"
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
	ctx := context.Background()
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
