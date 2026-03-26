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
	isNew := false
	if _, err := os.Stat(databaseFilename); os.IsNotExist(err) {
		isNew = true
		slog.Info("Database doesn't exist - creating", "databaseFilename", databaseFilename)
	}

	database, err := gorm.Open(sqlite.Open(databaseFilename), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if isNew {
		database.AutoMigrate(
			&schedule.ScheduleLocation{},
			&schedule.Schedule{},
			&schedule.Tiploc{},
			&schedule.Timetable{},
		)
	}

	return database, nil
}
