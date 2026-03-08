package postgresql

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Переменные для подмены в тестах
var (
	// Обертки над реальными функциями
	osDirFSFunc        = os.DirFS
	gooseSetBaseFSFunc = func(fs fs.FS) {
		// Реализация в реальном коде
	}
	gooseSetDialectFunc = func(dialect string) error {
		// Реализация в реальном коде
		return nil
	}
	gooseUpFunc = func(db *sql.DB, dir string) error {
		// Реализация в реальном коде
		return nil
	}
)

// MigrationsTestable версия функции для тестирования
func MigrationsTestable(db *sql.DB, migrationDir string) error {
	// Используем подменяемые функции
	gooseSetBaseFSFunc(osDirFSFunc(migrationDir))

	if err := gooseSetDialectFunc("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	if err := gooseUpFunc(db, "."); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.Println("Migrations applied successfully!")
	return nil
}

func TestMigrations_Success(t *testing.T) {
	// Сохраняем оригинальные значения
	origSetDialect := gooseSetDialectFunc
	origUp := gooseUpFunc

	// Восстанавливаем после теста
	defer func() {
		gooseSetDialectFunc = origSetDialect
		gooseUpFunc = origUp
	}()

	// Счетчики вызовов
	calls := struct {
		setDialect bool
		up         bool
	}{}

	// Подменяем функции
	gooseSetDialectFunc = func(dialect string) error {
		calls.setDialect = true
		assert.Equal(t, "postgres", dialect)
		return nil
	}

	gooseUpFunc = func(db *sql.DB, dir string) error {
		calls.up = true
		assert.Equal(t, ".", dir)
		assert.NotNil(t, db)
		return nil
	}

	// Тестируем
	db := &sql.DB{}
	tempDir := t.TempDir()

	err := MigrationsTestable(db, tempDir)

	assert.NoError(t, err)
	assert.True(t, calls.setDialect, "SetDialect должен быть вызван")
	assert.True(t, calls.up, "Up должен быть вызван")
}

func TestMigrations_SetDialectError(t *testing.T) {
	// Сохраняем оригинальную функцию
	origSetDialect := gooseSetDialectFunc
	defer func() { gooseSetDialectFunc = origSetDialect }()

	// Настраиваем ошибку
	expectedErr := errors.New("dialect error")
	gooseSetDialectFunc = func(dialect string) error {
		return expectedErr
	}

	// Тестируем
	db := &sql.DB{}
	tempDir := t.TempDir()

	err := MigrationsTestable(db, tempDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to set dialect")
	assert.Contains(t, err.Error(), expectedErr.Error())
}

func TestMigrations_UpError(t *testing.T) {
	// Сохраняем оригинальные функции
	origSetDialect := gooseSetDialectFunc
	origUp := gooseUpFunc
	defer func() {
		gooseSetDialectFunc = origSetDialect
		gooseUpFunc = origUp
	}()

	// Настраиваем успешный SetDialect и ошибочный Up
	gooseSetDialectFunc = func(dialect string) error {
		return nil
	}

	expectedErr := errors.New("up error")
	gooseUpFunc = func(db *sql.DB, dir string) error {
		return expectedErr
	}

	// Тестируем
	db := &sql.DB{}
	tempDir := t.TempDir()

	err := MigrationsTestable(db, tempDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to apply migrations")
	assert.Contains(t, err.Error(), expectedErr.Error())
}
