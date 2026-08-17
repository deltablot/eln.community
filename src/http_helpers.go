package main

import (
	"encoding/json"
	"errors"
	"net/http"
)

var (
	ErrInvalidRequest  = errors.New("invalid request body")
	ErrInvalidResponse = errors.New("failed to encode JSON response")
)

func requireJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		errorLogger.Printf("%s: %v", ErrInvalidRequest, err)
		http.Error(w, ErrInvalidRequest.Error(), http.StatusBadRequest)
		return ErrInvalidRequest
	}
	return nil
}

func writeJson(w http.ResponseWriter, statusCode int, data any) {
	payload, err := json.Marshal(data)
	if err != nil {
		errorLogger.Printf("failed to marshal JSON response: %w", err)
		http.Error(w, ErrInvalidResponse.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if _, err := w.Write(payload); err != nil {
		errorLogger.Printf("failed to write JSON response: %w", err)
	}
}
