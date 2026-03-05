// Main package for the URL shortening application.
// The application provides an HTTP server for processing URL shortening requests,
// using file storage or PostgreSQL database for data persistence.
package main

import (
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/skiphead/practicum/internal/initapp"
)

var buildVersion, buildDate, buildCommit string

// main is the entry point. Initializes components and starts the application.
func main() {
	printBuildInfo()

	logger := initapp.InitLogger()

	cfg := initapp.LoadConfig()

	app, err := initapp.NewApp(logger, cfg)
	if err != nil {
		logger.Fatal("failed to initialize application", zap.Error(err))
	}

	// Run блокирует выполнение до завершения работы приложения
	app.Run()

	// Sync logger после завершения работы
	if err := logger.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to sync logger: %v\n", err)
	}
}

// printBuildInfo displays version, date, and commit information.
func printBuildInfo() {
	if buildVersion == "" {
		buildVersion = "N/A"
	}
	if buildCommit == "" {
		buildCommit = "N/A"
	}
	if buildDate == "" {
		buildDate = "N/A"
	}

	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)
}
