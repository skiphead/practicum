package repository

import (
	"context"
	"os"
	"testing"

	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedFileStorage(t *testing.T) {
	// Создаем временный файл для тестов
	tmpFile, err := os.CreateTemp("", "test_storage_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Создаем хранилище
	storage, err := NewCachedFileStorage(tmpFile.Name())
	require.NoError(t, err)
	require.NotNil(t, storage)

	t.Run("Save and Get", func(t *testing.T) {
		err := storage.Save("user1", "corr1", "abc123", "https://example.com")
		assert.NoError(t, err)

		record, found, err := storage.Get("abc123")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.NotNil(t, record)
		assert.Equal(t, "https://example.com", record.OriginalURL)
		assert.Equal(t, "abc123", record.ShortCode)
		assert.Equal(t, "user1", record.UserID)
		assert.True(t, record.IsActive)
	})

	t.Run("Save duplicate URL", func(t *testing.T) {
		err := storage.Save("user2", "corr2", "def456", "https://example.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "URL already exists")
	})

	t.Run("Get non-existent", func(t *testing.T) {
		record, found, err := storage.Get("nonexistent")
		assert.NoError(t, err)
		assert.False(t, found)
		assert.Nil(t, record)
	})

	t.Run("GetByID", func(t *testing.T) {
		// Сначала сохраним запись
		err := storage.Save("user3", "corr3", "ghi789", "https://google.com")
		assert.NoError(t, err)

		// Получим запись по ShortCode чтобы узнать ID
		record, found, err := storage.Get("ghi789")
		assert.NoError(t, err)
		assert.True(t, found)

		// Теперь получим по ID
		recordByID, found, err := storage.GetByID(record.ID)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, record.ID, recordByID.ID)
		assert.Equal(t, "https://google.com", recordByID.OriginalURL)
	})

	t.Run("FindByOriginalURL", func(t *testing.T) {
		record, err := storage.FindByOriginalURL("https://example.com")
		assert.NoError(t, err)
		assert.NotNil(t, record)
		assert.Equal(t, "abc123", record.ShortCode)

		// Несуществующий URL
		record, err = storage.FindByOriginalURL("https://nonexistent.com")
		assert.NoError(t, err)
		assert.Nil(t, record)
	})

	t.Run("FindByUserID", func(t *testing.T) {
		// Сохраним еще одну запись для того же пользователя
		err := storage.Save("user1", "corr4", "jkl012", "https://user1-site.com")
		assert.NoError(t, err)

		records, err := storage.FindByUserID("user1")
		assert.NoError(t, err)
		assert.Len(t, records, 2)

		// Проверим, что все записи принадлежат user1
		for _, record := range records {
			assert.Equal(t, "user1", record.UserID)
			assert.True(t, record.IsActive)
		}

		// Несуществующий пользователь
		records, err = storage.FindByUserID("nonexistent")
		assert.NoError(t, err)
		assert.Empty(t, records)
	})

	t.Run("Delete", func(t *testing.T) {
		// Удаляем существующую запись
		err := storage.Delete("abc123")
		assert.NoError(t, err)

		// Проверяем, что запись помечена как неактивная
		record, found, err := storage.Get("abc123")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.False(t, record.IsActive)

		// Проверяем, что запись не находится по оригинальному URL
		record, err = storage.FindByOriginalURL("https://example.com")
		assert.NoError(t, err)
		assert.Nil(t, record)

		// Проверяем, что запись не возвращается в FindByUserID
		records, err := storage.FindByUserID("user1")
		assert.NoError(t, err)
		// Должна остаться только одна активная запись
		assert.Len(t, records, 1)
		assert.Equal(t, "jkl012", records[0].ShortCode)

		// Удаление несуществующей записи
		err = storage.Delete("nonexistent")
		assert.Error(t, err)
	})

	t.Run("DeleteByID", func(t *testing.T) {
		// Создаем запись для удаления
		err := storage.Save("user4", "corr5", "mno345", "https://delete-test.com")
		assert.NoError(t, err)

		record, found, err := storage.Get("mno345")
		assert.NoError(t, err)
		assert.True(t, found)

		// Удаляем по ID
		err = storage.DeleteByID(record.ID)
		assert.NoError(t, err)

		// Проверяем, что запись помечена как неактивная
		record, found, err = storage.Get("mno345")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.False(t, record.IsActive)
	})

	t.Run("BatchSave", func(t *testing.T) {
		batch := []entity.ShortURL{
			{
				UserID:        "batch_user",
				CorrelationID: "batch1",
				ShortCode:     "batch001",
				OriginalURL:   "https://batch1.com",
			},
			{
				UserID:        "batch_user",
				CorrelationID: "batch2",
				ShortCode:     "batch002",
				OriginalURL:   "https://batch2.com",
			},
		}

		err := storage.BatchSave(context.Background(), batch)
		assert.NoError(t, err)

		// Проверяем, что записи сохранились
		record1, found, err := storage.Get("batch001")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "https://batch1.com", record1.OriginalURL)

		record2, found, err := storage.Get("batch002")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "https://batch2.com", record2.OriginalURL)

		// Проверяем, что записи принадлежат правильному пользователю
		records, err := storage.FindByUserID("batch_user")
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})

	t.Run("SetDeletedByUserIDAndURLs", func(t *testing.T) {
		// Создаем несколько записей для пользователя
		err := storage.Save("test_user", "corr6", "test001", "https://test1.com")
		assert.NoError(t, err)
		err = storage.Save("test_user", "corr7", "test002", "https://test2.com")
		assert.NoError(t, err)
		err = storage.Save("test_user", "corr8", "test003", "https://test3.com")
		assert.NoError(t, err)

		// Помечаем две записи как удаленные
		shortURLs := []string{"test001", "test002"}
		err = storage.SetDeletedByUserIDAndURLs("test_user", shortURLs, true)
		assert.NoError(t, err)

		// Проверяем, что записи помечены как неактивные
		record1, found, err := storage.Get("test001")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.False(t, record1.IsActive)

		record2, found, err := storage.Get("test002")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.False(t, record2.IsActive)

		// Проверяем, что третья запись осталась активной
		record3, found, err := storage.Get("test003")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.True(t, record3.IsActive)

		// Восстанавливаем одну запись
		err = storage.SetDeletedByUserIDAndURLs("test_user", []string{"test001"}, false)
		assert.NoError(t, err)

		record1, found, err = storage.Get("test001")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.True(t, record1.IsActive)
	})

	t.Run("Stats", func(t *testing.T) {
		stats := storage.Stats()
		assert.NotNil(t, stats)

		// Проверяем наличие ожидаемых полей
		assert.Contains(t, stats, "total_records")
		assert.Contains(t, stats, "active_records")
		assert.Contains(t, stats, "deleted_records")
		assert.Contains(t, stats, "file_path")
		assert.Contains(t, stats, "file_size_bytes")
		assert.Contains(t, stats, "unique_users")

		total := stats["total_records"].(int)
		active := stats["active_records"].(int)
		deleted := stats["deleted_records"].(int)

		assert.Greater(t, total, 0)
		assert.GreaterOrEqual(t, active, 0)
		assert.GreaterOrEqual(t, deleted, 0)
		assert.Equal(t, total, active+deleted)
	})
}

func TestCachedFileStorage_EdgeCases(t *testing.T) {
	t.Run("Empty file", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test_empty_*.json")
		require.NoError(t, err)
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		storage, err := NewCachedFileStorage(tmpFile.Name())
		assert.NoError(t, err)
		assert.NotNil(t, storage)

		// Проверяем, что хранилище пустое
		stats := storage.Stats()
		assert.Equal(t, 0, stats["total_records"])
	})

	t.Run("Non-existent file directory", func(t *testing.T) {
		storage, err := NewCachedFileStorage("/non/existent/directory/file.json")
		assert.Error(t, err)
		assert.Nil(t, storage)
	})

	t.Run("Context cancellation in BatchSave", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test_context_*.json")
		require.NoError(t, err)
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		storage, err := NewCachedFileStorage(tmpFile.Name())
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Немедленно отменяем контекст

		batch := []entity.ShortURL{
			{
				UserID:      "user",
				ShortCode:   "code",
				OriginalURL: "https://test.com",
			},
		}

		err = storage.BatchSave(ctx, batch)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestCachedFileStorage_ConcurrentAccess(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_concurrent_*.json")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	storage, err := NewCachedFileStorage(tmpFile.Name())
	require.NoError(t, err)

	// Количество горутин
	numGoroutines := 10
	operationsPerGoroutine := 10

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(userID string) {
			for j := 0; j < operationsPerGoroutine; j++ {
				shortCode := userID + "_" + string(rune('a'+j))
				originalURL := "https://" + userID + "-" + string(rune('a'+j)) + ".com"

				// Сохраняем
				err := storage.Save(userID, "corr", shortCode, originalURL)
				assert.NoError(t, err)

				// Получаем
				record, found, err := storage.Get(shortCode)
				assert.NoError(t, err)
				assert.True(t, found)
				assert.Equal(t, originalURL, record.OriginalURL)

				// Ищем по пользователю
				records, err := storage.FindByUserID(userID)
				assert.NoError(t, err)
				assert.Greater(t, len(records), 0)
			}
			done <- true
		}("user" + string(rune('0'+i)))
	}

	// Ждем завершения всех горутин
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Проверяем итоговое состояние
	stats := storage.Stats()
	assert.Greater(t, stats["total_records"], 0)
	assert.Greater(t, stats["active_records"], 0)
}

func TestCachedFileStorage_Reopen(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_reopen_*.json")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Создаем первое хранилище и сохраняем данные
	storage1, err := NewCachedFileStorage(tmpFile.Name())
	require.NoError(t, err)

	err = storage1.Save("user1", "corr1", "code1", "https://example1.com")
	assert.NoError(t, err)
	err = storage1.Save("user1", "corr2", "code2", "https://example2.com")
	assert.NoError(t, err)

	// Удаляем одну запись
	err = storage1.Delete("code1")
	assert.NoError(t, err)

	// Создаем второе хранилище с тем же файлом
	storage2, err := NewCachedFileStorage(tmpFile.Name())
	require.NoError(t, err)

	// Проверяем, что данные сохранились после переоткрытия
	record1, found, err := storage2.Get("code1")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.False(t, record1.IsActive) // Должна быть неактивной

	record2, found, err := storage2.Get("code2")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.True(t, record2.IsActive) // Должна быть активной

	// Проверяем поиск по пользователю (только активные)
	records, err := storage2.FindByUserID("user1")
	assert.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, "code2", records[0].ShortCode)
}
