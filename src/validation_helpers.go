package main

import (
	"fmt"
	"net/http"
	"strings"
)

const commentMaxLength = 500

func requireValidCommentContent(w http.ResponseWriter, r *http.Request, source string, content string) (string, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		errorLogger.Printf("%s: empty comment content: method %q, path %q", source, r.Method, r.URL.Path)
		http.Error(w, "Comment content cannot be empty", http.StatusBadRequest)
		return "", false
	}

	if len(content) > commentMaxLength {
		errorLogger.Printf("%s: comment content too long: method %q, path %q, length %d ", source, r.Method, r.URL.Path, len(content))
		http.Error(w, fmt.Sprintf("Comment content too long (max %d characters)", commentMaxLength), http.StatusBadRequest)
		return "", false
	}
	return content, true
}
