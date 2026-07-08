package retention

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/VmythV/image-build-platform/internal/settings"
)

const (
	logRetentionKey     = "retention.log_days"
	contextRetentionKey = "retention.context_days"
	defaultInterval     = 24 * time.Hour
)

type Settings interface {
	FindByKey(ctx context.Context, key string) (settings.Setting, error)
}

type Cleaner struct {
	LogDir                      string
	ContextDir                  string
	DefaultLogRetentionDays     int
	DefaultContextRetentionDays int
	Settings                    Settings
	Logger                      *slog.Logger
	Now                         func() time.Time
}

type Report struct {
	DeletedLogFiles     int
	DeletedContextItems int
}

func (c Cleaner) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = defaultInterval
	}

	c.runOnceWithLog(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.runOnceWithLog(ctx)
		}
	}
}

func (c Cleaner) RunOnce(ctx context.Context) (Report, error) {
	now := c.now()
	logDays := c.settingInt(ctx, logRetentionKey, c.DefaultLogRetentionDays)
	contextDays := c.settingInt(ctx, contextRetentionKey, c.DefaultContextRetentionDays)

	var report Report
	if logDays > 0 {
		deleted, err := cleanupChildrenOlderThan(filepath.Join(c.LogDir, "builds"), now.Add(-durationDays(logDays)))
		if err != nil {
			return report, err
		}
		report.DeletedLogFiles = deleted
	}
	if contextDays > 0 {
		deleted, err := cleanupChildrenOlderThan(c.ContextDir, now.Add(-durationDays(contextDays)))
		if err != nil {
			return report, err
		}
		report.DeletedContextItems = deleted
	}

	return report, nil
}

func (c Cleaner) runOnceWithLog(ctx context.Context) {
	report, err := c.RunOnce(ctx)
	if err != nil {
		c.logger().Warn("run retention cleanup", "error", err)
		return
	}
	if report.DeletedLogFiles > 0 || report.DeletedContextItems > 0 {
		c.logger().Info("retention cleanup completed", "deleted_log_files", report.DeletedLogFiles, "deleted_context_items", report.DeletedContextItems)
	}
}

func cleanupChildrenOlderThan(root string, cutoff time.Time) (int, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return 0, nil
	}
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("stat retention root %q: %w", root, err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("retention root %q is not a directory", root)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, fmt.Errorf("read retention root %q: %w", root, err)
	}

	deleted := 0
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return deleted, fmt.Errorf("stat retention item %q: %w", entry.Name(), err)
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return deleted, fmt.Errorf("remove retention item %q: %w", entry.Name(), err)
		}
		deleted++
	}

	return deleted, nil
}

func (c Cleaner) settingInt(ctx context.Context, key string, fallback int) int {
	if fallback < 0 {
		fallback = 0
	}
	if c.Settings == nil {
		return fallback
	}
	setting, err := c.Settings.FindByKey(ctx, key)
	if err != nil {
		if !errors.Is(err, settings.ErrNotFound) {
			c.logger().Warn("load retention setting", "key", key, "error", err)
		}
		return fallback
	}
	value, err := strconv.Atoi(strings.TrimSpace(setting.Value))
	if err != nil || value < 0 {
		c.logger().Warn("parse retention setting", "key", key, "value", setting.Value)
		return fallback
	}
	return value
}

func (c Cleaner) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c Cleaner) logger() *slog.Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return slog.Default()
}

func durationDays(days int) time.Duration {
	return time.Duration(days) * 24 * time.Hour
}
