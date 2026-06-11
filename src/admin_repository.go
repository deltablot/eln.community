package main

import (
	"context"
	"database/sql"
	"time"
)

type Admin struct {
	Orcid      string
	Email      string
	CreatedAt  time.Time `json:"created_at"`
	ModifiedAt time.Time `json:"modified_at"`
}

type AdminRepository interface {
	IsAdmin(ctx context.Context, orcid string) (bool, error)
	GetAllAdmins(ctx context.Context) ([]Admin, error)
}

type PostgresAdminRepository struct {
	db *sql.DB
}

func NewPostgresAdminRepository(db *sql.DB) *PostgresAdminRepository {
	return &PostgresAdminRepository{db: db}
}

func (r *PostgresAdminRepository) IsAdmin(ctx context.Context, orcid string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM admin_orcids WHERE orcid = $1)`, orcid).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (r *PostgresAdminRepository) GetAllAdmins(ctx context.Context) ([]Admin, error) {
	rows, err := r.db.Query(`SELECT orcid, created_at, modified_at FROM admin_orcids`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var admins []Admin
	for rows.Next() {
		var admin Admin
		if err := rows.Scan(&admin.Orcid, &admin.CreatedAt, &admin.ModifiedAt); err != nil {
			return admins, err
		}
		admins = append(admins, admin)
	}
	if err = rows.Err(); err != nil {
		return admins, err
	}

	return admins, nil
}
