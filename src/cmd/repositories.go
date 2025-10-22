package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrCategoryNotFound      = errors.New("category not found")
	ErrCategoryAlreadyExists = errors.New("category already exists")
	ErrAdminNotFound         = errors.New("admin not found")
)

type Category struct {
	Id         int64
	Name       string
	CreatedAt  time.Time
	ModifiedAt time.Time
}

type Admin struct {
	Orcid      string
	CreatedAt  time.Time
	ModifiedAt time.Time
}

// CategoryRepository defines the interface for category data operations
type CategoryRepository interface {
	GetAll(ctx context.Context) ([]Category, error)
	GetByID(ctx context.Context, id int64) (*Category, error)
	Create(ctx context.Context, name string) (*Category, error)
	Update(ctx context.Context, id int64, name string) (*Category, error)
	Delete(ctx context.Context, id int64) error
}

// AdminRepository defines the interface for admin operations
type AdminRepository interface {
	IsAdmin(ctx context.Context, orcid string) (bool, error)
	GetAll(ctx context.Context) ([]Admin, error)
	Add(ctx context.Context, orcid string) (*Admin, error)
	Remove(ctx context.Context, orcid string) error
}

// PostgresCategoryRepository implements CategoryRepository using PostgreSQL
type PostgresCategoryRepository struct {
	db *sql.DB
}

// NewPostgresCategoryRepository creates a new PostgreSQL category repository
func NewPostgresCategoryRepository(db *sql.DB) *PostgresCategoryRepository {
	return &PostgresCategoryRepository{db: db}
}

// GetAll retrieves all categories
func (r *PostgresCategoryRepository) GetAll(ctx context.Context) ([]Category, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, created_at, modified_at 
		FROM categories 
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.Id, &c.Name, &c.CreatedAt, &c.ModifiedAt); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}

// GetByID retrieves a category by its ID
func (r *PostgresCategoryRepository) GetByID(ctx context.Context, id int64) (*Category, error) {
	var category Category
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, created_at, modified_at 
		FROM categories 
		WHERE id = $1
	`, id).Scan(&category.Id, &category.Name, &category.CreatedAt, &category.ModifiedAt)

	if err == sql.ErrNoRows {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, err
	}

	return &category, nil
}

// Create creates a new category
func (r *PostgresCategoryRepository) Create(ctx context.Context, name string) (*Category, error) {
	var category Category
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO categories (name) 
		VALUES ($1) 
		RETURNING id, name, created_at, modified_at
	`, name).Scan(&category.Id, &category.Name, &category.CreatedAt, &category.ModifiedAt)

	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			return nil, ErrCategoryAlreadyExists
		}
		return nil, err
	}

	return &category, nil
}

// Update updates an existing category
func (r *PostgresCategoryRepository) Update(ctx context.Context, id int64, name string) (*Category, error) {
	var category Category
	err := r.db.QueryRowContext(ctx, `
		UPDATE categories 
		SET name = $1 
		WHERE id = $2 
		RETURNING id, name, created_at, modified_at
	`, name, id).Scan(&category.Id, &category.Name, &category.CreatedAt, &category.ModifiedAt)

	if err == sql.ErrNoRows {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			return nil, ErrCategoryAlreadyExists
		}
		return nil, err
	}

	return &category, nil
}

// Delete deletes a category by ID
func (r *PostgresCategoryRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM categories WHERE id = $1
	`, id)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrCategoryNotFound
	}

	return nil
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
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM admin_orcids WHERE orcid = $1)
	`, orcid).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// GetAll retrieves all admin ORCIDs
func (r *PostgresAdminRepository) GetAll(ctx context.Context) ([]Admin, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT orcid, created_at, modified_at 
		FROM admin_orcids 
		ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var admins []Admin
	for rows.Next() {
		var admin Admin
		if err := rows.Scan(&admin.Orcid, &admin.CreatedAt, &admin.ModifiedAt); err != nil {
			return nil, err
		}
		admins = append(admins, admin)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return admins, nil
}

// Add adds a new admin ORCID
func (r *PostgresAdminRepository) Add(ctx context.Context, orcid string) (*Admin, error) {
	var admin Admin
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_orcids (orcid) 
		VALUES ($1) 
		RETURNING orcid, created_at, modified_at
	`, orcid).Scan(&admin.Orcid, &admin.CreatedAt, &admin.ModifiedAt)

	if err != nil {
		return nil, err
	}

	return &admin, nil
}

// Remove removes an admin ORCID
func (r *PostgresAdminRepository) Remove(ctx context.Context, orcid string) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM admin_orcids WHERE orcid = $1
	`, orcid)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrAdminNotFound
	}

	return nil
}
