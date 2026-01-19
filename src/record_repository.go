package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrRecordNotFound = errors.New("record not found")
)

// RecordRepository defines the interface for record data operations
type RecordRepository interface {
	GetAllPaginated(ctx context.Context, limit, offset int, orderBy, sortOrder string) ([]Record, int, error)
	GetAllByCategoriesPaginated(ctx context.Context, categoryIDs []int64, limit, offset int, orderBy, sortOrder string) ([]Record, int, error)
	GetAllByRorIDsPaginated(ctx context.Context, rorIDs []string, limit, offset int, orderBy, sortOrder string) ([]Record, int, error)
	GetAllByOrcidPaginated(ctx context.Context, orcid string, limit, offset int) ([]Record, int, error)
	SearchPaginated(ctx context.Context, query string, categoryID int64, limit, offset int, orderBy, sortOrder string) ([]Record, int, error)
	GetByID(ctx context.Context, id string) (*Record, error)
	Create(ctx context.Context, tx *sql.Tx, record *Record, s3Key string) error
	Update(ctx context.Context, tx *sql.Tx, record *Record) error
	Delete(ctx context.Context, tx *sql.Tx, id string) error
	GetS3Key(ctx context.Context, id string) (string, error)
	IncrementDownloadCount(ctx context.Context, id string) (int, error)
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
func (r *PostgresRecordRepository) GetAllPaginated(ctx context.Context, limit, offset int, orderBy, sortOrder string) ([]Record, int, error) {
	// Get total count - only approved records for public view
	var totalCount int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM records WHERE moderation_status = 'approved'`).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Build ORDER BY clause with SQL injection protection
	orderByClause := fmt.Sprintf("ORDER BY %s %s", orderBy, strings.ToUpper(sortOrder))

	query := fmt.Sprintf(`
		SELECT id, sha256, name, metadata, created_at, modified_at, uploader_name, uploader_orcid, download_count
		FROM records
		WHERE moderation_status = 'approved'
		%s
		LIMIT $1 OFFSET $2
	`, orderByClause)

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
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
			&record.DownloadCount,
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
	var moderationStatus string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, sha256, name, metadata, created_at, modified_at, uploader_name, uploader_orcid, download_count, moderation_status
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
		&record.DownloadCount,
		&moderationStatus,
	)

	if err == sql.ErrNoRows {
		return nil, ErrRecordNotFound
	}
	if err != nil {
		return nil, err
	}

	record.ModerationStatus = ModerationStatus(moderationStatus)

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
	// Get initial moderation status based on configuration
	moderationStatus := GetInitialModerationStatus()

	// Insert the main record without ROR IDs
	_, err := tx.ExecContext(ctx,
		`INSERT INTO records (id, s3_key, sha256, name, metadata, uploader_name, uploader_orcid, moderation_status) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		record.Id, s3Key, record.Sha256, record.Name, record.Metadata,
		record.UploaderName, record.UploaderOrcid, string(moderationStatus),
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

// GetAllByCategoriesPaginated retrieves records filtered by multiple categories with pagination
func (r *PostgresRecordRepository) GetAllByCategoriesPaginated(ctx context.Context, categoryIDs []int64, limit, offset int, orderBy, sortOrder string) ([]Record, int, error) {
	if len(categoryIDs) == 0 {
		return r.GetAllPaginated(ctx, limit, offset, orderBy, sortOrder)
	}

	// Build the query with placeholders for multiple category IDs
	placeholders := make([]string, len(categoryIDs))
	args := make([]interface{}, len(categoryIDs))
	for i, id := range categoryIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	inClause := strings.Join(placeholders, ",")

	// Get total count
	var totalCount int
	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT r.id)
		FROM records r
		JOIN records_categories rc ON r.id = rc.record_id
		WHERE r.moderation_status = 'approved' AND rc.category_id IN (%s)
	`, inClause)

	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Build ORDER BY clause with SQL injection protection
	orderByClause := fmt.Sprintf("ORDER BY r.%s %s", orderBy, strings.ToUpper(sortOrder))

	// Get records
	selectQuery := fmt.Sprintf(`
		SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_name, r.uploader_orcid, r.download_count
		FROM records r
		JOIN records_categories rc ON r.id = rc.record_id
		WHERE r.moderation_status = 'approved' AND rc.category_id IN (%s)
		%s
		LIMIT $%d OFFSET $%d
	`, inClause, orderByClause, len(categoryIDs)+1, len(categoryIDs)+2)

	queryArgs := append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, selectQuery, queryArgs...)
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
			&record.DownloadCount,
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

// GetAllByRorIDsPaginated retrieves records filtered by multiple ROR IDs with pagination
func (r *PostgresRecordRepository) GetAllByRorIDsPaginated(ctx context.Context, rorIDs []string, limit, offset int, orderBy, sortOrder string) ([]Record, int, error) {
	if len(rorIDs) == 0 {
		return []Record{}, 0, nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(rorIDs))
	args := make([]interface{}, len(rorIDs))
	for i, rorID := range rorIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = rorID
	}
	inClause := strings.Join(placeholders, ",")

	// Get total count
	var totalCount int
	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT r.id)
		FROM records r
		JOIN records_ror rr ON r.id = rr.record_id
		WHERE r.moderation_status = 'approved' AND rr.ror IN (%s)
	`, inClause)
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Build ORDER BY clause with SQL injection protection
	orderByClause := fmt.Sprintf("ORDER BY r.%s %s", orderBy, strings.ToUpper(sortOrder))

	// Get records
	// Add limit and offset to args
	args = append(args, limit, offset)
	selectQuery := fmt.Sprintf(`
		SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_name, r.uploader_orcid, r.download_count
		FROM records r
		JOIN records_ror rr ON r.id = rr.record_id
		WHERE r.moderation_status = 'approved' AND rr.ror IN (%s)
		%s
		LIMIT $%d OFFSET $%d
	`, inClause, orderByClause, len(rorIDs)+1, len(rorIDs)+2)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
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
			&record.DownloadCount,
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
		recordRorIds, err := r.rorRepo.GetRecordRorIds(ctx, record.Id)
		if err != nil {
			return nil, 0, err
		}
		record.RorIds = recordRorIds

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return records, totalCount, nil
}

// SearchPaginated retrieves records based on search query with pagination
func (r *PostgresRecordRepository) SearchPaginated(ctx context.Context, query string, categoryID int64, limit, offset int, orderBy, sortOrder string) ([]Record, int, error) {
	var countQuery string
	var sqlQuery string
	var args []interface{}
	var countArgs []interface{}

	// Build ORDER BY clause with SQL injection protection
	orderByClause := fmt.Sprintf("ORDER BY r.%s %s", orderBy, strings.ToUpper(sortOrder))

	if categoryID > 0 {
		// Count query for specific category
		countQuery = `
			SELECT COUNT(DISTINCT r.id)
			FROM records r
			JOIN records_categories rc ON r.id = rc.record_id
			LEFT JOIN records_ror rr ON r.id = rr.record_id
			LEFT JOIN categories c ON rc.category_id = c.id
			WHERE r.moderation_status = 'approved' AND rc.category_id = $1 AND (
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
		sqlQuery = fmt.Sprintf(`
			SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_name, r.uploader_orcid, r.download_count
			FROM records r
			JOIN records_categories rc ON r.id = rc.record_id
			LEFT JOIN records_ror rr ON r.id = rr.record_id
			LEFT JOIN categories c ON rc.category_id = c.id
			WHERE r.moderation_status = 'approved' AND rc.category_id = $1 AND (
				r.name ILIKE $2 OR
				r.metadata::text ILIKE $2 OR
				r.uploader_name ILIKE $2 OR
				r.uploader_orcid ILIKE $2 OR
				rr.ror ILIKE $2 OR
				c.name ILIKE $2
			)
			%s
			LIMIT $3 OFFSET $4
		`, orderByClause)
		args = []interface{}{categoryID, "%" + query + "%", limit, offset}
	} else {
		// Count query for all records
		countQuery = `
			SELECT COUNT(DISTINCT r.id)
			FROM records r
			LEFT JOIN records_ror rr ON r.id = rr.record_id
			LEFT JOIN records_categories rc ON r.id = rc.record_id
			LEFT JOIN categories c ON rc.category_id = c.id
			WHERE r.moderation_status = 'approved' AND (
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
		sqlQuery = fmt.Sprintf(`
			SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_name, r.uploader_orcid, r.download_count
			FROM records r
			LEFT JOIN records_ror rr ON r.id = rr.record_id
			LEFT JOIN records_categories rc ON r.id = rc.record_id
			LEFT JOIN categories c ON rc.category_id = c.id
			WHERE r.moderation_status = 'approved' AND (
				r.name ILIKE $1 OR
				r.metadata::text ILIKE $1 OR
				r.uploader_name ILIKE $1 OR
				r.uploader_orcid ILIKE $1 OR
				rr.ror ILIKE $1 OR
				c.name ILIKE $1
			)
			%s
			LIMIT $2 OFFSET $3
		`, orderByClause)
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
			&record.DownloadCount,
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

// GetAllByOrcidPaginated retrieves records by ORCID with pagination
func (r *PostgresRecordRepository) GetAllByOrcidPaginated(ctx context.Context, orcid string, limit, offset int) ([]Record, int, error) {
	// Get total count
	var totalCount int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM records WHERE uploader_orcid = $1`, orcid).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated records
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, sha256, metadata, created_at, modified_at, uploader_name, uploader_orcid, download_count, moderation_status
		FROM records
		WHERE uploader_orcid = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, orcid, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var rec Record
		var moderationStatus string
		err := rows.Scan(
			&rec.Id,
			&rec.Name,
			&rec.Sha256,
			&rec.Metadata,
			&rec.CreatedAt,
			&rec.ModifiedAt,
			&rec.UploaderName,
			&rec.UploaderOrcid,
			&rec.DownloadCount,
			&moderationStatus,
		)
		if err != nil {
			return nil, 0, err
		}
		rec.ModerationStatus = ModerationStatus(moderationStatus)

		// Fetch categories for this record
		categories, err := r.categoryRepo.GetRecordCategories(ctx, rec.Id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, 0, err
		}
		rec.Categories = categories

		// Fetch ROR IDs for this record
		rorIds, err := r.rorRepo.GetRecordRorIds(ctx, rec.Id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, 0, err
		}
		rec.RorIds = rorIds

		records = append(records, rec)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return records, totalCount, nil
}

// IncrementDownloadCount increments the download count for a record and returns the new count
func (r *PostgresRecordRepository) IncrementDownloadCount(ctx context.Context, id string) (int, error) {
	var newCount int
	err := r.db.QueryRowContext(ctx, `
		UPDATE records 
		SET download_count = download_count + 1 
		WHERE id = $1 
		RETURNING download_count
	`, id).Scan(&newCount)

	if err == sql.ErrNoRows {
		return 0, ErrRecordNotFound
	}
	if err != nil {
		return 0, err
	}

	return newCount, nil
}
