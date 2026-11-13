package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"strings"

	_ "github.com/lib/pq"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"tekstobot/internal/config"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations(db *sql.DB) error {
	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create iofs driver: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", d, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database migrations applied successfully")
	return nil
}

// dsnQuote escapa um valor para o formato key='value' do libpq,
// para que senhas com espaços, aspas ou barras não quebrem a conexão.
var dsnQuote = strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace

func GetDSN(cfg *config.Config) string {
	return fmt.Sprintf("host='%s' port='%s' user='%s' password='%s' dbname='%s' sslmode=disable",
		dsnQuote(cfg.DBHost), dsnQuote(cfg.DBPort), dsnQuote(cfg.DBUser), dsnQuote(cfg.DBPassword), dsnQuote(cfg.DBName))
}

func InitDB(cfg *config.Config) (*sql.DB, error) {
	connStr := GetDSN(cfg)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Successfully connected to the database")
	return db, nil
}
