package sync_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"uk-rail-schedule-api/internal/schedule"
	internalsync "uk-rail-schedule-api/internal/sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB returns a fresh in-memory SQLite database with the schedule schema applied.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		t.Fatal("failed to open test database:", err)
	}
	if err := db.AutoMigrate(
		&schedule.ScheduleLocation{},
		&schedule.Schedule{},
		&schedule.Tiploc{},
		&schedule.Timetable{},
	); err != nil {
		t.Fatal("failed to migrate test database:", err)
	}
	return db
}

// writeFeedFile writes a line-delimited feed file to a temp path and returns its path.
func writeFeedFile(t *testing.T, lines ...string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "feed-*.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		fmt.Fprintln(f, line)
	}
	f.Close()
	return f.Name()
}

// Minimal valid feed lines used across multiple tests.
const (
	metadataLine = `{"JsonTimetableV1":{"classification":"full","timestamp":1683043200,"owner":"Network Rail","Sender":{"organisation":"Network Rail","application":"NTROD","component":"CIF_TO_JSON"},"Metadata":{"type":"CIF_FULL_DAILY","sequence":1}}}`

	scheduleLine = `{"JsonScheduleV1":{"CIF_bank_holiday_running":"","CIF_stp_indicator":"P","CIF_train_uid":"C00206","applicable_timetable":"Y","atoc_code":"GW","new_schedule_segment":{"traction_class":"","uic_code":""},"schedule_days_runs":"0000001","schedule_end_date":"2099-12-31","schedule_segment":{"signalling_id":"2A20","CIF_train_category":"OO","CIF_headcode":"","CIF_course_indicator":1,"CIF_train_service_code":"22209000","CIF_business_sector":"??","CIF_power_type":"DMU","CIF_timing_load":"E","CIF_speed":"100","CIF_operating_characteristics":"D","CIF_train_class":"B","CIF_sleepers":"","CIF_reservations":"","CIF_connection_indicator":"","CIF_catering_code":"","CIF_service_branding":"","schedule_location":[{"record_identity":"LO","tiploc_code":"DRBY","departure":"0756","public_departure":"0756"}]},"schedule_start_date":"2023-01-01","train_status":"P","transaction_type":"Create"}}`

	tiplocLine = `{"TiplocV1":{"transaction_type":"Create","tiploc_code":"DRBY","nalco":"161050","stanox":"52101","crs_code":"DBY","description":"DERBY","tps_description":"DERBY"}}`
)

func TestIsRefreshingDatabase_InitiallyFalse(t *testing.T) {
	// Ensure any previous test cleaned up.
	internalsync.SetRefreshingDatabase(false)
	if internalsync.IsRefreshingDatabase() {
		t.Error("expected IsRefreshingDatabase() to return false before any refresh")
	}
}

func TestRefreshSchedules_SkipsWhenAlreadyRefreshing(t *testing.T) {
	internalsync.SetRefreshingDatabase(true)
	t.Cleanup(func() { internalsync.SetRefreshingDatabase(false) })

	db := setupTestDB(t)
	internalsync.RefreshSchedules("irrelevant.json", db, t.TempDir(), false)

	var count int64
	db.Model(&schedule.Schedule{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 schedules inserted when already refreshing, got %d", count)
	}
}

func TestRefreshSchedules_FileNotFound(t *testing.T) {
	db := setupTestDB(t)
	// Should return gracefully without panicking.
	internalsync.RefreshSchedules("/nonexistent/path/feed.json", db, t.TempDir(), false)

	var count int64
	db.Model(&schedule.Schedule{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 schedules when feed file is missing, got %d", count)
	}
}

func TestRefreshSchedules_InvalidFirstLine(t *testing.T) {
	db := setupTestDB(t)
	feedFile := writeFeedFile(t, "this is not valid json")
	internalsync.RefreshSchedules(feedFile, db, t.TempDir(), false)

	var count int64
	db.Model(&schedule.Schedule{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 schedules when first line is invalid JSON, got %d", count)
	}
}

func TestRefreshSchedules_FirstLineNotMetadata(t *testing.T) {
	db := setupTestDB(t)
	// First line is a valid schedule record, not a timetable metadata record.
	feedFile := writeFeedFile(t, scheduleLine)
	internalsync.RefreshSchedules(feedFile, db, t.TempDir(), false)

	var count int64
	db.Model(&schedule.Schedule{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 schedules when first line is not metadata, got %d", count)
	}
}

func TestRefreshSchedules_LoadsSchedulesAndTiplocs(t *testing.T) {
	db := setupTestDB(t)
	feedFile := writeFeedFile(t, metadataLine, scheduleLine, tiplocLine)
	internalsync.RefreshSchedules(feedFile, db, t.TempDir(), false)

	var schedCount int64
	db.Model(&schedule.Schedule{}).Count(&schedCount)
	if schedCount != 1 {
		t.Errorf("expected 1 schedule loaded from feed, got %d", schedCount)
	}

	var tiplocCount int64
	db.Model(&schedule.Tiploc{}).Count(&tiplocCount)
	if tiplocCount != 1 {
		t.Errorf("expected 1 tiploc loaded from feed, got %d", tiplocCount)
	}

	var timetableCount int64
	db.Model(&schedule.Timetable{}).Count(&timetableCount)
	if timetableCount != 1 {
		t.Errorf("expected 1 timetable record saved, got %d", timetableCount)
	}
}

func TestRefreshSchedules_ScheduleIsAugmented(t *testing.T) {
	db := setupTestDB(t)
	feedFile := writeFeedFile(t, metadataLine, scheduleLine)
	internalsync.RefreshSchedules(feedFile, db, t.TempDir(), false)

	var sch schedule.Schedule
	db.First(&sch)

	if sch.SignallingID != "2A20" {
		t.Errorf("expected SignallingID '2A20', got %q", sch.SignallingID)
	}
	if sch.CIFPowerTypeDescription == "" {
		t.Error("expected CIFPowerTypeDescription to be populated after augmentation")
	}
	if sch.ScheduleStartDateTS == 0 {
		t.Error("expected ScheduleStartDateTS to be computed by augmentation")
	}
}

func TestRefreshSchedules_SkipsOlderFeedThanDatabase(t *testing.T) {
	db := setupTestDB(t)

	// Seed the DB with a timetable newer than the feed file's timestamp (1683043200).
	db.Create(&schedule.Timetable{
		Classification: "full",
		Timestamp:      9999999999,
		Owner:          "Network Rail",
	})

	feedFile := writeFeedFile(t, metadataLine, scheduleLine)
	internalsync.RefreshSchedules(feedFile, db, t.TempDir(), false)

	var count int64
	db.Model(&schedule.Schedule{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 schedules when feed is older than DB timetable, got %d", count)
	}
}

func TestRefreshSchedules_DeletesExpiredSchedules(t *testing.T) {
	db := setupTestDB(t)

	// Insert a schedule whose end date is firmly in the past.
	expired := schedule.Schedule{
		SignallingID: "9Z99",
		CIFTrainUID:  "Z99999",
		Source:       "Feed",
		ScheduleStartDate: "2020-01-01",
		ScheduleEndDate:   "2020-12-31",
	}
	expired.AugmentSchedule()
	db.Create(&expired)

	feedFile := writeFeedFile(t, metadataLine)
	internalsync.RefreshSchedules(feedFile, db, t.TempDir(), true)

	var count int64
	db.Model(&schedule.Schedule{}).Count(&count)
	if count != 0 {
		t.Errorf("expected expired schedule to be deleted, got %d remaining schedules", count)
	}
}

func TestRefreshSchedules_ReplaysVSTPFilesFromDataDir(t *testing.T) {
	db := setupTestDB(t)

	// Copy the vstp test fixture into a temp dataDir.
	dataDir := t.TempDir()
	vstpData, err := os.ReadFile(filepath.Join("..", "..", "test-fixtures", "vstp.json"))
	if err != nil {
		t.Fatal("failed to read vstp fixture:", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "vstp-replay.json"), vstpData, 0644); err != nil {
		t.Fatal("failed to write vstp replay file:", err)
	}

	feedFile := writeFeedFile(t, metadataLine)
	internalsync.RefreshSchedules(feedFile, db, dataDir, false)

	var count int64
	db.Model(&schedule.Schedule{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 VSTP schedule replayed from dataDir, got %d", count)
	}
}
