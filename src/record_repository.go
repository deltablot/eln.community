package main

import (
	"context"
	"database/sql"
	"errors"
)

var (
	ErrRecordNotFound = errors.New("record not found")
)

// RecordRepository defines the interface for record data operations
type RecordRepository interface {
	GetAllPaginated(ctx context.Context, limit, offset int) ([]Record, int, error)
	GetAllByCategoryPaginated(ctx context.Context, categoryID int64, limit, offset int) ([]Record, int, error)
	GetAllByRorIDPaginated(ctx context.Context, rorID string, limit, offset int) ([]Record, int, error)
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

// GetAllPaginated retrieves records with pagination
func (r *PostgresRecordRepository) GetAllPaginated(ctx context.Context, limit, offset int) ([]Record, int, error) {
	// Get total count
	var totalCount int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM records`).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, sha256, name, metadata, created_at, modified_at, uploader_name, uploader_orcid
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
			&record.UploaderName,
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
		SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_name, r.uploader_orcid
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
			&record.UploaderName,
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
		SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_name, r.uploader_orcid
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
			&record.UploaderName,
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
			SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_name, r.uploader_orcid
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
			SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_name, r.uploader_orcid
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
			&record.UploaderName,
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
