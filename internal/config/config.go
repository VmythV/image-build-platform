package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Security SecurityConfig `yaml:"security"`
	Storage  StorageConfig  `yaml:"storage"`
	Build    BuildConfig    `yaml:"build"`
	Logs     LogsConfig     `yaml:"logs"`
	Contexts ContextsConfig `yaml:"contexts"`
}

type ServerConfig struct {
	Addr           string   `yaml:"addr"`
	PublicURL      string   `yaml:"public_url"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	StaticDir      string   `yaml:"static_dir"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type SecurityConfig struct {
	SecretKey    string `yaml:"secret_key"`
	SessionTTL   string `yaml:"session_ttl"`
	SecureCookie bool   `yaml:"secure_cookie"`
	CSRFEnabled  bool   `yaml:"csrf_enabled"`
}

type StorageConfig struct {
	DataDir    string `yaml:"data_dir"`
	LogDir     string `yaml:"log_dir"`
	ContextDir string `yaml:"context_dir"`
	TmpDir     string `yaml:"tmp_dir"`
	BackupDir  string `yaml:"backup_dir"`
}

type BuildConfig struct {
	DefaultTimeout       string `yaml:"default_timeout"`
	MaxGlobalConcurrency int    `yaml:"max_global_concurrency"`
	EnableBuildkit       bool   `yaml:"enable_buildkit"`
}

type LogsConfig struct {
	RetentionDays int `yaml:"retention_days"`
}

type ContextsConfig struct {
	RetentionDays int `yaml:"retention_days"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Addr:           "0.0.0.0:8080",
			PublicURL:      "http://localhost:8080",
			AllowedOrigins: []string{},
			StaticDir:      "web/dist",
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    "data/app.db",
		},
		Security: SecurityConfig{
			SecretKey:    "change-me-use-a-long-random-secret",
			SessionTTL:   "24h",
			SecureCookie: false,
			CSRFEnabled:  true,
		},
		Storage: StorageConfig{
			DataDir:    "data",
			LogDir:     "data/logs",
			ContextDir: "data/contexts",
			TmpDir:     "data/tmp",
			BackupDir:  "data/backups",
		},
		Build: BuildConfig{
			DefaultTimeout:       "1h",
			MaxGlobalConcurrency: 2,
			EnableBuildkit:       true,
		},
		Logs: LogsConfig{
			RetentionDays: 30,
		},
		Contexts: ContextsConfig{
			RetentionDays: 7,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	if path == "" {
		path = os.Getenv("IBP_CONFIG")
	}

	if path != "" {
		if err := mergeFile(&cfg, path); err != nil {
			return Config{}, err
		}
	} else if _, err := os.Stat("config.yaml"); err == nil {
		if err := mergeFile(&cfg, "config.yaml"); err != nil {
			return Config{}, err
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("stat config.yaml: %w", err)
	}

	applyEnv(&cfg)

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func mergeFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config file %q: %w", path, err)
	}
	return nil
}

func applyEnv(cfg *Config) {
	setString(&cfg.Server.Addr, "IBP_SERVER_ADDR")
	setString(&cfg.Server.PublicURL, "IBP_PUBLIC_URL")
	setString(&cfg.Server.StaticDir, "IBP_STATIC_DIR")
	setString(&cfg.Database.Driver, "IBP_DATABASE_DRIVER")
	setString(&cfg.Database.DSN, "IBP_DATABASE_DSN")
	setString(&cfg.Security.SecretKey, "IBP_SECURITY_SECRET_KEY")
	setString(&cfg.Security.SessionTTL, "IBP_SECURITY_SESSION_TTL")
	setBool(&cfg.Security.SecureCookie, "IBP_SECURITY_SECURE_COOKIE")
	setBool(&cfg.Security.CSRFEnabled, "IBP_SECURITY_CSRF_ENABLED")
	setString(&cfg.Storage.DataDir, "IBP_DATA_DIR")
	setString(&cfg.Storage.LogDir, "IBP_LOG_DIR")
	setString(&cfg.Storage.ContextDir, "IBP_CONTEXT_DIR")
	setString(&cfg.Storage.TmpDir, "IBP_TMP_DIR")
	setString(&cfg.Storage.BackupDir, "IBP_BACKUP_DIR")
	setString(&cfg.Build.DefaultTimeout, "IBP_BUILD_DEFAULT_TIMEOUT")
	setInt(&cfg.Build.MaxGlobalConcurrency, "IBP_MAX_GLOBAL_CONCURRENCY")
	setBool(&cfg.Build.EnableBuildkit, "IBP_BUILD_ENABLE_BUILDKIT")
	setInt(&cfg.Logs.RetentionDays, "IBP_LOG_RETENTION_DAYS")
	setInt(&cfg.Contexts.RetentionDays, "IBP_CONTEXT_RETENTION_DAYS")
}

func validate(cfg Config) error {
	if strings.TrimSpace(cfg.Server.Addr) == "" {
		return errors.New("server.addr is required")
	}
	if strings.TrimSpace(cfg.Database.Driver) == "" {
		return errors.New("database.driver is required")
	}
	if strings.TrimSpace(cfg.Database.DSN) == "" {
		return errors.New("database.dsn is required")
	}
	if len(strings.TrimSpace(cfg.Security.SecretKey)) < 32 {
		return errors.New("security.secret_key must be at least 32 characters")
	}
	if cfg.Build.MaxGlobalConcurrency < 1 {
		return errors.New("build.max_global_concurrency must be at least 1")
	}
	if strings.TrimSpace(cfg.Build.DefaultTimeout) == "" {
		return errors.New("build.default_timeout is required")
	}
	timeout, err := time.ParseDuration(cfg.Build.DefaultTimeout)
	if err != nil {
		return fmt.Errorf("build.default_timeout is invalid: %w", err)
	}
	if timeout <= 0 {
		return errors.New("build.default_timeout must be positive")
	}
	if strings.TrimSpace(cfg.Security.SessionTTL) != "" {
		ttl, err := time.ParseDuration(cfg.Security.SessionTTL)
		if err != nil {
			return fmt.Errorf("security.session_ttl is invalid: %w", err)
		}
		if ttl <= 0 {
			return errors.New("security.session_ttl must be positive")
		}
	}
	return nil
}

func setString(target *string, key string) {
	if value := os.Getenv(key); value != "" {
		*target = value
	}
}

func setInt(target *int, key string) {
	value := os.Getenv(key)
	if value == "" {
		return
	}
	parsed, err := strconv.Atoi(value)
	if err == nil {
		*target = parsed
	}
}

func setBool(target *bool, key string) {
	value := os.Getenv(key)
	if value == "" {
		return
	}
	parsed, err := strconv.ParseBool(value)
	if err == nil {
		*target = parsed
	}
}
