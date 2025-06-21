package main

import "encoding/json"

// helper to return indented JSON or the raw bytes on error
func prettyJSON(raw json.RawMessage) string {
	var obj interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return string(raw)
	}
	b, _ := json.MarshalIndent(obj, "", "  ")
	return string(b)
}
