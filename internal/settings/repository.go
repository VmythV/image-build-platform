package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNotFound   = errors.New("setting not found")
	ErrValidation = errors.New("setting validation failed")
)

type Repository struct {
	db         *sql.DB
	driverName string
}

func NewRepository(db *sql.DB, driverName string) Repository {
	return Repository{db: db, driverName: driverName}
}

func (r Repository) EnsureDefaults(ctx context.Context, now time.Time) error {
	return r.EnsureDefaultsWithValues(ctx, now, nil)
}

func (r Repository) EnsureDefaultsWithValues(ctx context.Context, now time.Time, values map[string]string) error {
	for _, definition := range Defaults() {
		value := definition.Value
		if override, ok := values[definition.Key]; ok {
			normalized, err := normalizeValue(override, definition.ValueType)
			if err != nil {
				return fmt.Errorf("normalize default setting %s: %w", definition.Key, err)
			}
			value = normalized
		}
		query := `
INSERT INTO system_settings (key, value, value_type, description, updated_by, updated_at)
VALUES (` + placeholders(r.driverName, 6) + `)
ON CONFLICT (key) DO NOTHING`
		if _, err := r.db.ExecContext(ctx, query, definition.Key, value, definition.ValueType, definition.Description, nil, formatTime(now)); err != nil {
			return fmt.Errorf("ensure default setting %s: %w", definition.Key, err)
		}
	}
	return nil
}

func (r Repository) List(ctx context.Context) ([]Setting, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT key, value, value_type, description, updated_by, updated_at
FROM system_settings
ORDER BY key ASC`)
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	defer rows.Close()

	settings := make([]Setting, 0)
	for rows.Next() {
		setting, err := scanSetting(rows)
		if err != nil {
			return nil, err
		}
		settings = append(settings, setting)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settings: %w", err)
	}
	return settings, nil
}

func (r Repository) Update(ctx context.Context, key string, value string, actorID string, updatedAt time.Time) (Setting, error) {
	definition, ok := definitionByKey(key)
	if !ok {
		return Setting{}, ErrNotFound
	}
	normalized, err := normalizeValue(value, definition.ValueType)
	if err != nil {
		return Setting{}, err
	}

	query := `
UPDATE system_settings
SET value = ` + placeholder(r.driverName, 1) + `,
    value_type = ` + placeholder(r.driverName, 2) + `,
    description = ` + placeholder(r.driverName, 3) + `,
    updated_by = ` + placeholder(r.driverName, 4) + `,
    updated_at = ` + placeholder(r.driverName, 5) + `
WHERE key = ` + placeholder(r.driverName, 6)
	result, err := r.db.ExecContext(ctx, query, normalized, definition.ValueType, definition.Description, nullString(actorID), formatTime(updatedAt), key)
	if err != nil {
		return Setting{}, fmt.Errorf("update setting: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected == 0 {
		return Setting{}, ErrNotFound
	}
	return r.FindByKey(ctx, key)
}

func (r Repository) FindByKey(ctx context.Context, key string) (Setting, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT key, value, value_type, description, updated_by, updated_at
FROM system_settings
WHERE key = `+placeholder(r.driverName, 1), key)
	setting, err := scanSetting(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Setting{}, ErrNotFound
		}
		return Setting{}, err
	}
	return setting, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSetting(row rowScanner) (Setting, error) {
	var setting Setting
	var updatedBy sql.NullString
	var updatedAt string
	if err := row.Scan(&setting.Key, &setting.Value, &setting.ValueType, &setting.Description, &updatedBy, &updatedAt); err != nil {
		return Setting{}, err
	}
	setting.UpdatedBy = updatedBy.String
	setting.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return setting, nil
}

func definitionByKey(key string) (Definition, bool) {
	for _, definition := range Defaults() {
		if definition.Key == key {
			return definition, true
		}
	}
	return Definition{}, false
}

func normalizeValue(value string, valueType string) (string, error) {
	value = strings.TrimSpace(value)
	switch valueType {
	case TypeInteger:
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return "", fmt.Errorf("%w: value must be a non-negative integer", ErrValidation)
		}
		return strconv.Itoa(parsed), nil
	case TypeBoolean:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return "", fmt.Errorf("%w: value must be true or false", ErrValidation)
		}
		return strconv.FormatBool(parsed), nil
	case TypeString:
		if value == "" {
			return "", fmt.Errorf("%w: value is required", ErrValidation)
		}
		return value, nil
	default:
		return "", fmt.Errorf("%w: unsupported value type", ErrValidation)
	}
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
