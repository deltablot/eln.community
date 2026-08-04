package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
)

const api = "/api/v1"

var (
	ErrInvalidPath      = errors.New("invalid path")
	ErrMissingID        = errors.New("missing id")
	ErrInvalidCommentID = errors.New("invalid comment id")
)

type pathConfig struct {
	prefix   string
	suffix   string
	resource string
}

type pathParams struct {
	recordID          string
	commentID         int64
	isModerationRoute bool
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
		errorLogger.Printf("%s for %s in result path: method %q, path %q", ErrMissingID, config.resource, r.Method, r.URL.Path)
		http.Error(w, ErrMissingID.Error(), http.StatusBadRequest)
		return "", ErrMissingID
	}
	return result, nil
}

func requireCommentPathParams(w http.ResponseWriter, r *http.Request) (pathParams, error) {
	params := pathParams{
		recordID: r.PathValue("recordID"),
	}
	commentIDStr := r.PathValue("commentID")
	if commentIDStr == "" {
		commentIDStr = r.PathValue("id")
		params.isModerationRoute = true
	}
	commentID, err := strconv.ParseInt(commentIDStr, 10, 64)
	if err != nil {
		errorLogger.Printf("%s for %q: %v", ErrInvalidCommentID, commentIDStr, err)
		http.Error(w, ErrInvalidCommentID.Error(), http.StatusBadRequest)
		return pathParams{}, ErrInvalidCommentID
	}
	params.commentID = commentID
	return params, nil
}
