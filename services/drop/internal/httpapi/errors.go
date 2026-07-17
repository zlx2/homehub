package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"drop/internal/store"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

type apiError struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
}

func (e *apiError) Error() string { return e.Code }

func writeAPIError(w http.ResponseWriter, err error) {
	var target *apiError
	if !errors.As(err, &target) {
		target = mapStoreError(err)
	}
	writeJSON(w, target.Status, errorBody{Error: errorDetail{
		Code: target.Code, Message: target.Message, Details: detailsOrEmpty(target.Details),
	}})
}

func mapStoreError(err error) *apiError {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return &apiError{Status: http.StatusNotFound, Code: "not_found", Message: "Resource not found"}
	case errors.Is(err, store.ErrQuotaExceeded):
		return &apiError{Status: http.StatusInsufficientStorage, Code: "storage_quota_exceeded", Message: "Storage quota exceeded"}
	case errors.Is(err, store.ErrInvalidInput):
		return &apiError{Status: http.StatusBadRequest, Code: "invalid_request", Message: "Invalid request"}
	case errors.Is(err, store.ErrCodeInvalid):
		return &apiError{Status: http.StatusUnauthorized, Code: "authorization_code_invalid", Message: "Authorization code is invalid or expired"}
	default:
		return &apiError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "Internal server error"}
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, limit int64, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return &apiError{Status: http.StatusBadRequest, Code: "invalid_json", Message: "Request body must be valid JSON"}
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return &apiError{Status: http.StatusBadRequest, Code: "invalid_json", Message: "Request body must contain one JSON value"}
	}
	return nil
}

func detailsOrEmpty(details map[string]any) map[string]any {
	if details == nil {
		return map[string]any{}
	}
	return details
}

func recovery(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("http panic", "method", r.Method, "path", r.URL.Path, "error", recovered)
				writeAPIError(w, &apiError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "Internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
