package main

import (
	"net/http"
	"encoding/json"
)

func requireJSONBody(w http.ResponseWriter, r *http.Request, source string, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		errorLogger.Printf("%s: invalid request body: %v", source, err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return false
	}
	return true
}

func writeJson(w http.ResponseWriter, source string, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		errorLogger.Printf("%s: failed to encode JSON response: %v", source, err)
	}
}
