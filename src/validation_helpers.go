package main

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

func enforceLength(content string, maxLength int) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", errors.New("content cannot be empty")
	}

	if utf8.RuneCountInString(content) > maxLength {
		return "", errors.New(fmt.Sprintf("comment content too long, got %d characters, but max is %d characters.", utf8.RuneCountInString(content), maxLength))
	}
	return content, nil
}
