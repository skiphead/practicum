package config

import (
	"gopkg.in/yaml.v3"
	"os"
)

// LoadConfig загружает конфигурацию из YAML-файла
func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}

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
