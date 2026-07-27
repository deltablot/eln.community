package main

import (
	"strings"
    "unicode/utf8"
    "errors"
    "fmt"
)

const commentMaxLength = 5000

/*
func requireValidCommentContent(w http.ResponseWriter, r *http.Request, source string, content string) (string, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		errorLogger.Printf("%s: empty comment content: method %q, path %q", source, r.Method, r.URL.Path)
		http.Error(w, "Comment content cannot be empty", http.StatusBadRequest)
		return "", false
	}

	if utf8.RuneCountInString(content) > commentMaxLength {
		errorLogger.Printf("%s comment content too long: method %q, path %q, length %d ", source, r.Method, r.URL.Path, len(content))
		http.Error(w, fmt.Sprintf("Comment content too long (max %d characters)", commentMaxLength), http.StatusBadRequest)
		return "", false
	}
	return content, true
}
*/

func enforceLength(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", errors.New("content cannot be empty")
	}

	if utf8.RuneCountInString(content) > commentMaxLength {
        return "", errors.New(fmt.Sprintf("comment content too long, got %d characters, but max is %d characters.", utf8.RuneCountInString(content), commentMaxLength))
	}
	return content, nil
}
