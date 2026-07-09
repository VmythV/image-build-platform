package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VmythV/image-build-platform/internal/config"
	"github.com/VmythV/image-build-platform/internal/retention"
	"github.com/VmythV/image-build-platform/internal/server"
	systemsettings "github.com/VmythV/image-build-platform/internal/settings"
	"github.com/VmythV/image-build-platform/internal/storage"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "", "configuration file path")
	addr := flag.String("addr", "", "HTTP listen address")
	staticDir := flag.String("static-dir", "", "frontend static asset directory")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	if *addr != "" {
		cfg.Server.Addr = *addr
	}
	if *staticDir != "" {
		cfg.Server.StaticDir = *staticDir
	}

	defaultBuildTimeout, err := time.ParseDuration(cfg.Build.DefaultTimeout)
	if err != nil {
		logger.Error("parse build timeout", "error", err)
		os.Exit(1)
	}
	schedulerInterval, err := time.ParseDuration(cfg.Build.SchedulerInterval)
	if err != nil {
		logger.Error("parse scheduler interval", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := storage.Open(ctx, cfg)
	if err != nil {
		logger.Error("initialize storage", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("close storage", "error", err)
		}
	}()

	runtimeCtx, runtimeCancel := context.WithCancel(context.Background())
	defer runtimeCancel()

	handler, err := server.New(server.Options{
		StaticDir:            cfg.Server.StaticDir,
		Version:              version,
		Logger:               logger,
		DB:                   store.DB,
		DriverName:           store.DriverName,
		SessionTTL:           cfg.Security.SessionTTL,
		SecureCookie:         cfg.Security.SecureCookie,
		CSRFEnabled:          cfg.Security.CSRFEnabled,
		SecretKey:            cfg.Security.SecretKey,
		ContextDir:           cfg.Storage.ContextDir,
		LogDir:               cfg.Storage.LogDir,
		DefaultBuildTimeout:  defaultBuildTimeout,
		MaxGlobalConcurrency: cfg.Build.MaxGlobalConcurrency,
		SchedulerEnabled:     cfg.Build.SchedulerEnabled,
		SchedulerInterval:    schedulerInterval,
		LogRetentionDays:     cfg.Logs.RetentionDays,
		ContextRetentionDays: cfg.Contexts.RetentionDays,
		RuntimeContext:       runtimeCtx,
	})
	if err != nil {
		logger.Error("initialize server", "error", err)
		os.Exit(1)
	}

	retentionCleaner := retention.Cleaner{
		LogDir:                      cfg.Storage.LogDir,
		ContextDir:                  cfg.Storage.ContextDir,
		DefaultLogRetentionDays:     cfg.Logs.RetentionDays,
		DefaultContextRetentionDays: cfg.Contexts.RetentionDays,
		Settings:                    systemsettings.NewRepository(store.DB, store.DriverName),
		Logger:                      logger,
	}
	go retentionCleaner.Run(runtimeCtx, 24*time.Hour)

	httpServer := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("starting image build platform", "addr", cfg.Server.Addr, "version", version, "database_driver", store.DriverName)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	runtimeCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	logger.Info("shutting down")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}
