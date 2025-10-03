package config

import (
	"flag"
	"gopkg.in/yaml.v3"
	"os"
)

// LoadConfig загружает конфигурацию из YAML-файла
func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}

	flag.StringVar(&config.ServerAddr, "a", "localhost:8080", "Порт для запуска сервера")
	flag.StringVar(&config.BaseURL, "b", "", "Базовый адрес результирующего сокращённого URL (например: http://localhost:8000/qsd54gFg)")
	flag.Parse()

	// Если порт передан аргументом, он имеет высший приоритет и перезаписывает конфиг
	if config.ServerAddr != "" {
		return config, nil
	}

	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(file, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
