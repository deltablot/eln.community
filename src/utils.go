package main

import (
	"encoding/json"
	"regexp"
	"strings"
)

// helper to return indented JSON or the raw bytes on error
func prettyJSON(raw json.RawMessage) string {
	var obj interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return string(raw)
	}
	b, _ := json.MarshalIndent(obj, "", "  ")
	return string(b)
}

// validateAndNormalizeRorId validates and normalizes a ROR ID to full URL format
func validateAndNormalizeRorId(rorId string) (string, bool) {
	if rorId == "" {
		return "", true // empty is valid (optional field)
	}

	rorId = strings.TrimSpace(rorId)

	// ROR ID pattern: 0 followed by 6 alphanumeric chars followed by 2 digits
	rorIdPattern := regexp.MustCompile(`^0[a-z0-9]{6}[0-9]{2}$`)

	// If it's already a full URL, validate it
	if strings.HasPrefix(rorId, "https://ror.org/") {
		idPart := strings.TrimPrefix(rorId, "https://ror.org/")
		if rorIdPattern.MatchString(idPart) {
			return rorId, true
		}
		return "", false
	}

	// If it starts with ror.org/, extract the ID part
	if strings.HasPrefix(rorId, "ror.org/") {
		idPart := strings.TrimPrefix(rorId, "ror.org/")
		if rorIdPattern.MatchString(idPart) {
			return "https://ror.org/" + idPart, true
		}
		return "", false
	}

	// If it's just the ID part, validate and convert to full URL
	if rorIdPattern.MatchString(rorId) {
		return "https://ror.org/" + rorId, true
	}

	return "", false
}
