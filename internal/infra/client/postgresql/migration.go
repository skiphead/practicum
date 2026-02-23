// Package postgresql provides PostgreSQL database utilities including migration functionality.
package postgresql

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/pressly/goose/v3"
)

// GooseWrapper interface for abstracting goose operations
type GooseWrapper interface {
	SetBaseFS(fs fs.FS)
	SetDialect(dialect string) error
	Up(db *sql.DB, dir string) error
}

// RealGooseWrapper production implementation
type RealGooseWrapper struct{}

func (r RealGooseWrapper) SetBaseFS(fs fs.FS) {
	goose.SetBaseFS(fs)
}

func (r RealGooseWrapper) SetDialect(dialect string) error {
	return goose.SetDialect(dialect)
}

func (r RealGooseWrapper) Up(db *sql.DB, dir string) error {
	return goose.Up(db, dir)
}

// MigrationsWithDI runs migrations with dependency injection for testing
func MigrationsWithDI(db *sql.DB, migrationDir string, gooseWrapper GooseWrapper, fs fs.FS) error {
	gooseWrapper.SetBaseFS(fs)

	if err := gooseWrapper.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	if err := gooseWrapper.Up(db, "."); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.Println("Migrations applied successfully!")
	return nil
}

// Migrations runs database migrations from the specified directory
func Migrations(db *sql.DB, migrationDir string) error {
	fsDir := os.DirFS(migrationDir)
	wrapper := RealGooseWrapper{}
	return MigrationsWithDI(db, migrationDir, wrapper, fsDir)
}
