package main

import (
	"context"
)

// initFirstAdmin creates the first admin if no admins exist
// This should be called during application startup
func initFirstAdmin(ctx context.Context, orcid string) error {
	if orcid == "" {
		return nil // No first admin specified
	}

	// Check if any admins exist
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_orcids`).Scan(&count)
	if err != nil {
		return err
	}

	// If no admins exist, create the first one
	if count == 0 {
		_, err = db.ExecContext(ctx, `INSERT INTO admin_orcids (orcid) VALUES ($1)`, orcid)
		if err != nil {
			return err
		}
		infoLogger.Printf("Created first admin with ORCID: %s", orcid)
	}

	return nil
}
