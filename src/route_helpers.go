package main

import (
	"net/http"
	"strings"
)

func requireRecordIdFromCommentPath(w http.ResponseWriter, r *http.Request, source string) (string, bool) {
    const prefix = "/api/v1/records/"
    const suffix = "/comments"

    path := r.URL.Path
    if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
        errorLogger.Printf("%s: invalid comment path: method %q, path %q", source, r.Method, r.URL.Path)
        http.Error(w, "invalid record path", http.StatusBadRequest)
        return "", false
    }

    recordId := strings.TrimPrefix(path, prefix)
    recordId = strings.TrimSuffix(recordId, suffix)
    if recordId == "" {
        errorLogger.Printf("%s: missing record id in comment path: method %q, path %q", source, r.Method, r.URL.Path)
        http.Error(w, "missing record id", http.StatusBadRequest)
        return "", false
    }

    return recordId, true
}
