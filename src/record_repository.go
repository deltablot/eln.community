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
	GetAllPaginated(ctx context.Context, limit, offset int) ([]Record, int, error)
	GetAllByCategory(ctx context.Context, categoryID int64) ([]Record, error)
	GetAllByCategoryPaginated(ctx context.Context, categoryID int64, limit, offset int) ([]Record, int, error)
	GetAllByRorID(ctx context.Context, rorID string) ([]Record, error)
	GetAllByRorIDPaginated(ctx context.Context, rorID string, limit, offset int) ([]Record, int, error)
	Search(ctx context.Context, query string, categoryID int64) ([]Record, error)
	SearchPaginated(ctx context.Context, query string, categoryID int64, limit, offset int) ([]Record, int, error)
	GetByID(ctx context.Context, id string) (*Record, error)
	Create(ctx context.Context, tx *sql.Tx, record *Record, s3Key string) error
	Update(ctx context.Context, tx *sql.Tx, record *Record) error
	Delete(ctx context.Context, tx *sql.Tx, id string) error
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
		SELECT id, sha256, name, metadata, created_at, modified_at, uploader_orcid
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
			&record.UploaderOrcid,
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

// GetAllPaginated retrieves records with pagination
func (r *PostgresRecordRepository) GetAllPaginated(ctx context.Context, limit, offset int) ([]Record, int, error) {
	// Get total count
	var totalCount int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM records`).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, sha256, name, metadata, created_at, modified_at, uploader_orcid
		FROM records
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
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
			&record.UploaderOrcid,
		); err != nil {
			return nil, 0, err
		}

		// Get categories for this record
		categories, err := r.categoryRepo.GetRecordCategories(ctx, record.Id)
		if err != nil {
			return nil, 0, err
		}
		record.Categories = categories

		// Get ROR IDs for this record
		rorIds, err := r.rorRepo.GetRecordRorIds(ctx, record.Id)
		if err != nil {
			return nil, 0, err
		}
		record.RorIds = rorIds

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return records, totalCount, nil
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
		SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_orcid
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
			&record.UploaderOrcid,
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

// GetAllByCategoryPaginated retrieves records filtered by category with pagination
func (r *PostgresRecordRepository) GetAllByCategoryPaginated(ctx context.Context, categoryID int64, limit, offset int) ([]Record, int, error) {
	// Get total count
	var totalCount int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT r.id)
		FROM records r
		JOIN records_categories rc ON r.id = rc.record_id
		WHERE rc.category_id = $1
	`, categoryID).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_orcid
		FROM records r
		JOIN records_categories rc ON r.id = rc.record_id
		WHERE rc.category_id = $1
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3
	`, categoryID, limit, offset)
	if err != nil {
		return nil, 0, err
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
			&record.UploaderOrcid,
		); err != nil {
			return nil, 0, err
		}

		// Get categories for this record
		categories, err := r.categoryRepo.GetRecordCategories(ctx, record.Id)
		if err != nil {
			return nil, 0, err
		}
		record.Categories = categories

		// Get ROR IDs for this record
		rorIds, err := r.rorRepo.GetRecordRorIds(ctx, record.Id)
		if err != nil {
			return nil, 0, err
		}
		record.RorIds = rorIds

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return records, totalCount, nil
}

// GetAllByRorID retrieves all records filtered by ROR ID with their categories and ROR IDs
func (r *PostgresRecordRepository) GetAllByRorID(ctx context.Context, rorID string) ([]Record, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_orcid
		FROM records r
		JOIN records_ror rr ON r.id = rr.record_id
		WHERE rr.ror = $1
		ORDER BY r.created_at DESC
	`, rorID)
	if err != nil {
		log.Printf("Error in GetAllByRorID query for ROR %s: %v", rorID, err)
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
			&record.UploaderOrcid,
		); err != nil {
			log.Printf("Error scanning record: %v", err)
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

// GetAllByRorIDPaginated retrieves records filtered by ROR ID with pagination
func (r *PostgresRecordRepository) GetAllByRorIDPaginated(ctx context.Context, rorID string, limit, offset int) ([]Record, int, error) {
	// Get total count
	var totalCount int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT r.id)
		FROM records r
		JOIN records_ror rr ON r.id = rr.record_id
		WHERE rr.ror = $1
	`, rorID).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_orcid
		FROM records r
		JOIN records_ror rr ON r.id = rr.record_id
		WHERE rr.ror = $1
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3
	`, rorID, limit, offset)
	if err != nil {
		return nil, 0, err
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
			&record.UploaderOrcid,
		); err != nil {
			return nil, 0, err
		}

		// Get categories for this record
		categories, err := r.categoryRepo.GetRecordCategories(ctx, record.Id)
		if err != nil {
			return nil, 0, err
		}
		record.Categories = categories

		// Get ROR IDs for this record
		rorIds, err := r.rorRepo.GetRecordRorIds(ctx, record.Id)
		if err != nil {
			return nil, 0, err
		}
		record.RorIds = rorIds

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return records, totalCount, nil
}

// Search retrieves records based on search query, optionally filtered by category
func (r *PostgresRecordRepository) Search(ctx context.Context, query string, categoryID int64) ([]Record, error) {
	var sqlQuery string
	var args []interface{}

	if categoryID > 0 {
		// Search within a specific category
		sqlQuery = `
			SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_orcid
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
			SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_orcid
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
			&record.UploaderOrcid,
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

// SearchPaginated retrieves records based on search query with pagination
func (r *PostgresRecordRepository) SearchPaginated(ctx context.Context, query string, categoryID int64, limit, offset int) ([]Record, int, error) {
	var countQuery string
	var sqlQuery string
	var args []interface{}
	var countArgs []interface{}

	if categoryID > 0 {
		// Count query for specific category
		countQuery = `
			SELECT COUNT(DISTINCT r.id)
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
		`
		countArgs = []interface{}{categoryID, "%" + query + "%"}

		// Search within a specific category
		sqlQuery = `
			SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_orcid
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
			LIMIT $3 OFFSET $4
		`
		args = []interface{}{categoryID, "%" + query + "%", limit, offset}
	} else {
		// Count query for all records
		countQuery = `
			SELECT COUNT(DISTINCT r.id)
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
		`
		countArgs = []interface{}{"%" + query + "%"}

		// Search across all records
		sqlQuery = `
			SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_orcid
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
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{"%" + query + "%", limit, offset}
	}

	// Get total count
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, 0, err
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
			&record.UploaderOrcid,
		); err != nil {
			return nil, 0, err
		}

		// Get categories for this record
		categories, err := r.categoryRepo.GetRecordCategories(ctx, record.Id)
		if err != nil {
			return nil, 0, err
		}
		record.Categories = categories

		// Get ROR IDs for this record
		rorIds, err := r.rorRepo.GetRecordRorIds(ctx, record.Id)
		if err != nil {
			return nil, 0, err
		}
		record.RorIds = rorIds

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return records, totalCount, nil
}

// Update updates an existing record within a transaction
func (r *PostgresRecordRepository) Update(ctx context.Context, tx *sql.Tx, record *Record) error {
	// Update the main record
	_, err := tx.ExecContext(ctx,
		`UPDATE records SET name = $2, modified_at = now() WHERE id = $1`,
		record.Id, record.Name,
	)
	if err != nil {
		return err
	}

	// Clear existing category associations
	_, err = tx.ExecContext(ctx,
		`DELETE FROM records_categories WHERE record_id = $1`,
		record.Id,
	)
	if err != nil {
		return err
	}

	// Clear existing ROR associations
	_, err = tx.ExecContext(ctx,
		`DELETE FROM records_ror WHERE record_id = $1`,
		record.Id,
	)
	if err != nil {
		return err
	}

	// Insert new ROR associations
	for _, rorId := range record.RorIds {
		if err := r.rorRepo.AssociateRorWithRecord(ctx, tx, record.Id, rorId); err != nil {
			return err
		}
	}

	return nil
}

// Delete removes a record and all its associations within a transaction
func (r *PostgresRecordRepository) Delete(ctx context.Context, tx *sql.Tx, id string) error {
	// Delete the record (cascading deletes will handle associations)
	result, err := tx.ExecContext(ctx,
		`DELETE FROM records WHERE id = $1`,
		id,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}
