package main

import (
	"context"
	"database/sql"
	"time"
)

type Admin struct {
	orcid      string
	email      string
	CreatedAt  time.Time `json:"created_at"`
	ModifiedAt time.Time `json:"modified_at"`
}

// AdminRepository defines the interface for admin operations
type AdminRepository interface {
	IsAdmin(ctx context.Context, orcid string) (bool, error)
	GetNotifiableAdmins(ctx context.Context) ([]Admin, error)
}

// PostgresAdminRepository implements AdminRepository using PostgreSQL
type PostgresAdminRepository struct {
	db *sql.DB
}

// NewPostgresAdminRepository creates a new PostgreSQL admin repository
func NewPostgresAdminRepository(db *sql.DB) *PostgresAdminRepository {
	return &PostgresAdminRepository{db: db}
}

// IsAdmin checks if the given ORCID belongs to an admin
func (r *PostgresAdminRepository) IsAdmin(ctx context.Context, orcid string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM admin_orcids WHERE orcid = $1)`, orcid).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *PostgresAdminRepository) GetNotifiableAdmins(ctx context.Context) ([]Admin, error) {
	rows, err := db.Query(`SELECT orcid, email, created_at, modified_at FROM admin_orcids WHERE email IS NOT NULL AND email != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var admins []Admin
	for rows.Next() {
		var admin Admin
		if err := rows.Scan(&admin.orcid, &admin.email, &admin.CreatedAt, &admin.ModifiedAt); err != nil {
			return admins, err
		}
		admins = append(admins, admin)
	}
	if err = rows.Err(); err != nil {
		return admins, err
	}
	return admins, nil
}
