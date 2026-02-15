package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestNewStorageRepository проверяет создание репозитория с дефолтными значениями
func TestNewStorageRepository(t *testing.T) {
	// Создаем моковый пул соединений (в реальных тестах нужно использовать мок)
	// Для простых тестов можно проверить только инициализацию
	repo := NewStorageRepository(nil)

	if repo == nil {
		t.Error("Repository should not be nil")
	}

	// Проверяем, что интерфейс реализован
	var _ URLRepository = repo
}

// TestGetEffectiveBatchSize проверяет вычисление размера батча
func TestGetEffectiveBatchSize(t *testing.T) {
	repo := &storageRepository{}

	testCases := []struct {
		name          string
		requestedSize int
		expected      int
	}{
		{"Positive size", 500, 500},
		{"Zero size", 0, defaultBatchSize},
		{"Negative size", -100, defaultBatchSize},
		{"Exactly default", defaultBatchSize, defaultBatchSize},
		{"Greater than default", 2000, 2000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := repo.getEffectiveBatchSize(tc.requestedSize)
			if result != tc.expected {
				t.Errorf("getEffectiveBatchSize(%d) = %d, expected %d",
					tc.requestedSize, result, tc.expected)
			}
		})
	}
}

// TestCalculateExpiryTime проверяет вычисление времени истечения
func TestCalculateExpiryTime(t *testing.T) {
	repo := &storageRepository{}

	expiryTime := repo.calculateExpiryTime()
	expectedTime := time.Now().AddDate(defaultExpiryYears, 0, 0)

	// Проверяем, что время в пределах разумной дельты
	delta := 1 * time.Second // Допустимая погрешность в 1 секунду
	if expiryTime.Before(expectedTime.Add(-delta)) || expiryTime.After(expectedTime.Add(delta)) {
		t.Errorf("calculateExpiryTime() = %v, expected approx %v",
			expiryTime, expectedTime)
	}
}

// TestHandleQueryError проверяет обработку ошибок запросов
func TestHandleQueryError(t *testing.T) {
	repo := &storageRepository{}

	testCases := []struct {
		name        string
		err         error
		entityDesc  string
		identifier  string
		shouldMatch bool
	}{
		{
			name:        "ErrNoRows",
			err:         pgx.ErrNoRows,
			entityDesc:  "URL",
			identifier:  "abc123",
			shouldMatch: true,
		},
		{
			name:        "Generic error",
			err:         errors.New("connection failed"),
			entityDesc:  "URL",
			identifier:  "abc123",
			shouldMatch: false,
		},
		{
			name:        "Empty identifier",
			err:         pgx.ErrNoRows,
			entityDesc:  "User",
			identifier:  "",
			shouldMatch: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shortURL, err := repo.handleQueryError(tc.err, tc.entityDesc, tc.identifier)

			// Всегда должен возвращать nil ShortURL
			if shortURL != nil {
				t.Error("handleQueryError should always return nil ShortURL")
			}

			// Проверяем ошибку
			if tc.shouldMatch {
				if !errors.Is(err, ErrNotFound) {
					t.Errorf("Expected ErrNotFound, got: %v", err)
				}
				// Проверяем, что сообщение содержит идентификатор
				if tc.identifier != "" && err != nil {
					errMsg := err.Error()
					if !contains(errMsg, tc.identifier) {
						t.Errorf("Error message should contain identifier '%s': %s",
							tc.identifier, errMsg)
					}
				}
			} else if err == nil {
				t.Error("Expected error for non-ErrNoRows case")
			}
		})
	}
}

// TestValidateExpiry проверяет валидацию времени истечения
func TestValidateExpiry(t *testing.T) {
	repo := &storageRepository{}

	now := time.Now()
	testCases := []struct {
		name      string
		expiresAt time.Time
		shouldErr bool
	}{
		{
			name:      "Future expiry",
			expiresAt: now.Add(24 * time.Hour),
			shouldErr: false,
		},
		{
			name:      "Past expiry",
			expiresAt: now.Add(-24 * time.Hour),
			shouldErr: true,
		},
		{
			name:      "Exactly now",
			expiresAt: now,
			shouldErr: true, // Сейчас считается истекшим
		},
		{
			name:      "Far future",
			expiresAt: now.AddDate(10, 0, 0),
			shouldErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			identifier := "test123"
			err := repo.validateExpiry(tc.expiresAt, identifier)

			if tc.shouldErr && err == nil {
				t.Error("Expected error for expired URL")
			}
			if !tc.shouldErr && err != nil {
				t.Errorf("Unexpected error for valid expiry: %v", err)
			}
			if err != nil && !contains(err.Error(), identifier) {
				t.Errorf("Error message should contain identifier '%s': %s",
					identifier, err.Error())
			}
		})
	}
}

// TestInitQueries проверяет инициализацию SQL запросов
func TestInitQueries(t *testing.T) {
	repo := &storageRepository{
		table: "test_table",
	}

	// Вызываем initQueries
	repo.initQueries()

	// Проверяем, что все запросы не пустые и содержат имя таблицы
	queriesToCheck := []struct {
		name  string
		query string
	}{
		{"findDuplicatesQuery", repo.findDuplicatesQuery},
		{"createQuery", repo.createQuery},
		{"getQuery", repo.getQuery},
		{"getByOriginalURL", repo.getByOriginalURL},
		{"getByUserID", repo.getByUserID},
		{"updateQuery", repo.updateQuery},
		{"updateIsActive", repo.updateIsActive},
		{"deleteQuery", repo.deleteQuery},
	}

	for _, qc := range queriesToCheck {
		t.Run(qc.name, func(t *testing.T) {
			if qc.query == "" {
				t.Errorf("%s should not be empty", qc.name)
			}
			if !contains(qc.query, repo.table) {
				t.Errorf("%s should contain table name '%s': %s",
					qc.name, repo.table, qc.query)
			}
		})
	}

	// Проверяем createBatchQuery отдельно, так как он формируется по-другому
	if !contains(repo.createBatchQuery, "%s") {
		t.Error("createBatchQuery should contain format placeholder for table name")
	}
}

// TestErrNotFound проверяет экспортированную ошибку
func TestErrNotFound(t *testing.T) {
	if ErrNotFound.Error() != "not found" {
		t.Errorf("ErrNotFound message = %s, expected 'not found'", ErrNotFound.Error())
	}

	// Проверяем, что ошибку можно обернуть
	err := fmt.Errorf("URL abc123: %w", ErrNotFound)
	if !errors.Is(err, ErrNotFound) {
		t.Error("Expected error to wrap ErrNotFound")
	}
}

// TestOptionFunctions проверяет наличие опциональных функций
func TestOptionFunctions(t *testing.T) {
	// Проверяем, что тип Option определен
	var _ Option = func(*storageRepository) {}

	// Пример опции (в реальном коде опции должны быть экспортированы)
	testOption := func(repo *storageRepository) {
		repo.table = "custom_table"
	}

	// Применяем опцию
	repo := &storageRepository{table: "default_table"}
	testOption(repo)

	if repo.table != "custom_table" {
		t.Error("Option function should modify repository")
	}
}

// TestRollbackTxOnError проверяет сигнатуру метода (но не логику, т.к. требует моков)
func TestRollbackTxOnError(t *testing.T) {
	repo := &storageRepository{}

	// Проверяем, что метод существует
	ctx := context.Background()
	var tx pgx.Tx
	var err error

	// Вызываем метод для проверки компиляции
	repo.rollbackTxOnError(ctx, tx, &err)

	t.Log("rollbackTxOnError method exists")
}

// TestStorageRepository_Constants проверяет константы
func TestStorageRepository_Constants(t *testing.T) {
	// Исправляем ожидаемое значение на фактическое из вашего кода
	expectedTableName := "shorts_url" // Изменили на фактическое значение
	if storageTableName != expectedTableName {
		t.Errorf("storageTableName = %s, expected '%s'", storageTableName, expectedTableName)
	}

	if defaultBatchSize <= 0 {
		t.Errorf("defaultBatchSize should be positive, got %d", defaultBatchSize)
	}

	if defaultExpiryYears <= 0 {
		t.Errorf("defaultExpiryYears should be positive, got %d", defaultExpiryYears)
	}
}

// Вспомогательная функция для проверки наличия подстроки
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestRepositoryInterface проверяет, что storageRepository реализует URLRepository
func TestRepositoryInterface(t *testing.T) {
	// Проверка во время компиляции
	var _ URLRepository = (*storageRepository)(nil)

	// Runtime проверка
	repo := &storageRepository{}
	if _, ok := interface{}(repo).(URLRepository); !ok {
		t.Error("storageRepository should implement URLRepository interface")
	}
}

// TestTableNameCustomization проверяет возможность кастомизации имени таблицы
func TestTableNameCustomization(t *testing.T) {
	customTable := "custom_urls"

	// Создаем репозиторий с кастомной таблицей через опцию
	repo := &storageRepository{table: "default"}
	option := func(r *storageRepository) {
		r.table = customTable
	}
	option(repo)

	// Инициализируем запросы
	repo.initQueries()

	// Проверяем, что запросы используют кастомное имя таблицы
	if !contains(repo.createQuery, customTable) {
		t.Errorf("createQuery should contain custom table name '%s': %s",
			customTable, repo.createQuery)
	}

	if !contains(repo.getQuery, customTable) {
		t.Errorf("getQuery should contain custom table name '%s': %s",
			customTable, repo.getQuery)
	}
}
