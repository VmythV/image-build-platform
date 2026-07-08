package audit

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/VmythV/image-build-platform/internal/platform/id"
)

type Repository struct {
	db         *sql.DB
	driverName string
}

func NewRepository(db *sql.DB, driverName string) Repository {
	return Repository{db: db, driverName: driverName}
}

func (r Repository) Record(ctx context.Context, input RecordInput, createdAt time.Time) error {
	query := `
INSERT INTO audit_logs (
	id, actor_id, actor_name, action, resource_type, resource_id, resource_name,
	ip_address, user_agent, request_id, detail, created_at
) VALUES (` + placeholders(r.driverName, 12) + `)`

	_, err := r.db.ExecContext(
		ctx,
		query,
		id.New(),
		nullString(input.ActorID),
		nullString(input.ActorName),
		input.Action,
		input.ResourceType,
		nullString(input.ResourceID),
		nullString(input.ResourceName),
		nullString(input.IPAddress),
		nullString(input.UserAgent),
		nullString(input.RequestID),
		nullString(input.Detail),
		formatTime(createdAt),
	)
	if err != nil {
		return fmt.Errorf("record audit log: %w", err)
	}
	return nil
}

func (r Repository) List(ctx context.Context, filter ListFilter) ([]Log, int, error) {
	where, args := r.filterWhere(filter)
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	page := normalizePage(filter.Page)
	pageSize := normalizePageSize(filter.PageSize)
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	query := `
SELECT id, actor_id, actor_name, action, resource_type, resource_id, resource_name,
       ip_address, user_agent, request_id, detail, created_at
FROM audit_logs ` + where + `
ORDER BY created_at DESC
LIMIT ` + placeholder(r.driverName, len(args)-1) + ` OFFSET ` + placeholder(r.driverName, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	logs := make([]Log, 0)
	for rows.Next() {
		log, err := scanLog(rows)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate audit logs: %w", err)
	}
	return logs, total, nil
}

func (r Repository) filterWhere(filter ListFilter) (string, []any) {
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 3)

	if strings.TrimSpace(filter.ActorID) != "" {
		args = append(args, strings.TrimSpace(filter.ActorID))
		clauses = append(clauses, "actor_id = "+placeholder(r.driverName, len(args)))
	}
	if strings.TrimSpace(filter.Action) != "" {
		args = append(args, strings.TrimSpace(filter.Action))
		clauses = append(clauses, "action = "+placeholder(r.driverName, len(args)))
	}
	if strings.TrimSpace(filter.ResourceType) != "" {
		args = append(args, strings.TrimSpace(filter.ResourceType))
		clauses = append(clauses, "resource_type = "+placeholder(r.driverName, len(args)))
	}

	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLog(row rowScanner) (Log, error) {
	var log Log
	var actorID, actorName, resourceID, resourceName, ipAddress, userAgent, requestID, detail sql.NullString
	var createdAt string

	err := row.Scan(
		&log.ID,
		&actorID,
		&actorName,
		&log.Action,
		&log.ResourceType,
		&resourceID,
		&resourceName,
		&ipAddress,
		&userAgent,
		&requestID,
		&detail,
		&createdAt,
	)
	if err != nil {
		return Log{}, err
	}

	log.ActorID = actorID.String
	log.ActorName = actorName.String
	log.ResourceID = resourceID.String
	log.ResourceName = resourceName.String
	log.IPAddress = ipAddress.String
	log.UserAgent = userAgent.String
	log.RequestID = requestID.String
	log.Detail = detail.String
	log.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return log, nil
}

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizePageSize(pageSize int) int {
	if pageSize < 1 {
		return 50
	}
	if pageSize > 200 {
		return 200
	}
	return pageSize
}

func placeholders(driverName string, count int) string {
	parts := make([]string, count)
	for i := range parts {
		parts[i] = placeholder(driverName, i+1)
	}
	return strings.Join(parts, ", ")
}

func placeholder(driverName string, index int) string {
	if driverName == "postgres" || driverName == "pgx" {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}
