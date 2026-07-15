package main

import (
	"net/http"
	"strings"
)

const api = "/api/v1"

func parsePath(w http.ResponseWriter, r *http.Request, prefix string, suffix string, data string, source string) (string, bool) {
    prefix = api + prefix

    path := r.URL.Path
    if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
        errorLogger.Printf("%s: invalid %s path: method %q, path %q", source, data, r.Method, r.URL.Path)
        http.Error(w, "invalid path", http.StatusBadRequest)
        return "", false
    }
    result := strings.TrimPrefix(path, prefix)
    result = strings.TrimSuffix(result, suffix)
    if result == "" {
        errorLogger.Printf("%s: missing %s id in result path: method %q, path %q", source, data, r.Method, r.URL.Path)
        http.Error(w, "missing id", http.StatusBadRequest)
        return "", false
    }
    return result, true
}
