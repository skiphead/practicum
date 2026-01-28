package config

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

const schema = "http"

func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}

	if data, err := os.ReadFile(configPath); err == nil {
		if err = yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("ошибка парсинга YAML: %w", err)
		}
	}

	var flagServerAddr, flagBaseURL, flagFileStoragePath, flagDataBaseDSN, flagAuditFile, flagAuditURL string
	flag.StringVar(&flagServerAddr, "a", "", "Порт для запуска сервера")
	flag.StringVar(&flagBaseURL, "b", "", "Базовый адрес результирующего сокращённого URL")
	flag.StringVar(&flagDataBaseDSN, "d", "", "user=postgres password=secret host=localhost port=5432 database=pgx_test sslmode=disable")
	flag.StringVar(&flagFileStoragePath, "f", "", "Путь до файла хранилища")
	flag.StringVar(&flagAuditFile, "audit-file", "", "Путь к файлу-приёмнику, в который сохраняются логи аудита.")
	flag.StringVar(&flagAuditURL, "audit-url", "", "Полный URL удаленного сервера-приёмника, куда отправляются логи аудита")

	flag.Parse()

	if flagServerAddr != "" {
		config.ServerAddr = flagServerAddr
	}
	if flagDataBaseDSN != "" {
		config.DatabaseDSN = flagDataBaseDSN
	}
	if flagBaseURL != "" {
		config.BaseURL = flagBaseURL
	}
	if flagAuditFile != "" {
		config.AuditFile = flagAuditFile
	}
	if flagAuditURL != "" {
		config.AuditURL = flagAuditURL
	}

	if env := os.Getenv("BASE_URL"); env != "" {
		config.BaseURL = env
	}
	if env := os.Getenv("SERVER_ADDRESS"); env != "" {
		config.ServerAddr = env
	}
	if env := os.Getenv("DATABASE_DSN"); env != "" {
		config.DatabaseDSN = env
	}
	if env := os.Getenv("FILE_STORAGE_PATH"); env != "" {
		config.FileStoragePath = env
	}
	if env := os.Getenv("AUDIT_FILE"); env != "" {
		config.AuditFile = env
	}
	if env := os.Getenv("AUDIT_URL"); env != "" {
		config.AuditURL = env
	}

	if config.DatabaseDSN == "" {
		config.DatabaseDSN = "user=postgres password=postgres host=localhost port=5432 database=pgx_test sslmode=disable"
	}

	if config.ServerAddr == "" {
		config.ServerAddr = "localhost:8080"
	}

	if config.BaseURL == "" {
		config.BaseURL = fmt.Sprintf("%s://%s", schema, config.ServerAddr)
	}

	if config.FileStoragePath == "" {
		config.FileStoragePath = "data.json"
	}

	return config, nil
}
