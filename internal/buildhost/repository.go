package buildhost

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrNotFound = errors.New("build host not found")

type Repository struct {
	db         *sql.DB
	driverName string
}

func NewRepository(db *sql.DB, driverName string) Repository {
	return Repository{db: db, driverName: driverName}
}

func (r Repository) CountLocalHosts(ctx context.Context) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM build_hosts WHERE connection_type = " + placeholder(r.driverName, 1) + " AND deleted_at IS NULL"
	if err := r.db.QueryRowContext(ctx, query, ConnectionLocalDocker).Scan(&count); err != nil {
		return 0, fmt.Errorf("count local build hosts: %w", err)
	}
	return count, nil
}

func (r Repository) List(ctx context.Context, filter ListFilter) ([]BuildHost, int, error) {
	where, args := r.filterWhere(filter)
	countQuery := "SELECT COUNT(*) FROM build_hosts h " + where

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count build hosts: %w", err)
	}

	page := normalizePage(filter.Page)
	pageSize := normalizePageSize(filter.PageSize)
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	query := selectHostSQL + where + `
ORDER BY h.created_at DESC
LIMIT ` + placeholder(r.driverName, len(args)-1) + ` OFFSET ` + placeholder(r.driverName, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list build hosts: %w", err)
	}
	defer rows.Close()

	hosts := make([]BuildHost, 0)
	for rows.Next() {
		host, err := scanHost(rows)
		if err != nil {
			return nil, 0, err
		}
		hosts = append(hosts, host)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate build hosts: %w", err)
	}

	return hosts, total, nil
}

func (r Repository) FindByID(ctx context.Context, id string) (BuildHost, error) {
	query := selectHostSQL + `
WHERE h.id = ` + placeholder(r.driverName, 1) + ` AND h.deleted_at IS NULL`

	row := r.db.QueryRowContext(ctx, query, id)
	host, err := scanHost(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BuildHost{}, ErrNotFound
		}
		return BuildHost{}, err
	}
	return host, nil
}

func (r Repository) Create(ctx context.Context, host BuildHost) error {
	labels, err := encodeLabels(host.Labels)
	if err != nil {
		return err
	}

	query := `
INSERT INTO build_hosts (
	id, name, connection_type, address, port, username, credential_id, docker_endpoint, docker_command,
	architecture, os, docker_version, buildkit_supported, labels, max_concurrency, current_running,
	status, last_checked_at, last_check_result, last_error, created_by, created_at, updated_at, deleted_at
) VALUES (` + placeholders(r.driverName, 24) + `)`

	_, err = r.db.ExecContext(
		ctx,
		query,
		host.ID,
		host.Name,
		host.ConnectionType,
		nullString(host.Address),
		nullInt(host.Port),
		nullString(host.Username),
		nullString(host.CredentialID),
		nullString(host.DockerEndpoint),
		nullString(host.DockerCommand),
		nullString(host.Architecture),
		nullString(host.OS),
		nullString(host.DockerVersion),
		host.BuildkitSupported,
		labels,
		host.MaxConcurrency,
		host.CurrentRunning,
		host.Status,
		nullTime(host.LastCheckedAt),
		nullString(host.LastCheckResult),
		nullString(host.LastError),
		nullString(host.CreatedBy),
		formatTime(host.CreatedAt),
		formatTime(host.UpdatedAt),
		nil,
	)
	if err != nil {
		return fmt.Errorf("create build host: %w", err)
	}
	return nil
}

func (r Repository) Update(ctx context.Context, host BuildHost) error {
	labels, err := encodeLabels(host.Labels)
	if err != nil {
		return err
	}

	query := `
UPDATE build_hosts
SET name = ` + placeholder(r.driverName, 1) + `,
    connection_type = ` + placeholder(r.driverName, 2) + `,
    address = ` + placeholder(r.driverName, 3) + `,
    port = ` + placeholder(r.driverName, 4) + `,
    username = ` + placeholder(r.driverName, 5) + `,
    credential_id = ` + placeholder(r.driverName, 6) + `,
    docker_endpoint = ` + placeholder(r.driverName, 7) + `,
    docker_command = ` + placeholder(r.driverName, 8) + `,
    architecture = NULL,
    os = NULL,
    docker_version = NULL,
    buildkit_supported = ` + placeholder(r.driverName, 9) + `,
    labels = ` + placeholder(r.driverName, 10) + `,
    max_concurrency = ` + placeholder(r.driverName, 11) + `,
    status = ` + placeholder(r.driverName, 12) + `,
    last_checked_at = NULL,
    last_check_result = NULL,
    last_error = NULL,
    updated_at = ` + placeholder(r.driverName, 13) + `
WHERE id = ` + placeholder(r.driverName, 14) + ` AND deleted_at IS NULL`

	result, err := r.db.ExecContext(
		ctx,
		query,
		host.Name,
		host.ConnectionType,
		nullString(host.Address),
		nullInt(host.Port),
		nullString(host.Username),
		nullString(host.CredentialID),
		nullString(host.DockerEndpoint),
		nullString(host.DockerCommand),
		false,
		labels,
		host.MaxConcurrency,
		host.Status,
		formatTime(host.UpdatedAt),
		host.ID,
	)
	if err != nil {
		return fmt.Errorf("update build host: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) UpdateCheckResult(ctx context.Context, hostID string, result CheckResult, checkedAt time.Time, rawResult string) error {
	query := `
UPDATE build_hosts
SET architecture = ` + placeholder(r.driverName, 1) + `,
    os = ` + placeholder(r.driverName, 2) + `,
    docker_version = ` + placeholder(r.driverName, 3) + `,
    buildkit_supported = ` + placeholder(r.driverName, 4) + `,
    status = ` + placeholder(r.driverName, 5) + `,
    last_checked_at = ` + placeholder(r.driverName, 6) + `,
    last_check_result = ` + placeholder(r.driverName, 7) + `,
    last_error = ` + placeholder(r.driverName, 8) + `,
    updated_at = ` + placeholder(r.driverName, 9) + `
WHERE id = ` + placeholder(r.driverName, 10) + ` AND deleted_at IS NULL`

	var errorMessage string
	if result.Error != nil {
		errorMessage = *result.Error
	}

	execResult, err := r.db.ExecContext(
		ctx,
		query,
		nullString(result.Architecture),
		nullString(result.OS),
		nullString(result.DockerVersion),
		result.BuildkitSupported,
		result.Status,
		formatTime(checkedAt),
		rawResult,
		nullString(errorMessage),
		formatTime(checkedAt),
		hostID,
	)
	if err != nil {
		return fmt.Errorf("update build host check result: %w", err)
	}
	return requireRowsAffected(execResult)
}

func (r Repository) SetStatus(ctx context.Context, id string, status string, updatedAt time.Time) error {
	query := `
UPDATE build_hosts
SET status = ` + placeholder(r.driverName, 1) + `,
    updated_at = ` + placeholder(r.driverName, 2) + `
WHERE id = ` + placeholder(r.driverName, 3) + ` AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, status, formatTime(updatedAt), id)
	if err != nil {
		return fmt.Errorf("set build host status: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) Delete(ctx context.Context, id string, deletedAt time.Time) error {
	query := `
UPDATE build_hosts
SET deleted_at = ` + placeholder(r.driverName, 1) + `,
    updated_at = ` + placeholder(r.driverName, 2) + `
WHERE id = ` + placeholder(r.driverName, 3) + ` AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, formatTime(deletedAt), formatTime(deletedAt), id)
	if err != nil {
		return fmt.Errorf("delete build host: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) filterWhere(filter ListFilter) (string, []any) {
	clauses := []string{"h.deleted_at IS NULL"}
	args := make([]any, 0, 3)

	if strings.TrimSpace(filter.Status) != "" {
		args = append(args, strings.TrimSpace(filter.Status))
		clauses = append(clauses, "h.status = "+placeholder(r.driverName, len(args)))
	}
	if strings.TrimSpace(filter.Architecture) != "" {
		args = append(args, strings.TrimSpace(filter.Architecture))
		clauses = append(clauses, "h.architecture = "+placeholder(r.driverName, len(args)))
	}
	if strings.TrimSpace(filter.ConnectionType) != "" {
		args = append(args, strings.TrimSpace(filter.ConnectionType))
		clauses = append(clauses, "h.connection_type = "+placeholder(r.driverName, len(args)))
	}

	return "WHERE " + strings.Join(clauses, " AND "), args
}

const selectHostSQL = `
SELECT h.id, h.name, h.connection_type, h.address, h.port, h.username, h.credential_id, c.fingerprint,
       h.docker_endpoint, h.docker_command, h.architecture, h.os, h.docker_version, h.buildkit_supported,
       h.labels, h.max_concurrency, h.current_running, h.status, h.last_checked_at, h.last_check_result,
       h.last_error, h.created_by, h.created_at, h.updated_at
FROM build_hosts h
LEFT JOIN credentials c ON c.id = h.credential_id
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHost(row rowScanner) (BuildHost, error) {
	var host BuildHost
	var address sql.NullString
	var port sql.NullInt64
	var username sql.NullString
	var credentialID sql.NullString
	var credentialFingerprint sql.NullString
	var dockerEndpoint sql.NullString
	var dockerCommand sql.NullString
	var architecture sql.NullString
	var osName sql.NullString
	var dockerVersion sql.NullString
	var labels string
	var lastCheckedAt sql.NullString
	var lastCheckResult sql.NullString
	var lastError sql.NullString
	var createdBy sql.NullString
	var createdAt string
	var updatedAt string

	err := row.Scan(
		&host.ID,
		&host.Name,
		&host.ConnectionType,
		&address,
		&port,
		&username,
		&credentialID,
		&credentialFingerprint,
		&dockerEndpoint,
		&dockerCommand,
		&architecture,
		&osName,
		&dockerVersion,
		&host.BuildkitSupported,
		&labels,
		&host.MaxConcurrency,
		&host.CurrentRunning,
		&host.Status,
		&lastCheckedAt,
		&lastCheckResult,
		&lastError,
		&createdBy,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return BuildHost{}, err
	}

	host.Address = address.String
	if port.Valid {
		host.Port = int(port.Int64)
	}
	host.Username = username.String
	host.CredentialID = credentialID.String
	host.CredentialFingerprint = credentialFingerprint.String
	host.DockerEndpoint = dockerEndpoint.String
	host.DockerCommand = dockerCommand.String
	host.Architecture = architecture.String
	host.OS = osName.String
	host.DockerVersion = dockerVersion.String
	host.LastCheckResult = lastCheckResult.String
	host.LastError = lastError.String
	host.CreatedBy = createdBy.String
	host.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	host.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if lastCheckedAt.Valid {
		parsed, err := time.Parse(time.RFC3339, lastCheckedAt.String)
		if err == nil {
			host.LastCheckedAt = &parsed
		}
	}
	if err := json.Unmarshal([]byte(labels), &host.Labels); err != nil {
		host.Labels = []string{}
	}

	return host, nil
}

func encodeLabels(labels []string) (string, error) {
	if labels == nil {
		labels = []string{}
	}
	data, err := json.Marshal(labels)
	if err != nil {
		return "", fmt.Errorf("encode build host labels: %w", err)
	}
	return string(data), nil
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

func nullInt(value int) any {
	if value == 0 {
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
