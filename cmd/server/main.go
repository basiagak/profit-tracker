// Command server wires configuration, the database, and both client
// surfaces (JSON API and Telegram bot) together and runs them concurrently
// until an interrupt/terminate signal triggers a graceful shutdown.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/joho/godotenv"
	"golang.org/x/sync/errgroup"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fsetiawan29/profit-tracker/internal/api"
	"github.com/fsetiawan29/profit-tracker/internal/api/handlers"
	"github.com/fsetiawan29/profit-tracker/internal/config"
	"github.com/fsetiawan29/profit-tracker/internal/repository"
	"github.com/fsetiawan29/profit-tracker/internal/service"
	"github.com/fsetiawan29/profit-tracker/internal/telegram"
	"github.com/fsetiawan29/profit-tracker/migrations"
)

// shutdownTimeout bounds how long the HTTP server waits for in-flight
// requests to finish once a shutdown signal is received.
const shutdownTimeout = 10 * time.Second

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on system environment")
	}

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get underlying sql.DB: %w", err)
	}
	defer sqlDB.Close()

	if err := applyMigrations(sqlDB); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	users := repository.NewUserRepository(db)
	ingredients := repository.NewIngredientRepository(db)
	items := repository.NewItemRepository(db)
	catalog := service.NewCatalogService(ingredients, items)
	sessions := api.NewSessionManager(cfg.SessionSecret, false)

	e := api.NewEcho(nil)
	groups := api.NewRouteGroups(e, sessions.Middleware())
	handlers.NewAuthHandler(cfg.TelegramToken, users, sessions).Register(groups.Auth, groups.Protected)
	handlers.NewIngredientsHandler(catalog).Register(groups.Protected)
	handlers.NewItemsHandler(catalog).Register(groups.Protected)

	bot, err := telegram.NewBot(cfg.TelegramToken, users)
	if err != nil {
		return fmt.Errorf("create telegram bot: %w", err)
	}
	bot.RegisterCatalogCommands(catalog)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Printf("http server listening on %s", cfg.HTTPAddr)
		if err := e.Start(cfg.HTTPAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		log.Println("telegram bot polling started")
		if err := bot.Run(gCtx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("telegram bot: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		log.Println("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := e.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http server shutdown: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}
	log.Println("shutdown complete")
	return nil
}

// applyMigrations runs every pending migration embedded in the migrations
// package against sqlDB. It is idempotent: an already up-to-date schema
// (migrate.ErrNoChange) is not an error.
func applyMigrations(sqlDB *sql.DB) error {
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}

	dbDriver, err := migratepostgres.WithInstance(sqlDB, &migratepostgres.Config{})
	if err != nil {
		return fmt.Errorf("create postgres migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", dbDriver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
