// Package httpx holds the JSON request/response and error conventions shared by all handlers.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

const maxBodyBytes = 1 << 20

// Error is an API error with a stable machine-readable code, rendered as
// {"error": {"code": "...", "message": "..."}}.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func BadRequest(msg string) *Error   { return &Error{http.StatusBadRequest, "bad_request", msg} }
func Unauthorized(msg string) *Error { return &Error{http.StatusUnauthorized, "unauthorized", msg} }
func Forbidden(msg string) *Error    { return &Error{http.StatusForbidden, "forbidden", msg} }
func NotFound(msg string) *Error     { return &Error{http.StatusNotFound, "not_found", msg} }
func Conflict(msg string) *Error     { return &Error{http.StatusConflict, "conflict", msg} }
func TooManyRequests(msg string) *Error {
	return &Error{http.StatusTooManyRequests, "too_many_requests", msg}
}

// HandlerFunc is an http handler that returns an error instead of writing it.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := h(w, r)
	if err == nil {
		return
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		slog.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		apiErr = &Error{http.StatusInternalServerError, "internal", "internal server error"}
	}
	WriteJSON(w, apiErr.Status, map[string]any{
		"error": map[string]string{"code": apiErr.Code, "message": apiErr.Message},
	})
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// DecodeJSON reads a single JSON object into dst, rejecting unknown fields.
func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return BadRequest(fmt.Sprintf("invalid JSON body: %v", err))
	}
	if dec.More() {
		return BadRequest("body must contain a single JSON object")
	}
	return nil
}
