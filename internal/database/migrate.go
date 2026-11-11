package database

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

func (d *DB) Migrate(ctx context.Context, migrations fs.FS, logger *slog.Logger) error {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("create migration lock: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, d.sql, migrations, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	for _, result := range results {
		logger.Info("migration applied",
			slog.Int64("schema_version", result.Source.Version),
			slog.String("file", result.Source.Path),
			slog.Duration("duration", result.Duration),
		)
	}

	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	logger.Info("database schema up to date", slog.Int64("schema_version", version), slog.Int("applied", len(results)))

	return nil
}
