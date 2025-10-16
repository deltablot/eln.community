package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var (
	ErrCategoryNotFound      = errors.New("category not found")
	ErrCategoryAlreadyExists = errors.New("category already exists")
)

// CategoryRepository defines the interface for category data operations
type CategoryRepository interface {
	GetAll(ctx context.Context) ([]Category, error)
	GetByID(ctx context.Context, id int64) (*Category, error)
	Create(ctx context.Context, name string) (*Category, error)
	Update(ctx context.Context, id int64, name string) (*Category, error)
	Delete(ctx context.Context, id int64) error
	// Record-category association methods
	AssociateCategoryWithRecord(ctx context.Context, tx *sql.Tx, recordID string, categoryID int64) error
	GetRecordCategories(ctx context.Context, recordID string) ([]Category, error)
}

// AdminRepository defines the interface for admin operations
type AdminRepository interface {
	IsAdmin(ctx context.Context, orcid string) (bool, error)
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

// AssociateCategoryWithRecord creates an association between a record and a category
func (r *PostgresCategoryRepository) AssociateCategoryWithRecord(ctx context.Context, tx *sql.Tx, recordID string, categoryID int64) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO records_categories (record_id, category_id) VALUES ($1, $2)`,
		recordID, categoryID,
	)
	return err
}

// GetRecordCategories retrieves all categories associated with a specific record
func (r *PostgresCategoryRepository) GetRecordCategories(ctx context.Context, recordID string) ([]Category, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.created_at, c.modified_at
		FROM categories c
		JOIN records_categories rc ON c.id = rc.category_id
		WHERE rc.record_id = $1
		ORDER BY c.name
	`, recordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var cat Category
		if err := rows.Scan(&cat.Id, &cat.Name, &cat.CreatedAt, &cat.ModifiedAt); err != nil {
			return nil, err
		}
		categories = append(categories, cat)
	}
	return categories, rows.Err()
}
