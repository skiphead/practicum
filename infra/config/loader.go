package config

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}

	if data, err := os.ReadFile(configPath); err == nil {
		if err = yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("ошибка парсинга YAML: %w", err)
		}
	}

	var flagServerAddr, flagBaseURL string
	flag.StringVar(&flagServerAddr, "a", "", "Порт для запуска сервера")
	flag.StringVar(&flagBaseURL, "b", "", "Базовый адрес результирующего сокращённого URL")
	flag.Parse()

	if flagServerAddr != "" {
		config.ServerAddr = flagServerAddr
	}
	if flagBaseURL != "" {
		config.BaseURL = flagBaseURL
	}

	if env := os.Getenv("BASE_URL"); env != "" {
		config.BaseURL = env
	}
	if env := os.Getenv("SERVER_ADDRESS"); env != "" {
		config.ServerAddr = env
	}

	if config.ServerAddr == "" {
		config.ServerAddr = "localhost:8080"
	}

	return config, nil
}
