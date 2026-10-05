package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// maxFaultsBodyBytes bounds PUT /admin/faults bodies.
const maxFaultsBodyBytes = 1 << 20 // 1 MiB

// newAdminHandler builds the admin-only mux: GET/PUT/DELETE /admin/faults.
// Callers must serve this on its own *http.Server and listener (see
// cmd/server) -- it must never be reachable from the public mux, and it
// has no authentication of its own (see README).
func newAdminHandler(fs *faultStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/faults", fs.handleGetFaults)
	mux.HandleFunc("PUT /admin/faults", fs.handlePutFaults)
	mux.HandleFunc("DELETE /admin/faults", fs.handleDeleteFaults)
	return mux
}

func (fs *faultStore) handleGetFaults(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, fs.config())
}

// handlePutFaults replaces the whole configuration atomically: the body is
// fully decoded and validated before fs.replace is called, so a rejected
// body never partially applies.
func (fs *faultStore) handlePutFaults(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFaultsBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var cfg faultConfig
	if err := dec.Decode(&cfg); err != nil {
		status, msg := decodeErrorResponse(err)
		writeFaultError(w, status, msg)
		return
	}
	if dec.More() {
		writeFaultError(w, http.StatusBadRequest, "body must contain a single JSON object")
		return
	}

	rules, err := validateRules(cfg.Rules)
	if err != nil {
		writeFaultError(w, http.StatusBadRequest, err.Error())
		return
	}

	fs.replace(rules)
	writeJSON(w, http.StatusOK, fs.config())
}

func (fs *faultStore) handleDeleteFaults(w http.ResponseWriter, _ *http.Request) {
	fs.replace(map[string]faultRule{})
	writeJSON(w, http.StatusOK, fs.config())
}

// decodeErrorResponse maps a JSON decode error to an HTTP status and a
// short message, without leaking encoding/json's internal error shapes
// into the response.
func decodeErrorResponse(err error) (int, string) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return http.StatusRequestEntityTooLarge, "request body too large"
	}
	// DisallowUnknownFields reports unknown fields as a plain error with
	// this fixed prefix; encoding/json has no typed error for it.
	if strings.HasPrefix(err.Error(), "json: unknown field ") {
		return http.StatusBadRequest, err.Error()
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return http.StatusBadRequest, "malformed JSON"
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return http.StatusBadRequest, "invalid value for field \"" + typeErr.Field + "\""
	}
	return http.StatusBadRequest, "invalid request body"
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type faultErrorBody struct {
	Error string `json:"error"`
}

func writeFaultError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, faultErrorBody{Error: msg})
}
