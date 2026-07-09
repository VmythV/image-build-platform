package registry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrNotFound = errors.New("registry not found")

type Repository struct {
	db         *sql.DB
	driverName string
}

func NewRepository(db *sql.DB, driverName string) Repository {
	return Repository{db: db, driverName: driverName}
}

func (r Repository) List(ctx context.Context, filter ListFilter) ([]Registry, int, error) {
	where, args := r.filterWhere(filter)
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM registries r "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count registries: %w", err)
	}

	page := normalizePage(filter.Page)
	pageSize := normalizePageSize(filter.Size)
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	query := selectRegistrySQL + where + `
ORDER BY r.created_at DESC
LIMIT ` + placeholder(r.driverName, len(args)-1) + ` OFFSET ` + placeholder(r.driverName, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list registries: %w", err)
	}
	defer rows.Close()

	registries := make([]Registry, 0)
	for rows.Next() {
		registry, err := scanRegistry(rows)
		if err != nil {
			return nil, 0, err
		}
		registries = append(registries, registry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate registries: %w", err)
	}

	return registries, total, nil
}

func (r Repository) FindByID(ctx context.Context, id string) (Registry, error) {
	query := selectRegistrySQL + `WHERE r.id = ` + placeholder(r.driverName, 1) + ` AND r.deleted_at IS NULL`
	row := r.db.QueryRowContext(ctx, query, id)
	registry, err := scanRegistry(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Registry{}, ErrNotFound
		}
		return Registry{}, err
	}
	return registry, nil
}

func (r Repository) FindDefaultPush(ctx context.Context) (Registry, error) {
	query := selectRegistrySQL + `
WHERE r.is_default_push = ` + placeholder(r.driverName, 1) + `
  AND r.allow_push = ` + placeholder(r.driverName, 2) + `
  AND r.deleted_at IS NULL
ORDER BY r.created_at DESC
LIMIT 1`
	registry, err := scanRegistry(r.db.QueryRowContext(ctx, query, boolInt(true), boolInt(true)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Registry{}, ErrNotFound
		}
		return Registry{}, err
	}
	return registry, nil
}

func (r Repository) FindDefaultPull(ctx context.Context) (Registry, error) {
	query := selectRegistrySQL + `
WHERE r.is_default_pull = ` + placeholder(r.driverName, 1) + `
  AND r.allow_pull = ` + placeholder(r.driverName, 2) + `
  AND r.deleted_at IS NULL
ORDER BY r.created_at DESC
LIMIT 1`
	registry, err := scanRegistry(r.db.QueryRowContext(ctx, query, boolInt(true), boolInt(true)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Registry{}, ErrNotFound
		}
		return Registry{}, err
	}
	return registry, nil
}

func (r Repository) Create(ctx context.Context, registry Registry) error {
	query := `
INSERT INTO registries (
	id, name, type, endpoint, namespace, region, credential_id, allow_pull, allow_push, is_default_pull,
	is_default_push, tls_verify, insecure_http, status, last_checked_at, last_check_result, last_error,
	created_by, created_at, updated_at, deleted_at
) VALUES (` + placeholders(r.driverName, 21) + `)`

	_, err := r.db.ExecContext(
		ctx,
		query,
		registry.ID,
		registry.Name,
		registry.Type,
		registry.Endpoint,
		nullString(registry.Namespace),
		nullString(registry.Region),
		nullString(registry.CredentialID),
		boolInt(registry.AllowPull),
		boolInt(registry.AllowPush),
		boolInt(registry.IsDefaultPull),
		boolInt(registry.IsDefaultPush),
		boolInt(registry.TLSVerify),
		boolInt(registry.InsecureHTTP),
		registry.Status,
		nullTime(registry.LastCheckedAt),
		nullString(registry.LastCheckResult),
		nullString(registry.LastError),
		nullString(registry.CreatedBy),
		formatTime(registry.CreatedAt),
		formatTime(registry.UpdatedAt),
		nil,
	)
	if err != nil {
		return fmt.Errorf("create registry: %w", err)
	}
	return nil
}

func (r Repository) Update(ctx context.Context, registry Registry) error {
	query := `
UPDATE registries
SET name = ` + placeholder(r.driverName, 1) + `,
    type = ` + placeholder(r.driverName, 2) + `,
    endpoint = ` + placeholder(r.driverName, 3) + `,
    namespace = ` + placeholder(r.driverName, 4) + `,
    region = ` + placeholder(r.driverName, 5) + `,
    credential_id = ` + placeholder(r.driverName, 6) + `,
    allow_pull = ` + placeholder(r.driverName, 7) + `,
    allow_push = ` + placeholder(r.driverName, 8) + `,
    is_default_pull = ` + placeholder(r.driverName, 9) + `,
    is_default_push = ` + placeholder(r.driverName, 10) + `,
    tls_verify = ` + placeholder(r.driverName, 11) + `,
    insecure_http = ` + placeholder(r.driverName, 12) + `,
    status = ` + placeholder(r.driverName, 13) + `,
    last_checked_at = NULL,
    last_check_result = NULL,
    last_error = NULL,
    updated_at = ` + placeholder(r.driverName, 14) + `
WHERE id = ` + placeholder(r.driverName, 15) + ` AND deleted_at IS NULL`

	result, err := r.db.ExecContext(
		ctx,
		query,
		registry.Name,
		registry.Type,
		registry.Endpoint,
		nullString(registry.Namespace),
		nullString(registry.Region),
		nullString(registry.CredentialID),
		boolInt(registry.AllowPull),
		boolInt(registry.AllowPush),
		boolInt(registry.IsDefaultPull),
		boolInt(registry.IsDefaultPush),
		boolInt(registry.TLSVerify),
		boolInt(registry.InsecureHTTP),
		registry.Status,
		formatTime(registry.UpdatedAt),
		registry.ID,
	)
	if err != nil {
		return fmt.Errorf("update registry: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) UpdateCheckResult(ctx context.Context, registryID string, result CheckResult, checkedAt time.Time, rawResult string) error {
	query := `
UPDATE registries
SET status = ` + placeholder(r.driverName, 1) + `,
    last_checked_at = ` + placeholder(r.driverName, 2) + `,
    last_check_result = ` + placeholder(r.driverName, 3) + `,
    last_error = ` + placeholder(r.driverName, 4) + `,
    updated_at = ` + placeholder(r.driverName, 5) + `
WHERE id = ` + placeholder(r.driverName, 6) + ` AND deleted_at IS NULL`

	var errorMessage string
	if result.Error != nil {
		errorMessage = *result.Error
	}

	execResult, err := r.db.ExecContext(ctx, query, result.Status, formatTime(checkedAt), rawResult, nullString(errorMessage), formatTime(checkedAt), registryID)
	if err != nil {
		return fmt.Errorf("update registry check result: %w", err)
	}
	return requireRowsAffected(execResult)
}

func (r Repository) SetStatus(ctx context.Context, id string, status string, updatedAt time.Time) error {
	query := `
UPDATE registries
SET status = ` + placeholder(r.driverName, 1) + `,
    updated_at = ` + placeholder(r.driverName, 2) + `
WHERE id = ` + placeholder(r.driverName, 3) + ` AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, status, formatTime(updatedAt), id)
	if err != nil {
		return fmt.Errorf("set registry status: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) Delete(ctx context.Context, id string, deletedAt time.Time) error {
	query := `
UPDATE registries
SET deleted_at = ` + placeholder(r.driverName, 1) + `,
    updated_at = ` + placeholder(r.driverName, 2) + `
WHERE id = ` + placeholder(r.driverName, 3) + ` AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, formatTime(deletedAt), formatTime(deletedAt), id)
	if err != nil {
		return fmt.Errorf("delete registry: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) ClearDefaultPull(ctx context.Context, exceptID string) error {
	return r.clearDefault(ctx, "is_default_pull", exceptID)
}

func (r Repository) ClearDefaultPush(ctx context.Context, exceptID string) error {
	return r.clearDefault(ctx, "is_default_push", exceptID)
}

func (r Repository) clearDefault(ctx context.Context, column string, exceptID string) error {
	query := "UPDATE registries SET " + column + " = " + placeholder(r.driverName, 1) + " WHERE id != " + placeholder(r.driverName, 2) + " AND deleted_at IS NULL"
	if _, err := r.db.ExecContext(ctx, query, boolInt(false), exceptID); err != nil {
		return fmt.Errorf("clear registry default %s: %w", column, err)
	}
	return nil
}

func (r Repository) filterWhere(filter ListFilter) (string, []any) {
	clauses := []string{"r.deleted_at IS NULL"}
	args := make([]any, 0, 2)

	if strings.TrimSpace(filter.Status) != "" {
		args = append(args, strings.TrimSpace(filter.Status))
		clauses = append(clauses, "r.status = "+placeholder(r.driverName, len(args)))
	}
	if strings.TrimSpace(filter.Type) != "" {
		args = append(args, strings.TrimSpace(filter.Type))
		clauses = append(clauses, "r.type = "+placeholder(r.driverName, len(args)))
	}

	return "WHERE " + strings.Join(clauses, " AND "), args
}

const selectRegistrySQL = `
SELECT r.id, r.name, r.type, r.endpoint, r.namespace, r.region, r.credential_id, c.name, c.fingerprint,
       r.allow_pull, r.allow_push, r.is_default_pull, r.is_default_push, r.tls_verify, r.insecure_http,
       r.status, r.last_checked_at, r.last_check_result, r.last_error, r.created_by, r.created_at, r.updated_at
FROM registries r
LEFT JOIN credentials c ON c.id = r.credential_id
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRegistry(row rowScanner) (Registry, error) {
	var registry Registry
	var namespace sql.NullString
	var region sql.NullString
	var credentialID sql.NullString
	var credentialName sql.NullString
	var credentialFingerprint sql.NullString
	var lastCheckedAt sql.NullString
	var lastCheckResult sql.NullString
	var lastError sql.NullString
	var createdBy sql.NullString
	var createdAt string
	var updatedAt string
	var allowPull, allowPush, isDefaultPull, isDefaultPush, tlsVerify, insecureHTTP int

	err := row.Scan(
		&registry.ID,
		&registry.Name,
		&registry.Type,
		&registry.Endpoint,
		&namespace,
		&region,
		&credentialID,
		&credentialName,
		&credentialFingerprint,
		&allowPull,
		&allowPush,
		&isDefaultPull,
		&isDefaultPush,
		&tlsVerify,
		&insecureHTTP,
		&registry.Status,
		&lastCheckedAt,
		&lastCheckResult,
		&lastError,
		&createdBy,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return Registry{}, err
	}

	registry.Namespace = namespace.String
	registry.Region = region.String
	registry.CredentialID = credentialID.String
	registry.CredentialName = credentialName.String
	registry.CredentialFingerprint = credentialFingerprint.String
	registry.AllowPull = allowPull != 0
	registry.AllowPush = allowPush != 0
	registry.IsDefaultPull = isDefaultPull != 0
	registry.IsDefaultPush = isDefaultPush != 0
	registry.TLSVerify = tlsVerify != 0
	registry.InsecureHTTP = insecureHTTP != 0
	registry.LastCheckResult = lastCheckResult.String
	registry.LastError = lastError.String
	registry.CreatedBy = createdBy.String
	registry.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	registry.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if lastCheckedAt.Valid {
		parsed, err := time.Parse(time.RFC3339, lastCheckedAt.String)
		if err == nil {
			registry.LastCheckedAt = &parsed
		}
	}
	return registry, nil
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

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizePageSize(pageSize int) int {
	if pageSize < 1 {
		return 20
	}
	if pageSize > 100 {
		return 100
	}
	return pageSize
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

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
