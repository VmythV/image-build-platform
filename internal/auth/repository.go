package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type Repository struct {
	db         *sql.DB
	driverName string
}

func NewRepository(db *sql.DB, driverName string) Repository {
	return Repository{db: db, driverName: driverName}
}

func (r Repository) CountUsers(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE deleted_at IS NULL").Scan(&count); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}

func (r Repository) ListUsers(ctx context.Context) ([]User, error) {
	query := `
SELECT id, username, display_name, password_hash, role, status, last_login_at, created_at, updated_at
FROM users
WHERE deleted_at IS NULL
ORDER BY created_at DESC, username ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		user, err := scanUserRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

func (r Repository) CreateUser(ctx context.Context, user User) error {
	query := `
INSERT INTO users (
	id, username, display_name, password_hash, role, status, last_login_at, created_at, updated_at, deleted_at
) VALUES (` + placeholders(r.driverName, 10) + `)`

	_, err := r.db.ExecContext(
		ctx,
		query,
		user.ID,
		user.Username,
		user.DisplayName,
		user.PasswordHash,
		user.Role,
		user.Status,
		nullableTime(user.LastLoginAt),
		formatTime(user.CreatedAt),
		formatTime(user.UpdatedAt),
		nil,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r Repository) FindUserByID(ctx context.Context, id string) (User, error) {
	query := `
SELECT id, username, display_name, password_hash, role, status, last_login_at, created_at, updated_at
FROM users
WHERE id = ` + placeholder(r.driverName, 1) + ` AND deleted_at IS NULL`

	return r.scanUser(r.db.QueryRowContext(ctx, query, id))
}

func (r Repository) FindUserByUsername(ctx context.Context, username string) (User, error) {
	query := `
SELECT id, username, display_name, password_hash, role, status, last_login_at, created_at, updated_at
FROM users
WHERE username = ` + placeholder(r.driverName, 1) + ` AND deleted_at IS NULL`

	return r.scanUser(r.db.QueryRowContext(ctx, query, username))
}

func (r Repository) UpdateUser(ctx context.Context, user User) error {
	query := `
UPDATE users
SET display_name = ` + placeholder(r.driverName, 1) + `,
    role = ` + placeholder(r.driverName, 2) + `,
    status = ` + placeholder(r.driverName, 3) + `,
    updated_at = ` + placeholder(r.driverName, 4) + `
WHERE id = ` + placeholder(r.driverName, 5) + ` AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, user.DisplayName, user.Role, user.Status, formatTime(user.UpdatedAt), user.ID)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) UpdatePassword(ctx context.Context, userID string, passwordHash string, updatedAt time.Time) error {
	query := `
UPDATE users
SET password_hash = ` + placeholder(r.driverName, 1) + `,
    updated_at = ` + placeholder(r.driverName, 2) + `
WHERE id = ` + placeholder(r.driverName, 3) + ` AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, query, passwordHash, formatTime(updatedAt), userID)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	return requireRowsAffected(result)
}

func (r Repository) FindUserBySessionHash(ctx context.Context, tokenHash string, now time.Time) (User, error) {
	query := `
SELECT u.id, u.username, u.display_name, u.password_hash, u.role, u.status, u.last_login_at, u.created_at, u.updated_at
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = ` + placeholder(r.driverName, 1) + `
  AND s.expires_at > ` + placeholder(r.driverName, 2) + `
  AND u.status = ` + placeholder(r.driverName, 3) + `
  AND u.deleted_at IS NULL`

	return r.scanUser(r.db.QueryRowContext(ctx, query, tokenHash, formatTime(now), UserStatusActive))
}

func (r Repository) UpdateLastLogin(ctx context.Context, userID string, at time.Time) error {
	query := "UPDATE users SET last_login_at = " + placeholder(r.driverName, 1) + ", updated_at = " + placeholder(r.driverName, 2) + " WHERE id = " + placeholder(r.driverName, 3)
	if _, err := r.db.ExecContext(ctx, query, formatTime(at), formatTime(at), userID); err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	return nil
}

func (r Repository) CreateSession(ctx context.Context, sessionID string, userID string, tokenHash string, userAgent string, ipAddress string, expiresAt time.Time, createdAt time.Time) error {
	query := `
INSERT INTO sessions (
	id, user_id, token_hash, user_agent, ip_address, expires_at, created_at, last_seen_at
) VALUES (` + placeholders(r.driverName, 8) + `)`

	_, err := r.db.ExecContext(ctx, query, sessionID, userID, tokenHash, userAgent, ipAddress, formatTime(expiresAt), formatTime(createdAt), formatTime(createdAt))
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r Repository) DeleteSession(ctx context.Context, tokenHash string) error {
	query := "DELETE FROM sessions WHERE token_hash = " + placeholder(r.driverName, 1)
	if _, err := r.db.ExecContext(ctx, query, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (r Repository) TouchSession(ctx context.Context, tokenHash string, at time.Time) error {
	query := "UPDATE sessions SET last_seen_at = " + placeholder(r.driverName, 1) + " WHERE token_hash = " + placeholder(r.driverName, 2)
	if _, err := r.db.ExecContext(ctx, query, formatTime(at), tokenHash); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

func (r Repository) scanUser(row *sql.Row) (User, error) {
	var user User
	var lastLoginAt sql.NullString
	var createdAt string
	var updatedAt string

	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&lastLoginAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("scan user: %w", err)
	}

	if lastLoginAt.Valid {
		parsed, err := time.Parse(time.RFC3339, lastLoginAt.String)
		if err == nil {
			user.LastLoginAt = &parsed
		}
	}
	user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	user.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return user, nil
}

func scanUserRows(rows *sql.Rows) (User, error) {
	var user User
	var lastLoginAt sql.NullString
	var createdAt string
	var updatedAt string

	err := rows.Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&lastLoginAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}

	if lastLoginAt.Valid {
		parsed, err := time.Parse(time.RFC3339, lastLoginAt.String)
		if err == nil {
			user.LastLoginAt = &parsed
		}
	}
	user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	user.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return user, nil
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

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}
