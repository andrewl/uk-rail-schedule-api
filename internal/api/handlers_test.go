package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"uk-rail-schedule-api/internal/api"
	"uk-rail-schedule-api/internal/schedule"
	"uk-rail-schedule-api/internal/store"
	internalsync "uk-rail-schedule-api/internal/sync"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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

// seedSchedule inserts a minimal schedule running on Sundays into the given DB.
func seedSchedule(t *testing.T, db *gorm.DB, signallingID, trainUID string) {
	t.Helper()
	sch := schedule.Schedule{
		CIFStpIndicator:   "P",
		SignallingID:      signallingID,
		CIFTrainUID:       trainUID,
		Source:            "Feed",
		ScheduleDaysRuns:  "0000001", // runs on Sunday
		ScheduleStartDate: "2023-01-01",
		ScheduleEndDate:   "2099-12-31",
		AtocCode:          "GW",
	}
	sch.AugmentSchedule()
	if err := db.Create(&sch).Error; err != nil {
		t.Fatal("failed to seed schedule:", err)
	}
}

// buildRouter wires a chi router with the JSON API routes using the given handler.
func buildRouter(h *api.Handler) http.Handler {
	r := chi.NewRouter()
	r.Route("/schedules/{identifierType}/{identifier}", func(r chi.Router) {
		r.Use(h.SchedulesCtx)
		r.Get("/", h.GetSchedules)
	})
	r.Route("/status", func(r chi.Router) {
		r.Use(h.StatusCtx)
		r.Get("/", h.GetStatus)
	})
	r.Post("/refresh", h.RunRefresh)
	return r
}

func TestGetSchedules_NotFound(t *testing.T) {
	db := setupTestDB(t)
	h := &api.Handler{Store: store.New(db, "test")}
	router := buildRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/schedules/headcode/9Z99?date=2023-05-21", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestGetSchedules_ReturnsJSON(t *testing.T) {
	db := setupTestDB(t)
	seedSchedule(t, db, "2A20", "C00206")
	h := &api.Handler{Store: store.New(db, "test")}
	router := buildRouter(h)

	// 2023-05-21 is a Sunday — matches ScheduleDaysRuns "0000001"
	req := httptest.NewRequest(http.MethodGet, "/schedules/headcode/2A20?date=2023-05-21", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp api.ScheduleAPIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.IdentifierType != "headcode" {
		t.Errorf("expected identifierType 'headcode', got %q", resp.IdentifierType)
	}
	if resp.Identifier != "2A20" {
		t.Errorf("expected identifier '2A20', got %q", resp.Identifier)
	}
	if len(resp.Schedules) != 1 {
		t.Errorf("expected 1 schedule, got %d", len(resp.Schedules))
	}
}

func TestGetSchedules_InvalidIdentifierType(t *testing.T) {
	db := setupTestDB(t)
	h := &api.Handler{Store: store.New(db, "test")}
	router := buildRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/schedules/INVALID/2A20?date=2023-05-21", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestGetSchedules_WithDateParam(t *testing.T) {
	db := setupTestDB(t)
	seedSchedule(t, db, "2A20", "C00206")
	h := &api.Handler{Store: store.New(db, "test")}
	router := buildRouter(h)

	// Querying on a Monday — schedule only runs Sundays, so should get 404
	req := httptest.NewRequest(http.MethodGet, "/schedules/headcode/2A20?date=2023-05-22", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for Monday query on Sunday-only schedule, got %d", rec.Code)
	}
}

func TestGetSchedules_WithTOCFilter(t *testing.T) {
	db := setupTestDB(t)
	seedSchedule(t, db, "2A20", "C00206") // AtocCode: "GW"
	h := &api.Handler{Store: store.New(db, "test")}
	router := buildRouter(h)

	// Filter by a different TOC — should not match
	req := httptest.NewRequest(http.MethodGet, "/schedules/headcode/2A20?date=2023-05-21&toc=LN", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 when filtering by non-matching TOC, got %d", rec.Code)
	}

	// Filter by the correct TOC — should match
	req2 := httptest.NewRequest(http.MethodGet, "/schedules/headcode/2A20?date=2023-05-21&toc=GW", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200 when filtering by matching TOC, got %d", rec2.Code)
	}
}

func TestGetStatus_ReturnsCounts(t *testing.T) {
	db := setupTestDB(t)
	seedSchedule(t, db, "2A20", "C00206")
	h := &api.Handler{Store: store.New(db, "v1.0")}
	router := buildRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var status store.APIStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}
	if status.ScheduleFileCount != 1 {
		t.Errorf("expected ScheduleFileCount 1, got %d", status.ScheduleFileCount)
	}
	if status.Version != "v1.0" {
		t.Errorf("expected Version 'v1.0', got %q", status.Version)
	}
}

func TestRunRefresh_WhenIdle(t *testing.T) {
	db := setupTestDB(t)
	// Use a non-existent feed file so the goroutine exits immediately
	h := &api.Handler{
		Store:            store.New(db, "test"),
		ScheduleFeedFile: "/nonexistent/feed.json",
		DataDir:          t.TempDir(),
	}
	router := buildRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
}

func TestRunRefresh_WhenBusy(t *testing.T) {
	internalsync.SetRefreshingDatabase(true)
	t.Cleanup(func() { internalsync.SetRefreshingDatabase(false) })

	db := setupTestDB(t)
	h := &api.Handler{Store: store.New(db, "test")}
	router := buildRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}
