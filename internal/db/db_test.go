package db_test

import (
	"path/filepath"
	"testing"
	"uk-rail-schedule-api/internal/db"
	"uk-rail-schedule-api/internal/schedule"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOpen_NewDatabase_CreatesSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("expected no error opening new database, got: %v", err)
	}

	if err := database.Create(&schedule.Schedule{CIFTrainUID: "A00001"}).Error; err != nil {
		t.Fatalf("expected to insert schedule into new database, got: %v", err)
	}
}

// TestOpen_ExistingDatabase_MigratesNewColumns simulates the scenario where the database was
// created before new columns were added to the Schedule struct. It creates a database with an
// old schema (missing several columns), then calls db.Open to verify that AutoMigrate runs on
// every open — not just when the database is brand new — so inserts succeed after the migration.
func TestOpen_ExistingDatabase_MigratesNewColumns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "existing.db")

	// Simulate an "old" database by creating it with only a subset of the Schedule columns.
	oldDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("failed to create legacy database: %v", err)
	}
	// AutoMigrate a cut-down version of the table that is missing derived fields
	// (e.g. time_of_departure_from_origin) that were added in later versions.
	type OldSchedule struct {
		ID              uint64 `gorm:"primaryKey"`
		CIFTrainUID     string
		CIFStpIndicator string
		SignallingID    string
		Source          string
	}
	if err := oldDB.AutoMigrate(&OldSchedule{}); err != nil {
		t.Fatalf("failed to migrate legacy schema: %v", err)
	}

	// Close the legacy connection.
	sqlDB, _ := oldDB.DB()
	sqlDB.Close()

	// Re-open via db.Open — this must apply AutoMigrate and add the missing columns.
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open on existing database failed: %v", err)
	}

	// Inserting a full Schedule record must succeed; if new columns are absent this returns
	// the "table has no column named ..." error that triggered this fix.
	sch := schedule.Schedule{
		CIFTrainUID:              "T99999",
		CIFStpIndicator:          "N",
		Source:                   "VSTP",
		TimeOfDepartureFromOrigin: "0800",
	}
	if err := database.Create(&sch).Error; err != nil {
		t.Fatalf("expected insert to succeed after migration, got: %v", err)
	}
}
