//go:build integration

// Package integration holds testcontainers-go integration tests for the
// repository/service layer (research.md §10; tasks.md's Tests note commits
// to real-Postgres tests as required, not optional, for the correctness-
// critical cost/profit snapshotting paths).
//
// Run with Docker available: go test -tags=integration ./tests/integration/...
package integration

import (
	"context"
	"database/sql"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fsetiawan29/profit-tracker/migrations"
)

// setupTestDB starts a disposable Postgres container, applies every embedded
// migration (the same set cmd/server/main.go applies at startup), and
// returns a connected *gorm.DB. The container is terminated via t.Cleanup.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("profit_tracker_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, container.Terminate(context.Background()))
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, applyMigrations(sqlDB))

	return db
}

// applyMigrations mirrors cmd/server/main.go's applyMigrations so tests
// exercise the exact embedded migration set the running server uses.
func applyMigrations(sqlDB *sql.DB) error {
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}

	dbDriver, err := migratepostgres.WithInstance(sqlDB, &migratepostgres.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", dbDriver)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
