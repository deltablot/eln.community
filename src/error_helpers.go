package main

import (
	"fmt"
)

// TODO: Use those helpers for email errors
func errorSource(fn, errMsg string) string {
	return "Error: " + fn + "(): " + errMsg
}

func errorUpdateRow(source string, data string, id int64, err error, n int64) error {
	if err != nil {
		return fmt.Errorf("%s get affected rows for %s %d: %w", source, data, id, err)
	}
	if n != 1 {
		return fmt.Errorf("%s: expected to update 1 row for %d, updated %d", source, id, n)
	}

	return nil
}
