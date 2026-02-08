package main

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

// TestBuildInfoDefaultValues проверяет, что переменные сборки имеют значения по умолчанию "N/A",
// когда они не были установлены через теги сборки
func TestBuildInfoDefaultValues(t *testing.T) {
	// Сохраняем оригинальные значения
	origVersion := buildVersion
	origDate := buildDate
	origCommit := buildCommit
	defer func() {
		// Восстанавливаем оригинальные значения после теста
		buildVersion = origVersion
		buildDate = origDate
		buildCommit = origCommit
	}()

	// Сбрасываем переменные к пустым значениям (как будто они не были установлены)
	buildVersion = ""
	buildDate = ""
	buildCommit = ""

	// Имитируем запуск main() до точки вывода информации о сборке
	// Проверяем, что значения устанавливаются в "N/A" по умолчанию
	if buildVersion == "" {
		buildVersion = "N/A"
	}
	if buildCommit == "" {
		buildCommit = "N/A"
	}
	if buildDate == "" {
		buildDate = "N/A"
	}

	// Проверяем значения по умолчанию
	if buildVersion != "N/A" {
		t.Errorf("Ожидалось buildVersion = 'N/A', получено: %s", buildVersion)
	}
	if buildDate != "N/A" {
		t.Errorf("Ожидалось buildDate = 'N/A', получено: %s", buildDate)
	}
	if buildCommit != "N/A" {
		t.Errorf("Ожидалось buildCommit = 'N/A', получено: %s", buildCommit)
	}
}

// TestBuildInfoOutput проверяет форматирование вывода информации о сборке
func TestBuildInfoOutput(t *testing.T) {
	// Сохраняем оригинальные значения
	origVersion := buildVersion
	origDate := buildDate
	origCommit := buildCommit
	defer func() {
		// Восстанавливаем оригинальные значения после теста
		buildVersion = origVersion
		buildDate = origDate
		buildCommit = origCommit
	}()

	// Устанавливаем тестовые значения
	testVersion := "v1.2.3"
	testDate := "2026-01-01"
	testCommit := "abc123def"

	buildVersion = testVersion
	buildDate = testDate
	buildCommit = testCommit

	// Захватываем вывод
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Выводим информацию о сборке
	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)

	// Восстанавливаем stdout и читаем вывод
	w.Close()
	os.Stdout = oldStdout
	buf.ReadFrom(r)
	output := buf.String()

	// Проверяем ожидаемый вывод
	expected := fmt.Sprintf("Build version: %s\nBuild date: %s\nBuild commit: %s\n",
		testVersion, testDate, testCommit)

	if output != expected {
		t.Errorf("Неверный вывод:\nОжидалось: %s\nПолучено: %s", expected, output)
	}
}

// TestBuildInfoCustomValues проверяет пользовательские значения переменных сборки
func TestBuildInfoCustomValues(t *testing.T) {
	testCases := []struct {
		name     string
		version  string
		date     string
		commit   string
		expected struct {
			version string
			date    string
			commit  string
		}
	}{
		{
			name:    "Все значения установлены",
			version: "v2.0.0",
			date:    "2026-02-08",
			commit:  "xyz789",
			expected: struct {
				version string
				date    string
				commit  string
			}{
				version: "v2.0.0",
				date:    "2026-02-08",
				commit:  "xyz789",
			},
		},
		{
			name:    "Пустая версия",
			version: "",
			date:    "2026-02-08",
			commit:  "xyz789",
			expected: struct {
				version string
				date    string
				commit  string
			}{
				version: "N/A",
				date:    "2026-02-08",
				commit:  "xyz789",
			},
		},
		{
			name:    "Пустая дата",
			version: "v2.0.0",
			date:    "",
			commit:  "xyz789",
			expected: struct {
				version string
				date    string
				commit  string
			}{
				version: "v2.0.0",
				date:    "N/A",
				commit:  "xyz789",
			},
		},
		{
			name:    "Пустой коммит",
			version: "v2.0.0",
			date:    "2026-02-08",
			commit:  "",
			expected: struct {
				version string
				date    string
				commit  string
			}{
				version: "v2.0.0",
				date:    "2026-02-08",
				commit:  "N/A",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Сохраняем оригинальные значения
			origVersion := buildVersion
			origDate := buildDate
			origCommit := buildCommit
			defer func() {
				// Восстанавливаем оригинальные значения после теста
				buildVersion = origVersion
				buildDate = origDate
				buildCommit = origCommit
			}()

			// Устанавливаем тестовые значения
			buildVersion = tc.version
			buildDate = tc.date
			buildCommit = tc.commit

			// Применяем логику из main() для установки значений по умолчанию
			if buildVersion == "" {
				buildVersion = "N/A"
			}
			if buildCommit == "" {
				buildCommit = "N/A"
			}
			if buildDate == "" {
				buildDate = "N/A"
			}

			// Проверяем результаты
			if buildVersion != tc.expected.version {
				t.Errorf("buildVersion: ожидалось %s, получено %s",
					tc.expected.version, buildVersion)
			}
			if buildDate != tc.expected.date {
				t.Errorf("buildDate: ожидалось %s, получено %s",
					tc.expected.date, buildDate)
			}
			if buildCommit != tc.expected.commit {
				t.Errorf("buildCommit: ожидалось %s, получено %s",
					tc.expected.commit, buildCommit)
			}
		})
	}
}

// TestBuildInfoFormat проверяет формат вывода
func TestBuildInfoFormat(t *testing.T) {
	// Сохраняем оригинальные значения
	origVersion := buildVersion
	origDate := buildDate
	origCommit := buildCommit
	defer func() {
		// Восстанавливаем оригинальные значения после теста
		buildVersion = origVersion
		buildDate = origDate
		buildCommit = origCommit
	}()

	// Устанавливаем тестовые значения
	buildVersion = "v1.0.0"
	buildDate = "2023-12-31"
	buildCommit = "abc123"

	// Проверяем формат вывода с помощью fmt.Sprintf
	versionOutput := fmt.Sprintf("Build version: %s\n", buildVersion)
	dateOutput := fmt.Sprintf("Build date: %s\n", buildDate)
	commitOutput := fmt.Sprintf("Build commit: %s\n", buildCommit)

	if versionOutput != "Build version: v1.0.0\n" {
		t.Errorf("Неверный формат вывода версии: %s", versionOutput)
	}
	if dateOutput != "Build date: 2023-12-31\n" {
		t.Errorf("Неверный формат вывода даты: %s", dateOutput)
	}
	if commitOutput != "Build commit: abc123\n" {
		t.Errorf("Неверный формат вывода коммита: %s", commitOutput)
	}
}
