// Package initapp provides application initialization and lifecycle management.
package initapp

import (
	"flag"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/skiphead/practicum/internal/infra/config"
)

// LoadConfig reads configuration from YAML file or uses defaults.
func LoadConfig() *config.Config {
	pathConfig := "configs/config.json"

	var flagConfig string
	flag.StringVar(&flagConfig, "config", "", "Path to config file")
	flag.Parse()

	if flagConfig != "" {
		pathConfig = flagConfig
	}
	if env := os.Getenv("CONFIG"); env != "" {
		pathConfig = env
	}

	cfg, err := config.LoadConfig(pathConfig)
	if err != nil {
		cfg = config.NewDefaultConfig()
		zap.L().Info("Using default configuration after failed config load",
			zap.Error(err),
			zap.String("config_path", pathConfig))
	}

	if err = cfg.Validate(); err != nil {
		zap.L().Fatal("Configuration validation failed", zap.Error(err))
	}
	return cfg
}

// InitLogger configures and returns the global zap logger.
func InitLogger() *zap.Logger {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}

	zap.ReplaceGlobals(logger)
	return logger
}
