// syncd is a long-running daemon that keeps the schedule database up-to-date.
// It loads the initial schedule feed file into SQLite and listens for real-time
// VSTP updates via the Network Rail STOMP feed.
package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"uk-rail-schedule-api/internal/config"
	"uk-rail-schedule-api/internal/db"
	internalsync "uk-rail-schedule-api/internal/sync"

	"github.com/joho/godotenv"
)

// version is set at build time via -ldflags "-X main.version=<value>".
var version = "dev"

func main() {
	_ = godotenv.Load()

	logger := setupLogger()
	slog.SetDefault(logger)

	database, err := db.Open(config.GetDatabaseFilename())
	if err != nil {
		slog.Error("Failed to open database", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting schedule sync daemon", "version", version)

	// Initial load of schedule feed
	go internalsync.RefreshSchedules(
		config.GetScheduleFeedFilename(),
		database,
		config.GetDataDir(),
		config.ShouldDeleteExpiredSchedulesAfterRefresh(),
	)

	connErr, stompURL, login, password := config.GetStompConnectionDetails()
	if connErr != nil {
		slog.Warn("STOMP credentials not configured - VSTP feed will not be consumed", "error", connErr)
	} else {
		go internalsync.ListenForVSTP(database, stompURL, login, password, config.GetDataDir())
	}

	// Block until a termination signal is received
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down syncd")
}

// setupLogger configures the logger to write to a file if LOG_FILENAME is set, or to stderr otherwise.
func setupLogger() *slog.Logger {
	var output *os.File
	if logFile := os.Getenv("LOG_FILENAME"); logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			panic("Error opening log file: " + err.Error())
		}
		output = f
	} else {
		output = os.Stderr
	}
	return slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}))
}
