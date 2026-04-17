package records

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

var (
	ErrRecordNotFound = errors.New("record not found")
)

// RecordRepository defines the interface for record data operations
type RecordRepository interface {
	GetAllPaginated(ctx context.Context, limit, offset int, orderBy, sortOrder string, filters map[string]interface{}) ([]Record, int, error)
	GetAllByCategoriesPaginated(ctx context.Context, categoryIDs []int64, limit, offset int, orderBy, sortOrder string, filters map[string]interface{}) ([]Record, int, error)
	GetAllByRorIDsPaginated(ctx context.Context, rorIDs []string, limit, offset int, orderBy, sortOrder string, filters map[string]interface{}) ([]Record, int, error)
	GetAllByOrcidPaginated(ctx context.Context, orcid string, limit, offset int) ([]Record, int, error)
	SearchPaginated(ctx context.Context, query string, categoryID int64, limit, offset int, orderBy, sortOrder string, filters map[string]interface{}) ([]Record, int, error)
	SearchPaginatedWithRorIDs(ctx context.Context, query string, categoryID int64, rorIDs []string, limit, offset int, orderBy, sortOrder string, filters map[string]interface{}) ([]Record, int, error)
	GetByID(ctx context.Context, id string) (*Record, error)
	Create(ctx context.Context, tx *sql.Tx, record *Record, s3Key string) error
	Update(ctx context.Context, tx *sql.Tx, record *Record) error
	Delete(ctx context.Context, tx *sql.Tx, id string) error
	Archive(ctx context.Context, id string, reason string) error
	Unarchive(ctx context.Context, id string) error
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

// buildFilterClause builds WHERE clause conditions and args from filter map
func buildFilterClause(filters map[string]interface{}, startArgIndex int) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argIndex := startArgIndex

	if filterName, ok := filters["name"].(string); ok && filterName != "" {
		filterType, _ := filters["nameType"].(string)
		switch filterType {
		case "equals":
			conditions = append(conditions, fmt.Sprintf("r.name = $%d", argIndex))
			args = append(args, filterName)
		case "notEqual":
			conditions = append(conditions, fmt.Sprintf("r.name != $%d", argIndex))
			args = append(args, filterName)
		case "startsWith":
			conditions = append(conditions, fmt.Sprintf("r.name ILIKE $%d", argIndex))
			args = append(args, filterName+"%")
		case "endsWith":
			conditions = append(conditions, fmt.Sprintf("r.name ILIKE $%d", argIndex))
			args = append(args, "%"+filterName)
		default: // contains
			conditions = append(conditions, fmt.Sprintf("r.name ILIKE $%d", argIndex))
			args = append(args, "%"+filterName+"%")
		}
		argIndex++
	}

	if filterAuthor, ok := filters["author"].(string); ok && filterAuthor != "" {
		filterType, _ := filters["authorType"].(string)
		switch filterType {
		case "equals":
			conditions = append(conditions, fmt.Sprintf("r.uploader_name = $%d", argIndex))
			args = append(args, filterAuthor)
		case "notEqual":
			conditions = append(conditions, fmt.Sprintf("r.uploader_name != $%d", argIndex))
			args = append(args, filterAuthor)
		case "startsWith":
			conditions = append(conditions, fmt.Sprintf("r.uploader_name ILIKE $%d", argIndex))
			args = append(args, filterAuthor+"%")
		case "endsWith":
			conditions = append(conditions, fmt.Sprintf("r.uploader_name ILIKE $%d", argIndex))
			args = append(args, "%"+filterAuthor)
		default: // contains
			conditions = append(conditions, fmt.Sprintf("r.uploader_name ILIKE $%d", argIndex))
			args = append(args, "%"+filterAuthor+"%")
		}
		argIndex++
	}

	if filterDownloads, ok := filters["downloads"].(int); ok {
		filterType, _ := filters["downloadsType"].(string)
		switch filterType {
		case "equals":
			conditions = append(conditions, fmt.Sprintf("r.download_count = $%d", argIndex))
			args = append(args, filterDownloads)
			argIndex++
		case "notEqual":
			conditions = append(conditions, fmt.Sprintf("r.download_count != $%d", argIndex))
			args = append(args, filterDownloads)
			argIndex++
		case "lessThan":
			conditions = append(conditions, fmt.Sprintf("r.download_count < $%d", argIndex))
			args = append(args, filterDownloads)
			argIndex++
		case "lessThanOrEqual":
			conditions = append(conditions, fmt.Sprintf("r.download_count <= $%d", argIndex))
			args = append(args, filterDownloads)
			argIndex++
		case "greaterThan":
			conditions = append(conditions, fmt.Sprintf("r.download_count > $%d", argIndex))
			args = append(args, filterDownloads)
			argIndex++
		case "greaterThanOrEqual":
			conditions = append(conditions, fmt.Sprintf("r.download_count >= $%d", argIndex))
			args = append(args, filterDownloads)
			argIndex++
		case "inRange":
			conditions = append(conditions, fmt.Sprintf("r.download_count >= $%d", argIndex))
			args = append(args, filterDownloads)
			argIndex++
			if filterDownloadsTo, ok := filters["downloadsTo"].(int); ok {
				conditions = append(conditions, fmt.Sprintf("r.download_count <= $%d", argIndex))
				args = append(args, filterDownloadsTo)
				argIndex++
			}
		}
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " AND " + strings.Join(conditions, " AND ")
	}

	return whereClause, args
}

// GetAllPaginated retrieves records with pagination
func (r *PostgresRecordRepository) GetAllPaginated(ctx context.Context, limit, offset int, orderBy, sortOrder string, filters map[string]interface{}) ([]Record, int, error) {
	// Build filter clause
	filterClause, filterArgs := buildFilterClause(filters, 1)

	// Get total count - only approved and non-archived records for public view
	var totalCount int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM records r WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL%s`, filterClause)
	err := r.db.QueryRowContext(ctx, countQuery, filterArgs...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Build ORDER BY clause with SQL injection protection
	orderByClause := fmt.Sprintf("ORDER BY %s %s", orderBy, strings.ToUpper(sortOrder))

	// Build query with filters
	query := fmt.Sprintf(`
		SELECT id, sha256, name, metadata, created_at, modified_at, uploader_name, uploader_orcid, download_count, license
		FROM records r
		WHERE moderation_status = 'approved' AND r.archived_at IS NULL%s
		%s
		LIMIT $%d OFFSET $%d
	`, filterClause, orderByClause, len(filterArgs)+1, len(filterArgs)+2)

	queryArgs := append(filterArgs, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
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
			&record.License,
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
	var archiveReason sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, sha256, name, metadata, created_at, modified_at, uploader_name, uploader_orcid, download_count, moderation_status, license, archived_at, archive_reason
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
		&record.License,
		&record.ArchivedAt,
		&archiveReason,
	)

	if err == sql.ErrNoRows {
		return nil, ErrRecordNotFound
	}
	if err != nil {
		return nil, err
	}

	record.ModerationStatus = ModerationStatus(moderationStatus)
	if archiveReason.Valid {
		record.ArchiveReason = archiveReason.String
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
	// Get initial moderation status based on configuration
	moderationStatus := GetInitialModerationStatus()

	// Default license if not provided
	if record.License == "" {
		record.License = "CC-BY-4.0"
	}

	// Insert the main record without ROR IDs
	_, err := tx.ExecContext(ctx,
		`INSERT INTO records (id, s3_key, sha256, name, metadata, uploader_name, uploader_orcid, moderation_status, license) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		record.Id, s3Key, record.Sha256, record.Name, record.Metadata,
		record.UploaderName, record.UploaderOrcid, string(moderationStatus), record.License,
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
func (r *PostgresRecordRepository) GetAllByCategoriesPaginated(ctx context.Context, categoryIDs []int64, limit, offset int, orderBy, sortOrder string, filters map[string]interface{}) ([]Record, int, error) {
	if len(categoryIDs) == 0 {
		return r.GetAllPaginated(ctx, limit, offset, orderBy, sortOrder, filters)
	}

	// Build the query with placeholders for multiple category IDs
	placeholders := make([]string, len(categoryIDs))
	args := make([]interface{}, len(categoryIDs))
	for i, id := range categoryIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	inClause := strings.Join(placeholders, ",")

	// Build filter clause
	filterClause, filterArgs := buildFilterClause(filters, len(categoryIDs)+1)

	// Get total count
	var totalCount int
	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT r.id)
		FROM records r
		JOIN records_categories rc ON r.id = rc.record_id
		WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND rc.category_id IN (%s)%s
	`, inClause, filterClause)

	countArgs := append(args, filterArgs...)
	err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount)
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
		WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND rc.category_id IN (%s)%s
		%s
		LIMIT $%d OFFSET $%d
	`, inClause, filterClause, orderByClause, len(countArgs)+1, len(countArgs)+2)

	queryArgs := append(countArgs, limit, offset)
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
func (r *PostgresRecordRepository) GetAllByRorIDsPaginated(ctx context.Context, rorIDs []string, limit, offset int, orderBy, sortOrder string, filters map[string]interface{}) ([]Record, int, error) {
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

	// Build filter clause
	filterClause, filterArgs := buildFilterClause(filters, len(rorIDs)+1)

	// Get total count
	var totalCount int
	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT r.id)
		FROM records r
		JOIN records_ror rr ON r.id = rr.record_id
		WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND rr.ror IN (%s)%s
	`, inClause, filterClause)

	countArgs := append(args, filterArgs...)
	err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Build ORDER BY clause with SQL injection protection
	orderByClause := fmt.Sprintf("ORDER BY r.%s %s", orderBy, strings.ToUpper(sortOrder))

	// Get records
	selectQuery := fmt.Sprintf(`
		SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_name, r.uploader_orcid, r.download_count
		FROM records r
		JOIN records_ror rr ON r.id = rr.record_id
		WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND rr.ror IN (%s)%s
		%s
		LIMIT $%d OFFSET $%d
	`, inClause, filterClause, orderByClause, len(countArgs)+1, len(countArgs)+2)

	queryArgs := append(countArgs, limit, offset)
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
func (r *PostgresRecordRepository) SearchPaginated(ctx context.Context, query string, categoryID int64, limit, offset int, orderBy, sortOrder string, filters map[string]interface{}) ([]Record, int, error) {
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
			WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND rc.category_id = $1 AND (
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
			WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND rc.category_id = $1 AND (
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
			WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND (
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
			WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND (
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

// SearchPaginatedWithRorIDs retrieves records based on search query with pagination and organization name matching
// This method extends SearchPaginated by also searching for records associated with specific ROR IDs (from organization name matches)
func (r *PostgresRecordRepository) SearchPaginatedWithRorIDs(ctx context.Context, query string, categoryID int64, rorIDs []string, limit, offset int, orderBy, sortOrder string, filters map[string]interface{}) ([]Record, int, error) {
	var countQuery string
	var sqlQuery string
	var args []interface{}
	var countArgs []interface{}

	// Build ORDER BY clause with SQL injection protection
	orderByClause := fmt.Sprintf("ORDER BY r.%s %s", orderBy, strings.ToUpper(sortOrder))

	if categoryID > 0 {
		// Count query for specific category
		if len(rorIDs) > 0 {
			countQuery = `
				SELECT COUNT(DISTINCT r.id)
				FROM records r
				JOIN records_categories rc ON r.id = rc.record_id
				LEFT JOIN records_ror rr ON r.id = rr.record_id
				LEFT JOIN categories c ON rc.category_id = c.id
				WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND rc.category_id = $1 AND (
					r.name ILIKE $2 OR
					r.metadata::text ILIKE $2 OR
					r.uploader_name ILIKE $2 OR
					r.uploader_orcid ILIKE $2 OR
					rr.ror ILIKE $2 OR
					c.name ILIKE $2 OR
					rr.ror = ANY($3)
				)
			`
			countArgs = []interface{}{categoryID, "%" + query + "%", pq.Array(rorIDs)}
		} else {
			countQuery = `
				SELECT COUNT(DISTINCT r.id)
				FROM records r
				JOIN records_categories rc ON r.id = rc.record_id
				LEFT JOIN records_ror rr ON r.id = rr.record_id
				LEFT JOIN categories c ON rc.category_id = c.id
				WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND rc.category_id = $1 AND (
					r.name ILIKE $2 OR
					r.metadata::text ILIKE $2 OR
					r.uploader_name ILIKE $2 OR
					r.uploader_orcid ILIKE $2 OR
					rr.ror ILIKE $2 OR
					c.name ILIKE $2
				)
			`
			countArgs = []interface{}{categoryID, "%" + query + "%"}
		}

		// Search within a specific category
		if len(rorIDs) > 0 {
			sqlQuery = fmt.Sprintf(`
				SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_name, r.uploader_orcid, r.download_count
				FROM records r
				JOIN records_categories rc ON r.id = rc.record_id
				LEFT JOIN records_ror rr ON r.id = rr.record_id
				LEFT JOIN categories c ON rc.category_id = c.id
				WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND rc.category_id = $1 AND (
					r.name ILIKE $2 OR
					r.metadata::text ILIKE $2 OR
					r.uploader_name ILIKE $2 OR
					r.uploader_orcid ILIKE $2 OR
					rr.ror ILIKE $2 OR
					c.name ILIKE $2 OR
					rr.ror = ANY($3)
				)
				%s
				LIMIT $4 OFFSET $5
			`, orderByClause)
			args = []interface{}{categoryID, "%" + query + "%", pq.Array(rorIDs), limit, offset}
		} else {
			sqlQuery = fmt.Sprintf(`
				SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_name, r.uploader_orcid, r.download_count
				FROM records r
				JOIN records_categories rc ON r.id = rc.record_id
				LEFT JOIN records_ror rr ON r.id = rr.record_id
				LEFT JOIN categories c ON rc.category_id = c.id
				WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND rc.category_id = $1 AND (
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
		}
	} else {
		// Count query for all records
		if len(rorIDs) > 0 {
			countQuery = `
				SELECT COUNT(DISTINCT r.id)
				FROM records r
				LEFT JOIN records_ror rr ON r.id = rr.record_id
				LEFT JOIN records_categories rc ON r.id = rc.record_id
				LEFT JOIN categories c ON rc.category_id = c.id
				WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND (
					r.name ILIKE $1 OR
					r.metadata::text ILIKE $1 OR
					r.uploader_name ILIKE $1 OR
					r.uploader_orcid ILIKE $1 OR
					rr.ror ILIKE $1 OR
					c.name ILIKE $1 OR
					rr.ror = ANY($2)
				)
			`
			countArgs = []interface{}{"%" + query + "%", pq.Array(rorIDs)}
		} else {
			countQuery = `
				SELECT COUNT(DISTINCT r.id)
				FROM records r
				LEFT JOIN records_ror rr ON r.id = rr.record_id
				LEFT JOIN records_categories rc ON r.id = rc.record_id
				LEFT JOIN categories c ON rc.category_id = c.id
				WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND (
					r.name ILIKE $1 OR
					r.metadata::text ILIKE $1 OR
					r.uploader_name ILIKE $1 OR
					r.uploader_orcid ILIKE $1 OR
					rr.ror ILIKE $1 OR
					c.name ILIKE $1
				)
			`
			countArgs = []interface{}{"%" + query + "%"}
		}

		// Search across all records
		if len(rorIDs) > 0 {
			sqlQuery = fmt.Sprintf(`
				SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_name, r.uploader_orcid, r.download_count
				FROM records r
				LEFT JOIN records_ror rr ON r.id = rr.record_id
				LEFT JOIN records_categories rc ON r.id = rc.record_id
				LEFT JOIN categories c ON rc.category_id = c.id
				WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND (
					r.name ILIKE $1 OR
					r.metadata::text ILIKE $1 OR
					r.uploader_name ILIKE $1 OR
					r.uploader_orcid ILIKE $1 OR
					rr.ror ILIKE $1 OR
					c.name ILIKE $1 OR
					rr.ror = ANY($2)
				)
				%s
				LIMIT $3 OFFSET $4
			`, orderByClause)
			args = []interface{}{"%" + query + "%", pq.Array(rorIDs), limit, offset}
		} else {
			sqlQuery = fmt.Sprintf(`
				SELECT DISTINCT r.id, r.sha256, r.name, r.metadata, r.created_at, r.modified_at, r.uploader_name, r.uploader_orcid, r.download_count
				FROM records r
				LEFT JOIN records_ror rr ON r.id = rr.record_id
				LEFT JOIN records_categories rc ON r.id = rc.record_id
				LEFT JOIN categories c ON rc.category_id = c.id
				WHERE r.moderation_status = 'approved' AND r.archived_at IS NULL AND (
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

// Archive soft-deletes a record by setting archived_at and archive_reason
func (r *PostgresRecordRepository) Archive(ctx context.Context, id string, reason string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE records SET archived_at = now(), archive_reason = $2 WHERE id = $1`,
		id, reason,
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

// Unarchive restores an archived record by clearing archived_at and archive_reason
func (r *PostgresRecordRepository) Unarchive(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE records SET archived_at = NULL, archive_reason = NULL WHERE id = $1`,
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

	// Get paginated records with pending version check
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			r.id, r.name, r.sha256, r.metadata, r.created_at, r.modified_at,
			r.uploader_name, r.uploader_orcid, r.download_count, r.moderation_status,
			CASE
				WHEN EXISTS (
					SELECT 1 FROM record_history rh
					WHERE rh.record_id = r.id
					AND rh.moderation_status = 'pending'
					AND rh.change_type = 'PENDING_VERSION'
				) THEN 'pending'
				ELSE r.moderation_status
			END as effective_status,
			r.archived_at
		FROM records r
		WHERE r.uploader_orcid = $1
		ORDER BY r.created_at DESC
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
		var effectiveStatus string
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
			&effectiveStatus,
			&rec.ArchivedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		// Use effective status which considers pending versions
		rec.ModerationStatus = ModerationStatus(effectiveStatus)

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
