package db

import (
	"log/slog"
	"os"
	"uk-rail-schedule-api/internal/schedule"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Open opens (or creates) the SQLite database at the given path and returns a GORM handle.
func Open(databaseFilename string) (*gorm.DB, error) {
	if _, err := os.Stat(databaseFilename); os.IsNotExist(err) {
		slog.Info("Database doesn't exist - creating", "databaseFilename", databaseFilename)
	}

	database, err := gorm.Open(sqlite.Open(databaseFilename), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := database.AutoMigrate(
		&schedule.ScheduleLocation{},
		&schedule.Schedule{},
		&schedule.Tiploc{},
		&schedule.Timetable{},
	); err != nil {
		return nil, err
	}

	return database, nil
}
