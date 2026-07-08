package credential

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"
)

var ErrNotFound = errors.New("credential not found")

type Repository struct {
	db         *sql.DB
	driverName string
}

func NewRepository(db *sql.DB, driverName string) Repository {
	return Repository{db: db, driverName: driverName}
}

func (r Repository) Create(ctx context.Context, credential Credential) error {
	query := `
INSERT INTO credentials (
	id, type, name, encrypted_value, encryption_version, fingerprint, created_by, created_at, updated_at
) VALUES (` + placeholders(r.driverName, 9) + `)`

	_, err := r.db.ExecContext(
		ctx,
		query,
		credential.ID,
		credential.Type,
		credential.Name,
		credential.EncryptedValue,
		credential.EncryptionVersion,
		nullString(credential.Fingerprint),
		nullString(credential.CreatedBy),
		formatTime(credential.CreatedAt),
		formatTime(credential.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("create credential: %w", err)
	}
	return nil
}

func (r Repository) UpdateValue(ctx context.Context, credential Credential) error {
	query := `
UPDATE credentials
SET name = ` + placeholder(r.driverName, 1) + `,
    encrypted_value = ` + placeholder(r.driverName, 2) + `,
    encryption_version = ` + placeholder(r.driverName, 3) + `,
    fingerprint = ` + placeholder(r.driverName, 4) + `,
    updated_at = ` + placeholder(r.driverName, 5) + `
WHERE id = ` + placeholder(r.driverName, 6)

	result, err := r.db.ExecContext(
		ctx,
		query,
		credential.Name,
		credential.EncryptedValue,
		credential.EncryptionVersion,
		nullString(credential.Fingerprint),
		formatTime(credential.UpdatedAt),
		credential.ID,
	)
	if err != nil {
		return fmt.Errorf("update credential: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) FindByID(ctx context.Context, id string) (Credential, error) {
	query := `
SELECT id, type, name, encrypted_value, encryption_version, fingerprint, created_by, created_at, updated_at
FROM credentials
WHERE id = ` + placeholder(r.driverName, 1)

	var credential Credential
	var fingerprint sql.NullString
	var createdBy sql.NullString
	var createdAt string
	var updatedAt string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&credential.ID,
		&credential.Type,
		&credential.Name,
		&credential.EncryptedValue,
		&credential.EncryptionVersion,
		&fingerprint,
		&createdBy,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Credential{}, ErrNotFound
		}
		return Credential{}, fmt.Errorf("find credential: %w", err)
	}

	credential.Fingerprint = fingerprint.String
	credential.CreatedBy = createdBy.String
	credential.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	credential.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return credential, nil
}

func requireRowsAffected(result sql.Result) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func placeholder(driverName string, position int) string {
	if driverName == "pgx" {
		return "$" + strconv.Itoa(position)
	}
	return "?"
}

func placeholders(driverName string, count int) string {
	values := make([]byte, 0, count*4)
	for i := 1; i <= count; i++ {
		if i > 1 {
			values = append(values, ',', ' ')
		}
		values = append(values, placeholder(driverName, i)...)
	}
	return string(values)
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
