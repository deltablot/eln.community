package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
)

var (
	ErrRecordNotFound = errors.New("record not found")
)

// RecordRepository defines the interface for record data operations
type RecordRepository interface {
	GetAll(ctx context.Context) ([]Record, error)
	GetAllByCategory(ctx context.Context, categoryID int64) ([]Record, error)
	Search(ctx context.Context, query string, categoryID int64) ([]Record, error)
	GetByID(ctx context.Context, id string) (*Record, error)
	Create(ctx context.Context, tx *sql.Tx, record *Record, s3Key string) error
	GetS3Key(ctx context.Context, id string) (string, error)
}

// PostgresRecordRepository implements RecordRepository using PostgreSQL
type PostgresRecordRepository struct {
	db           *sql.DB
	categoryRepo CategoryRepository
	rorRepo      RorRepository
}

// NewPostgresRecordRepository creates a new PostgreSQL record repository
func NewPostgresRecordRepository(db *sql.DB, categoryRepo CategoryRepository, rorRepo RorRepository) *PostgresRecordRepository {
	return &PostgresRecordRepository{
		db:           db,
		categoryRepo: categoryRepo,
		rorRepo:      rorRepo,
	}
}

// GetAll retrieves all records with their categories and ROR IDs
func (r *PostgresRecordRepository) GetAll(ctx context.Context) ([]Record, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, sha256, name, metadata, created_at, modified_at
		FROM records
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		if err := rows.Scan(
			&record.Id,
			&record.Sha256,
			&record.Name,
			&record.Metadata,
			&record.CreatedAt,
			&record.ModifiedAt,
		); err != nil {
			return nil, err
		}

		// Get categories for this record
		categories, err := r.categoryRepo.GetRecordCategories(ctx, record.Id)
		if err != nil {
			return nil, err
		}
		record.Categories = categories

		// Get ROR IDs for this record
		rorIds, err := r.rorRepo.GetRecordRorIds(ctx, record.Id)
		if err != nil {
			return nil, err
		}
		record.RorIds = rorIds

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

// GetByID retrieves a record by its ID with categories and ROR IDs
func (r *PostgresRecordRepository) GetByID(ctx context.Context, id string) (*Record, error) {
	var record Record
	err := r.db.QueryRowContext(ctx, `
		SELECT id, sha256, name, metadata, created_at, modified_at, uploader_name, uploader_orcid
		FROM records
		WHERE id = $1
	`, id).Scan(
		&record.Id,
		&record.Sha256,
		&record.Name,
		&record.Metadata,
		&record.CreatedAt,
		&record.ModifiedAt,
		&record.UploaderName,
		&record.UploaderOrcid,
	)

	if err == sql.ErrNoRows {
		return nil, ErrRecordNotFound
	}
	if err != nil {
		return nil, err
	}

	// Get categories for this record
	categories, err := r.categoryRepo.GetRecordCategories(ctx, record.Id)
	if err != nil {
		return nil, err
	}
	record.Categories = categories

	// Get ROR IDs for this record
	rorIds, err := r.rorRepo.GetRecordRorIds(ctx, record.Id)
	if err != nil {
		return nil, err
	}
	record.RorIds = rorIds

	return &record, nil
}

// Create creates a new record within a transaction
func (r *PostgresRecordRepository) Create(ctx context.Context, tx *sql.Tx, record *Record, s3Key string) error {
	// Insert the main record without ROR IDs
	_, err := tx.ExecContext(ctx,
		`INSERT INTO records (id, s3_key, sha256, name, metadata, uploader_name, uploader_orcid) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		record.Id, s3Key, record.Sha256, record.Name, record.Metadata,
		record.UploaderName, record.UploaderOrcid,
	)
	if err != nil {
		return err
	}

	// Insert ROR associations
	for _, rorId := range record.RorIds {
		if err := r.rorRepo.AssociateRorWithRecord(ctx, tx, record.Id, rorId); err != nil {
			return err
		}
	}

	return nil
}

// GetS3Key retrieves the S3 key for a record
func (r *PostgresRecordRepository) GetS3Key(ctx context.Context, id string) (string, error) {
	var s3Key string
	err := r.db.QueryRowContext(ctx,
		`SELECT s3_key FROM records WHERE id = $1`, id,
	).Scan(&s3Key)

	if err == sql.ErrNoRows {
		return "", ErrRecordNotFound
	}
	if err != nil {
		return "", err
	}

	return s3Key, nil
}

// GetAllByCategory retrieves all records filtered by category with their categories and ROR IDs
func (r *PostgresRecordRepository) GetAllByCategory(ctx context.Context, categoryID int64) ([]Record, error) {
	// First, let's check if the category exists
	var categoryExists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1)`, categoryID).Scan(&categoryExists)
	if err != nil {
		log.Printf("Error checking if category exists: %v", err)
		return nil, err
	}
	if !categoryExists {
		log.Printf("Category %d does not exist", categoryID)
		return []Record{}, nil // Return empty slice instead of error
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at
		FROM records r
		JOIN records_categories rc ON r.id = rc.record_id
		WHERE rc.category_id = $1
		ORDER BY r.created_at DESC
	`, categoryID)
	if err != nil {
		log.Printf("Error in GetAllByCategory query for category %d: %v", categoryID, err)
		return nil, err
	}
	defer rows.Close()

	var records []Record
	recordCount := 0
	for rows.Next() {
		recordCount++
		var record Record
		if err := rows.Scan(
			&record.Id,
			&record.Sha256,
			&record.Name,
			&record.Metadata,
			&record.CreatedAt,
			&record.ModifiedAt,
		); err != nil {
			log.Printf("Error scanning record %d: %v", recordCount, err)
			return nil, err
		}

		// Get categories for this record
		categories, err := r.categoryRepo.GetRecordCategories(ctx, record.Id)
		if err != nil {
			log.Printf("Error getting categories for record %s: %v", record.Id, err)
			return nil, err
		}
		record.Categories = categories

		// Get ROR IDs for this record
		rorIds, err := r.rorRepo.GetRecordRorIds(ctx, record.Id)
		if err != nil {
			log.Printf("Error getting ROR IDs for record %s: %v", record.Id, err)
			return nil, err
		}
		record.RorIds = rorIds

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error in rows iteration: %v", err)
		return nil, err
	}

	return records, nil
}

// Search retrieves records based on search query, optionally filtered by category
func (r *PostgresRecordRepository) Search(ctx context.Context, query string, categoryID int64) ([]Record, error) {
	var sqlQuery string
	var args []interface{}

	if categoryID > 0 {
		// Search within a specific category
		sqlQuery = `
			SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at
			FROM records r
			JOIN records_categories rc ON r.id = rc.record_id
			LEFT JOIN records_ror rr ON r.id = rr.record_id
			LEFT JOIN categories c ON rc.category_id = c.id
			WHERE rc.category_id = $1 AND (
				r.name ILIKE $2 OR
				r.metadata::text ILIKE $2 OR
				r.uploader_name ILIKE $2 OR
				r.uploader_orcid ILIKE $2 OR
				rr.ror ILIKE $2 OR
				c.name ILIKE $2
			)
			ORDER BY r.created_at DESC
		`
		args = []interface{}{categoryID, "%" + query + "%"}
	} else {
		// Search across all records
		sqlQuery = `
			SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at
			FROM records r
			LEFT JOIN records_ror rr ON r.id = rr.record_id
			LEFT JOIN records_categories rc ON r.id = rc.record_id
			LEFT JOIN categories c ON rc.category_id = c.id
			WHERE (
				r.name ILIKE $1 OR
				r.metadata::text ILIKE $1 OR
				r.uploader_name ILIKE $1 OR
				r.uploader_orcid ILIKE $1 OR
				rr.ror ILIKE $1 OR
				c.name ILIKE $1
			)
			ORDER BY r.created_at DESC
		`
		args = []interface{}{"%" + query + "%"}
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		if err := rows.Scan(
			&record.Id,
			&record.Sha256,
			&record.Name,
			&record.Metadata,
			&record.CreatedAt,
			&record.ModifiedAt,
		); err != nil {
			return nil, err
		}

		// Get categories for this record
		categories, err := r.categoryRepo.GetRecordCategories(ctx, record.Id)
		if err != nil {
			return nil, err
		}
		record.Categories = categories

		// Get ROR IDs for this record
		rorIds, err := r.rorRepo.GetRecordRorIds(ctx, record.Id)
		if err != nil {
			return nil, err
		}
		record.RorIds = rorIds

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}
