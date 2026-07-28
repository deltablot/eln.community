package main

import (
	"encoding/json"
	"errors"
	"net/http"
)

var ErrInvalidRequest = errors.New("invalid request body")
var ErrInvalidResponse = errors.New("failed to encode JSON response")

func requireJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		errorLogger.Printf("%s: %v", ErrInvalidRequest, err)
		http.Error(w, ErrInvalidRequest.Error(), http.StatusBadRequest)
		return ErrInvalidRequest
	}
	return nil
}

// TODO: RETURN ERROR
func writeJson(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		errorLogger.Printf("failed to encode JSON response: %v", err)
	}
}
