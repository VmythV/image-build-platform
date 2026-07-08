package retention

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VmythV/image-build-platform/internal/settings"
)

func TestCleanerRunOnceDeletesExpiredBuildLogsAndContexts(t *testing.T) {
	root := t.TempDir()
	logDir := filepath.Join(root, "logs")
	contextDir := filepath.Join(root, "contexts")
	buildLogDir := filepath.Join(logDir, "builds")
	if err := os.MkdirAll(buildLogDir, 0o750); err != nil {
		t.Fatalf("create log dir: %v", err)
	}
	if err := os.MkdirAll(contextDir, 0o750); err != nil {
		t.Fatalf("create context dir: %v", err)
	}

	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	oldLog := touchFile(t, buildLogDir, "old.log", now.Add(-48*time.Hour))
	newLog := touchFile(t, buildLogDir, "new.log", now.Add(-2*time.Hour))
	oldContext := touchDir(t, contextDir, "old-context", now.Add(-72*time.Hour))
	newContext := touchDir(t, contextDir, "new-context", now.Add(-2*time.Hour))

	cleaner := Cleaner{
		LogDir:                      logDir,
		ContextDir:                  contextDir,
		DefaultLogRetentionDays:     1,
		DefaultContextRetentionDays: 2,
		Now:                         func() time.Time { return now },
	}
	report, err := cleaner.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run cleaner: %v", err)
	}

	if report.DeletedLogFiles != 1 {
		t.Fatalf("expected one deleted log file, got %d", report.DeletedLogFiles)
	}
	if report.DeletedContextItems != 1 {
		t.Fatalf("expected one deleted context item, got %d", report.DeletedContextItems)
	}
	assertNotExists(t, oldLog)
	assertExists(t, newLog)
	assertNotExists(t, oldContext)
	assertExists(t, newContext)
}

func TestCleanerRunOnceAllowsSettingsToDisableRetention(t *testing.T) {
	root := t.TempDir()
	logDir := filepath.Join(root, "logs")
	buildLogDir := filepath.Join(logDir, "builds")
	if err := os.MkdirAll(buildLogDir, 0o750); err != nil {
		t.Fatalf("create log dir: %v", err)
	}

	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	oldLog := touchFile(t, buildLogDir, "old.log", now.Add(-48*time.Hour))

	cleaner := Cleaner{
		LogDir:                  logDir,
		DefaultLogRetentionDays: 1,
		Settings: fakeSettings{
			logRetentionKey: "0",
		},
		Now: func() time.Time { return now },
	}
	report, err := cleaner.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run cleaner: %v", err)
	}

	if report.DeletedLogFiles != 0 {
		t.Fatalf("expected zero deleted log files, got %d", report.DeletedLogFiles)
	}
	assertExists(t, oldLog)
}

type fakeSettings map[string]string

func (f fakeSettings) FindByKey(_ context.Context, key string) (settings.Setting, error) {
	value, ok := f[key]
	if !ok {
		return settings.Setting{}, settings.ErrNotFound
	}
	return settings.Setting{Key: key, Value: value, ValueType: settings.TypeInteger}, nil
}

func touchFile(t *testing.T, dir string, name string, modTime time.Time) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("log"), 0o640); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("set file time: %v", err)
	}
	return path
}

func touchDir(t *testing.T, dir string, name string, modTime time.Time) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "Dockerfile"), []byte("FROM scratch\n"), 0o640); err != nil {
		t.Fatalf("write context file: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("set dir time: %v", err)
	}
	return path
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s to be deleted", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat %s: %v", path, err)
	}
}
