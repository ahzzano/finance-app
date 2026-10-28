package services

import (
	"encoding/json"
	"net/http"
)

type Handler interface {
	Mount(*http.ServeMux)
}

type ErrorResponse struct {
	Data   any      `json:"data"`
	Errors []string `json:"errors"`
}

// Decode reads a JSON body (capped at 1MB) into v; on failure it writes a 400 and returns false.
func Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		WriteJSON(w, http.StatusBadRequest, ErrorResponse{Errors: []string{"malformed request body"}})
		return false
	}
	return true
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
