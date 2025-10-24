package main

import (
	"encoding/json"
	"fmt"
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

// validateAndNormalizeRorId validates and normalizes a ROR ID
func validateAndNormalizeRorId(rorId string) (string, bool) {
	if rorId == "" {
		return "", true // empty is valid (optional field)
	}

	rorId = strings.TrimSpace(rorId)

	// ROR ID pattern: 0 followed by 6 alphanumeric chars followed by 2 digits
	pattern := `^0[a-z|0-9]{8}$`
	matched, err := regexp.MatchString(pattern, rorId)
	if err != nil {
		fmt.Printf("Regex error: %v\n", err)
		return "", false
	}

	return rorId, matched
}

// validateAndNormalizeRorIds validates and normalizes multiple ROR IDs
func validateAndNormalizeRorIds(rorIds []string) ([]string, error) {
	var normalizedIds []string
	for _, rorId := range rorIds {
		normalized, isValid := validateAndNormalizeRorId(rorId)
		if !isValid {
			return nil, fmt.Errorf("invalid ROR ID format: %s", rorId)
		}
		if normalized != "" {
			normalizedIds = append(normalizedIds, normalized)
		}
	}
	return normalizedIds, nil
}
