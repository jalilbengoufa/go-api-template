package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/jalilbengoufa/go-api-template/internal/config"
	"github.com/jalilbengoufa/go-api-template/internal/database"
	"github.com/jalilbengoufa/go-api-template/internal/repository"
	"github.com/jalilbengoufa/go-api-template/internal/server"
	"github.com/jalilbengoufa/go-api-template/internal/service"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger := server.NewLogger(cfg.Log, os.Stdout).With(
		slog.String("service", "go-api-template"),
		slog.String("version", buildVersion()),
	)
	slog.SetDefault(logger)

	db, err := database.Open(ctx, cfg.Database, logger)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("failed to close database", slog.Any("error", err))
		}
	}()

	resourceService := service.NewResourceService(repository.NewResourceRepository(db.DB))

	srv, err := server.New(server.Dependencies{
		Config:   cfg,
		Logger:   logger,
		Database: db,
		Resource: resourceService,
		Version:  buildVersion(),
	})
	if err != nil {
		return fmt.Errorf("build server: %w", err)
	}

	return srv.Run(ctx)
}

func buildVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" && len(setting.Value) >= 7 {
			return setting.Value[:7]
		}
	}
	return version
}
