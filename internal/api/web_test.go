package api_test

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"uk-rail-schedule-api/internal/api"
	"uk-rail-schedule-api/internal/store"

	"github.com/go-chi/chi/v5"
)

// minimalTemplates builds an inline template set to avoid embedding real template files in tests.
func minimalTemplates(t *testing.T) *template.Template {
	t.Helper()
	funcMap := template.FuncMap{
		"now": func() time.Time { return time.Now() },
	}
	tmpl, err := template.New("index.html").Funcs(funcMap).Parse(`{{define "index.html"}}INDEX:{{.Version}}{{end}}`)
	if err != nil {
		t.Fatal("failed to parse index template:", err)
	}
	_, err = tmpl.New("partials/schedules.html").Parse(
		`{{define "partials/schedules.html"}}SCHEDULES:{{.Identifier}}:{{.Error}}:{{range .Schedules}}ITEM{{end}}{{end}}`,
	)
	if err != nil {
		t.Fatal("failed to parse schedules partial:", err)
	}
	_, err = tmpl.New("partials/status.html").Parse(
		`{{define "partials/status.html"}}STATUS:{{.ScheduleFileCount}}{{end}}`,
	)
	if err != nil {
		t.Fatal("failed to parse status partial:", err)
	}
	return tmpl
}

func buildWebRouter(wh *api.WebHandler) http.Handler {
	r := chi.NewRouter()
	r.Get("/", wh.GetIndex)
	r.Get("/search", wh.Search)
	r.Get("/status", wh.GetStatusPartial)
	return r
}

func TestGetIndex_RendersHTML(t *testing.T) {
	db := setupTestDB(t)
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "v1.0")},
		Templates: minimalTemplates(t),
	}
	router := buildWebRouter(wh)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "INDEX:v1.0") {
		t.Errorf("expected index output with version, got: %q", rec.Body.String())
	}
}

func TestSearch_NoResults(t *testing.T) {
	db := setupTestDB(t)
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "test")},
		Templates: minimalTemplates(t),
	}
	router := buildWebRouter(wh)

	req := httptest.NewRequest(http.MethodGet, "/search?identifier=9Z99&identifier_type=headcode&date=2023-05-21", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "SCHEDULES:9Z99::") {
		t.Errorf("expected schedules partial with identifier and no results, got: %q", body)
	}
	if strings.Contains(body, "ITEM") {
		t.Errorf("expected no schedule items, but found ITEM in: %q", body)
	}
}

func TestSearch_WithResults(t *testing.T) {
	db := setupTestDB(t)
	seedSchedule(t, db, "2A20", "C00206")
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "test")},
		Templates: minimalTemplates(t),
	}
	router := buildWebRouter(wh)

	req := httptest.NewRequest(http.MethodGet, "/search?identifier=2A20&identifier_type=headcode&date=2023-05-21", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "ITEM") {
		t.Errorf("expected at least one schedule item in output, got: %q", body)
	}
}

func TestSearch_DefaultsToHeadcodeType(t *testing.T) {
	db := setupTestDB(t)
	seedSchedule(t, db, "2A20", "C00206")
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "test")},
		Templates: minimalTemplates(t),
	}
	router := buildWebRouter(wh)

	// identifier_type omitted — should default to "headcode"
	req := httptest.NewRequest(http.MethodGet, "/search?identifier=2A20&date=2023-05-21", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ITEM") {
		t.Errorf("expected results when identifier_type defaults to headcode, got: %q", rec.Body.String())
	}
}

func TestSearch_DefaultsDateToToday(t *testing.T) {
	db := setupTestDB(t)
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "test")},
		Templates: minimalTemplates(t),
	}
	router := buildWebRouter(wh)

	// date omitted — should default to today without error
	req := httptest.NewRequest(http.MethodGet, "/search?identifier=2A20&identifier_type=headcode", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (even with no results), got %d", rec.Code)
	}
}

func TestSearch_InvalidIdentifierTypeRenderedInTemplate(t *testing.T) {
	db := setupTestDB(t)
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "test")},
		Templates: minimalTemplates(t),
	}
	router := buildWebRouter(wh)

	req := httptest.NewRequest(http.MethodGet, "/search?identifier=2A20&identifier_type=INVALID&date=2023-05-21", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Web handler surfaces store errors in the template, not as HTTP errors
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (error in template body), got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "identifier type") {
		t.Errorf("expected error text about identifier type in response body, got: %q", body)
	}
}

func TestGetStatusPartial_RendersHTML(t *testing.T) {
	db := setupTestDB(t)
	seedSchedule(t, db, "2A20", "C00206")
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "test")},
		Templates: minimalTemplates(t),
	}
	router := buildWebRouter(wh)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "STATUS:1") {
		t.Errorf("expected STATUS:1 in output, got: %q", rec.Body.String())
	}
}
