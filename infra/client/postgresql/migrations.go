package postgresql

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/pressly/goose/v3"
)

func Migrations(db *sql.DB, migrationDir string) error {

	// Set up migrations from the specified directory
	goose.SetBaseFS(os.DirFS(migrationDir))

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	// Apply migrations up to the latest version
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.Println("Migrations applied successfully!")
	return nil
}
