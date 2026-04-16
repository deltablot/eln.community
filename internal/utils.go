package app

import (
	"encoding/json"
	"fmt"
	"log"
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

// sanitizeFilename removes or replaces characters that could be problematic for filesystems
func sanitizeFilename(name string) string {
	problematicChars, err := regexp.Compile(`[<>:"/\\|?*\x00-\x1f]`)
	if err != nil {
		log.Printf("Error in sanitizing filename '%s': %v", name, err)
		return name
	}
	sanitized := problematicChars.ReplaceAllString(name, "_")

	// Trim whitespace and dots from the ends (problematic on Windows)
	sanitized = strings.Trim(sanitized, " .")

	// Ensure the filename isn't empty after sanitization
	if sanitized == "" {
		sanitized = "unnamed"
	}

	// Limit length to avoid filesystem issues (255 chars is common limit, leave room for .eln)
	if len(sanitized) > 250 {
		sanitized = sanitized[:250]
	}

	return sanitized
}
