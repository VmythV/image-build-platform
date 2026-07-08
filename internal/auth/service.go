package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VmythV/image-build-platform/internal/platform/clock"
	"github.com/VmythV/image-build-platform/internal/platform/id"
)

var (
	ErrAlreadyInitialized = errors.New("system is already initialized")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrNotFound           = errors.New("not found")
	ErrUnauthenticated    = errors.New("authentication is required")
	ErrForbidden          = errors.New("permission denied")
)

type Service struct {
	repo       Repository
	sessionTTL time.Duration
}

type ServiceOptions struct {
	Repository Repository
	SessionTTL string
}

func NewService(opts ServiceOptions) (Service, error) {
	ttl := 24 * time.Hour
	if strings.TrimSpace(opts.SessionTTL) != "" {
		parsed, err := time.ParseDuration(opts.SessionTTL)
		if err != nil {
			return Service{}, fmt.Errorf("parse session ttl: %w", err)
		}
		ttl = parsed
	}
	if ttl <= 0 {
		return Service{}, errors.New("session ttl must be positive")
	}

	return Service{
		repo:       opts.Repository,
		sessionTTL: ttl,
	}, nil
}

func (s Service) IsInitialized(ctx context.Context) (bool, error) {
	count, err := s.repo.CountUsers(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s Service) InitializeAdmin(ctx context.Context, username string, password string, displayName string) (User, error) {
	initialized, err := s.IsInitialized(ctx)
	if err != nil {
		return User{}, err
	}
	if initialized {
		return User{}, ErrAlreadyInitialized
	}

	username = strings.TrimSpace(username)
	displayName = strings.TrimSpace(displayName)
	if username == "" {
		return User{}, errors.New("username is required")
	}
	if displayName == "" {
		displayName = username
	}
	if err := validatePassword(username, password); err != nil {
		return User{}, err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}

	now := clock.Now()
	user := User{
		ID:           id.New(),
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: hash,
		Role:         RoleAdmin,
		Status:       UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return User{}, err
	}

	return user, nil
}

func (s Service) Login(ctx context.Context, username string, password string, userAgent string, ipAddress string) (User, string, time.Time, error) {
	user, err := s.repo.FindUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return User{}, "", time.Time{}, ErrInvalidCredentials
		}
		return User{}, "", time.Time{}, err
	}
	if user.Status != UserStatusActive {
		return User{}, "", time.Time{}, ErrInvalidCredentials
	}
	if !CheckPassword(user.PasswordHash, password) {
		return User{}, "", time.Time{}, ErrInvalidCredentials
	}

	token, tokenHash, err := NewSessionToken()
	if err != nil {
		return User{}, "", time.Time{}, err
	}

	now := clock.Now()
	expiresAt := now.Add(s.sessionTTL)
	if err := s.repo.CreateSession(ctx, id.New(), user.ID, tokenHash, userAgent, ipAddress, expiresAt, now); err != nil {
		return User{}, "", time.Time{}, err
	}
	if err := s.repo.UpdateLastLogin(ctx, user.ID, now); err != nil {
		return User{}, "", time.Time{}, err
	}
	user.LastLoginAt = &now
	user.UpdatedAt = now

	return user, token, expiresAt, nil
}

func (s Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repo.DeleteSession(ctx, HashSessionToken(token))
}

func (s Service) CurrentUser(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthenticated
	}

	tokenHash := HashSessionToken(token)
	user, err := s.repo.FindUserBySessionHash(ctx, tokenHash, clock.Now())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return User{}, ErrUnauthenticated
		}
		return User{}, err
	}
	_ = s.repo.TouchSession(ctx, tokenHash, clock.Now())

	return user, nil
}
