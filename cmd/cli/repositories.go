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
	ParentId   *int64
	CreatedAt  time.Time
	ModifiedAt time.Time
}

type Admin struct {
	Orcid      string
	Email      string
	CreatedAt  time.Time
	ModifiedAt time.Time
}

// CategoryRepository defines the interface for category data operations
type CategoryRepository interface {
	GetAll(ctx context.Context) ([]Category, error)
	GetByID(ctx context.Context, id int64) (*Category, error)
	GetByName(ctx context.Context, name string) (*Category, error)
	Create(ctx context.Context, name string, parentID *int64) (*Category, error)
	Update(ctx context.Context, id int64, name string, parentID *int64) (*Category, error)
	Delete(ctx context.Context, id int64) error
}

// AdminRepository defines the interface for admin operations
type AdminRepository interface {
	IsAdmin(ctx context.Context, orcid string) (bool, error)
	GetAll(ctx context.Context) ([]Admin, error)
	Add(ctx context.Context, orcid string, email string) (*Admin, error)
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
		SELECT id, name, parent_id, created_at, modified_at 
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
		if err := rows.Scan(&c.Id, &c.Name, &c.ParentId, &c.CreatedAt, &c.ModifiedAt); err != nil {
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
		SELECT id, name, parent_id, created_at, modified_at 
		FROM categories 
		WHERE id = $1
	`, id).Scan(&category.Id, &category.Name, &category.ParentId, &category.CreatedAt, &category.ModifiedAt)

	if err == sql.ErrNoRows {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, err
	}

	return &category, nil
}

// GetByName retrieves a category by its name
func (r *PostgresCategoryRepository) GetByName(ctx context.Context, name string) (*Category, error) {
	var category Category
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, parent_id, created_at, modified_at 
		FROM categories 
		WHERE name = $1
	`, name).Scan(&category.Id, &category.Name, &category.ParentId, &category.CreatedAt, &category.ModifiedAt)

	if err == sql.ErrNoRows {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, err
	}

	return &category, nil
}

// Create creates a new category
func (r *PostgresCategoryRepository) Create(ctx context.Context, name string, parentID *int64) (*Category, error) {
	var category Category
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO categories (name, parent_id) 
		VALUES ($1, $2) 
		RETURNING id, name, parent_id, created_at, modified_at
	`, name, parentID).Scan(&category.Id, &category.Name, &category.ParentId, &category.CreatedAt, &category.ModifiedAt)

	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			return nil, ErrCategoryAlreadyExists
		}
		return nil, err
	}

	return &category, nil
}

// Update updates an existing category
func (r *PostgresCategoryRepository) Update(ctx context.Context, id int64, name string, parentID *int64) (*Category, error) {
	var category Category
	err := r.db.QueryRowContext(ctx, `
		UPDATE categories 
		SET name = $1, parent_id = $2 
		WHERE id = $3 
		RETURNING id, name, parent_id, created_at, modified_at
	`, name, parentID, id).Scan(&category.Id, &category.Name, &category.ParentId, &category.CreatedAt, &category.ModifiedAt)

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
		SELECT orcid, email, created_at, modified_at 
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
		var email sql.NullString
		if err := rows.Scan(&admin.Orcid, &email, &admin.CreatedAt, &admin.ModifiedAt); err != nil {
			return nil, err
		}
		if email.Valid {
			admin.Email = email.String
		}
		admins = append(admins, admin)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return admins, nil
}

// Add adds a new admin ORCID with optional email
func (r *PostgresAdminRepository) Add(ctx context.Context, orcid string, email string) (*Admin, error) {
	var admin Admin
	var emailVal sql.NullString
	if email != "" {
		emailVal = sql.NullString{String: email, Valid: true}
	}

	err := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_orcids (orcid, email) 
		VALUES ($1, $2) 
		RETURNING orcid, email, created_at, modified_at
	`, orcid, emailVal).Scan(&admin.Orcid, &emailVal, &admin.CreatedAt, &admin.ModifiedAt)

	if err != nil {
		return nil, err
	}

	if emailVal.Valid {
		admin.Email = emailVal.String
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
