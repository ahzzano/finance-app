package services

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type Handler interface {
	Mount(*http.ServeMux)
}

// Error types allowed by CONTRACT.md.
const (
	TypeValidation       = "VALIDATION_ERROR"
	TypeAuthentication   = "AUTHENTICATION_ERROR"
	TypeNotFound         = "NOT_FOUND"
	TypeMethodNotAllowed = "METHOD_NOT_ALLOWED"
	TypeConflict         = "CONFLICT"
	TypeRateLimited      = "RATE_LIMITED"
	TypeInternal         = "INTERNAL_ERROR"
)

var statusByType = map[string]int{
	TypeValidation:       http.StatusBadRequest,
	TypeAuthentication:   http.StatusUnauthorized,
	TypeNotFound:         http.StatusNotFound,
	TypeMethodNotAllowed: http.StatusMethodNotAllowed,
	TypeConflict:         http.StatusConflict,
	TypeRateLimited:      http.StatusTooManyRequests,
	TypeInternal:         http.StatusInternalServerError,
}

type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

type ErrorResponse struct {
	Data   any        `json:"data"`
	Errors []APIError `json:"errors"`
}

// ValidationError lets services report a field-specific validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string { return e.Message }

// WriteError writes e in the error envelope with the status mapped from e.Type (500 if unknown).
func WriteError(w http.ResponseWriter, e APIError) {
	status, ok := statusByType[e.Type]
	if !ok {
		status = http.StatusInternalServerError
	}
	WriteJSON(w, status, ErrorResponse{Errors: []APIError{e}})
}

// WriteData writes a success envelope: {"data": data, "errors": null}.
func WriteData(w http.ResponseWriter, status int, data any) {
	WriteJSON(w, status, struct {
		Data   any        `json:"data"`
		Errors []APIError `json:"errors"`
	}{Data: data})
}

// Decode reads a JSON body (capped at 1MB) into v; an empty body leaves v zero-valued; on failure it writes a 400 and returns false.
func Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil && !errors.Is(err, io.EOF) {
		WriteError(w, APIError{Type: TypeValidation, Message: "malformed request body"})
		return false
	}
	return true
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
