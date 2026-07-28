package main

import (
	"errors"
	"net/http"
	"strings"
)

const api = "/api/v1"

var (
	ErrInvalidPath = errors.New("invalid path")
	ErrMissingId   = errors.New("missing id")
)

type pathConfig struct {
	prefix   string
	suffix   string
	resource string
}

func parsePath(w http.ResponseWriter, r *http.Request, config pathConfig) (string, error) {
	prefix := api + config.prefix

	path := r.URL.Path
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, config.suffix) {
		errorLogger.Printf("%s for %s: method %q, path %q", ErrInvalidPath, config.resource, r.Method, r.URL.Path)
		http.Error(w, ErrInvalidPath.Error(), http.StatusBadRequest)
		return "", ErrInvalidPath
	}
	result := strings.TrimPrefix(path, prefix)
	result = strings.TrimSuffix(result, config.suffix)
	if result == "" {
		errorLogger.Printf("%s for %s in result path: method %q, path %q", ErrMissingId, config.resource, r.Method, r.URL.Path)
		http.Error(w, ErrMissingId.Error(), http.StatusBadRequest)
		return "", ErrMissingId
	}
	return result, nil
}
