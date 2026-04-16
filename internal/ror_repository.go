package app

import (
	"context"
	"database/sql"
)

// RorRepository defines the interface for ROR data operations
type RorRepository interface {
	GetRecordRorIds(ctx context.Context, recordId string) ([]string, error)
	GetAllUniqueRorIds(ctx context.Context) ([]string, error)
	AssociateRorWithRecord(ctx context.Context, tx *sql.Tx, recordId string, rorId string) error
	RemoveAllRorAssociations(ctx context.Context, tx *sql.Tx, recordId string) error
}

// PostgresRorRepository implements RorRepository using PostgreSQL
type PostgresRorRepository struct {
	db *sql.DB
}

// NewPostgresRorRepository creates a new PostgreSQL ROR repository
func NewPostgresRorRepository(db *sql.DB) *PostgresRorRepository {
	return &PostgresRorRepository{db: db}
}

// GetRecordRorIds retrieves all ROR IDs associated with a record
func (r *PostgresRorRepository) GetRecordRorIds(ctx context.Context, recordId string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ror 
		FROM records_ror 
		WHERE record_id = $1
		ORDER BY created_at ASC
	`, recordId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rorIds []string
	for rows.Next() {
		var rorId string
		if err := rows.Scan(&rorId); err != nil {
			return nil, err
		}
		rorIds = append(rorIds, rorId)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return rorIds, nil
}

// GetAllUniqueRorIds retrieves all unique ROR IDs from the database
func (r *PostgresRorRepository) GetAllUniqueRorIds(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ror 
		FROM records_ror 
		ORDER BY ror
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rorIds []string
	for rows.Next() {
		var rorId string
		if err := rows.Scan(&rorId); err != nil {
			return nil, err
		}
		rorIds = append(rorIds, rorId)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return rorIds, nil
}

// AssociateRorWithRecord creates an association between a record and a ROR ID
func (r *PostgresRorRepository) AssociateRorWithRecord(ctx context.Context, tx *sql.Tx, recordId string, rorId string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO records_ror (record_id, ror) 
		VALUES ($1, $2)
		ON CONFLICT (record_id, ror) DO NOTHING
	`, recordId, rorId)
	return err
}

// RemoveAllRorAssociations removes all ROR associations for a record
func (r *PostgresRorRepository) RemoveAllRorAssociations(ctx context.Context, tx *sql.Tx, recordId string) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM records_ror WHERE record_id = $1
	`, recordId)
	return err
}
