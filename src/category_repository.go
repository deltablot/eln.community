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
	GetAllHierarchical(ctx context.Context) ([]Category, error)
	GetByID(ctx context.Context, id int64) (*Category, error)
	Create(ctx context.Context, name string, parentID *int64) (*Category, error)
	Update(ctx context.Context, id int64, name string, parentID *int64) (*Category, error)
	Delete(ctx context.Context, id int64) error
	GetSubcategories(ctx context.Context, parentID int64) ([]Category, error)
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

// GetAll retrieves all categories (flat list)
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

// GetAllHierarchical retrieves all categories organized in a tree structure
func (r *PostgresCategoryRepository) GetAllHierarchical(ctx context.Context) ([]Category, error) {
	// Get all categories
	allCategories, err := r.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	// Build a map for quick lookup
	categoryMap := make(map[int64]*Category)
	for i := range allCategories {
		categoryMap[allCategories[i].Id] = &allCategories[i]
		allCategories[i].Subcategories = []Category{}
	}

	// Build the tree structure
	var rootCategories []Category
	for i := range allCategories {
		cat := &allCategories[i]
		if cat.ParentId == nil {
			// Root level category
			rootCategories = append(rootCategories, *cat)
		} else {
			// Add to parent's subcategories
			if parent, exists := categoryMap[*cat.ParentId]; exists {
				parent.Subcategories = append(parent.Subcategories, *cat)
			}
		}
	}

	// Update root categories with their populated subcategories
	for i := range rootCategories {
		if cat, exists := categoryMap[rootCategories[i].Id]; exists {
			rootCategories[i].Subcategories = cat.Subcategories
		}
	}

	return rootCategories, nil
}

// GetSubcategories retrieves all direct children of a category
func (r *PostgresCategoryRepository) GetSubcategories(ctx context.Context, parentID int64) ([]Category, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, parent_id, created_at, modified_at 
		FROM categories 
		WHERE parent_id = $1
		ORDER BY name
	`, parentID)
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

	return categories, rows.Err()
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
		SELECT c.id, c.name, c.parent_id, c.created_at, c.modified_at
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
		if err := rows.Scan(&cat.Id, &cat.Name, &cat.ParentId, &cat.CreatedAt, &cat.ModifiedAt); err != nil {
			return nil, err
		}
		categories = append(categories, cat)
	}
	return categories, rows.Err()
}
