package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// AdminOrcid represents an admin ORCID entry
type AdminOrcid struct {
	Orcid      string `json:"orcid"`
	CreatedAt  string `json:"created_at"`
	ModifiedAt string `json:"modified_at"`
}

// POST /api/v1/admin/orcids - Add a new admin ORCID (requires existing admin)
func addAdminOrcidHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Orcid string `json:"orcid"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Orcid) == "" {
		http.Error(w, "ORCID is required", http.StatusBadRequest)
		return
	}

	// Validate ORCID format (basic check)
	if len(req.Orcid) != 19 || !strings.Contains(req.Orcid, "-") {
		http.Error(w, "Invalid ORCID format. Expected format: 0000-0000-0000-0000", http.StatusBadRequest)
		return
	}

	var adminOrcid AdminOrcid
	err := db.QueryRowContext(r.Context(), `
		INSERT INTO admin_orcids (orcid) 
		VALUES ($1) 
		RETURNING orcid, created_at, modified_at
	`, req.Orcid).Scan(&adminOrcid.Orcid, &adminOrcid.CreatedAt, &adminOrcid.ModifiedAt)

	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			http.Error(w, "ORCID already exists as admin", http.StatusConflict)
			return
		}
		http.Error(w, "Error adding admin ORCID", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(adminOrcid); err != nil {
		errorLogger.Printf("failed to write response: %v", err)
	}
}

// GET /api/v1/admin/orcids - List all admin ORCIDs (requires admin)
func listAdminOrcidsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.QueryContext(r.Context(), `
		SELECT orcid, created_at, modified_at 
		FROM admin_orcids 
		ORDER BY created_at DESC
	`)
	if err != nil {
		http.Error(w, "Error fetching admin ORCIDs", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var adminOrcids []AdminOrcid
	for rows.Next() {
		var ao AdminOrcid
		if err := rows.Scan(&ao.Orcid, &ao.CreatedAt, &ao.ModifiedAt); err != nil {
			http.Error(w, "Error scanning admin ORCIDs", http.StatusInternalServerError)
			return
		}
		adminOrcids = append(adminOrcids, ao)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Error iterating admin ORCIDs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(adminOrcids); err != nil {
		errorLogger.Printf("failed to write response: %v", err)
	}
}

// DELETE /api/v1/admin/orcids/{orcid} - Remove an admin ORCID (requires admin)
func removeAdminOrcidHandler(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/admin/orcids/"
	orcid := strings.TrimPrefix(r.URL.Path, prefix)

	if strings.TrimSpace(orcid) == "" {
		http.Error(w, "ORCID is required", http.StatusBadRequest)
		return
	}

	result, err := db.ExecContext(r.Context(), `
		DELETE FROM admin_orcids WHERE orcid = $1
	`, orcid)

	if err != nil {
		http.Error(w, "Error removing admin ORCID", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Error checking deletion result", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "Admin ORCID not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// requireAdmin middleware to check admin permissions (standalone version)
func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		orcid, ok := sessionManager.Get(ctx, "orcid").(string)
		if !ok || orcid == "" {
			http.Error(w, "Unauthorized: login required", http.StatusUnauthorized)
			return
		}

		adminRepo := NewPostgresAdminRepository(db)
		isAdminUser, err := adminRepo.IsAdmin(ctx, orcid)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if !isAdminUser {
			http.Error(w, "Forbidden: admin access required", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

// adminRouter handles routing for admin endpoints
func adminRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/v1/admin/orcids" && r.Method == "GET":
		requireAdmin(listAdminOrcidsHandler)(w, r)
	case path == "/api/v1/admin/orcids" && r.Method == "POST":
		requireAdmin(addAdminOrcidHandler)(w, r)
	case strings.HasPrefix(path, "/api/v1/admin/orcids/") && r.Method == "DELETE":
		requireAdmin(removeAdminOrcidHandler)(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

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
