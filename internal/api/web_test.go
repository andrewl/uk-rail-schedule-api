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

// realisticIndexTemplate builds a template that references the same fields as
// the real index.html, so test failures surface the same errors as production.
func realisticIndexTemplate(t *testing.T) *template.Template {
	t.Helper()
	funcMap := template.FuncMap{
		"now":     func() string { return "2026-01-01" },
		"daysRun": func(s string) string { return s },
	}
	// Mirrors the field references in cmd/web/templates/index.html.
	const indexTmpl = `{{define "index.html"}}` +
		`<input name="headcode" value="{{.Headcode}}">` +
		`<input name="tiploc" value="{{.Tiploc}}">` +
		`<input name="date" value="{{if .Date}}{{.Date}}{{else}}{{now}}{{end}}">` +
		`<input name="toc" value="{{.TOC}}">` +
		`<input name="location" value="{{.Location}}">` +
		`{{if .HidePassed}}<input checked>{{end}}` +
		`{{if .Searched}}{{template "partials/schedules.html" .}}{{end}}` +
		`{{end}}`
	tmpl, err := template.New("index.html").Funcs(funcMap).Parse(indexTmpl)
	if err != nil {
		t.Fatal("failed to parse realistic index template:", err)
	}
	_, err = tmpl.New("partials/schedules.html").Parse(
		`{{define "partials/schedules.html"}}SCHEDULES:{{.Error}}:{{range .Schedules}}ITEM{{end}}{{end}}`,
	)
	if err != nil {
		t.Fatal("failed to parse schedules partial:", err)
	}
	_, err = tmpl.New("partials/status.html").Parse(
		`{{define "partials/status.html"}}STATUS{{end}}`,
	)
	if err != nil {
		t.Fatal("failed to parse status partial:", err)
	}
	return tmpl
}

func TestGetIndex_RealisticTemplate_RendersHTML(t *testing.T) {
	db := setupTestDB(t)
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "v1.0")},
		Templates: realisticIndexTemplate(t),
	}
	router := buildWebRouter(wh)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "template error") {
		t.Errorf("template execution failed: %s", rec.Body.String())
	}
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

func htmxRequest(method, url string) *http.Request {
	req := httptest.NewRequest(method, url, nil)
	req.Header.Set("HX-Request", "true")
	return req
}

func TestSearch_NoResults(t *testing.T) {
	db := setupTestDB(t)
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "test")},
		Templates: minimalTemplates(t),
	}
	router := buildWebRouter(wh)

	req := htmxRequest(http.MethodGet, "/search?headcode=9Z99&date=2023-05-21")
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

	req := htmxRequest(http.MethodGet, "/search?headcode=2A20&date=2023-05-21")
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

func TestSearch_ByHeadcode(t *testing.T) {
	db := setupTestDB(t)
	seedSchedule(t, db, "2A20", "C00206")
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "test")},
		Templates: minimalTemplates(t),
	}
	router := buildWebRouter(wh)

	req := htmxRequest(http.MethodGet, "/search?headcode=2A20&date=2023-05-21")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ITEM") {
		t.Errorf("expected results when searching by headcode, got: %q", rec.Body.String())
	}
}

func TestSearch_DefaultsDateToToday(t *testing.T) {
	db := setupTestDB(t)
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "test")},
		Templates: minimalTemplates(t),
	}
	router := buildWebRouter(wh)

	// date omitted — server returns a validation error (still 200 with error in partial)
	req := htmxRequest(http.MethodGet, "/search?headcode=2A20")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with validation error in body, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "date") {
		t.Errorf("expected validation error mentioning date, got: %q", rec.Body.String())
	}
}

func TestSearch_NoSearchParams_ShowsValidationError(t *testing.T) {
	db := setupTestDB(t)
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "test")},
		Templates: minimalTemplates(t),
	}
	router := buildWebRouter(wh)

	// No headcode, tiploc, toc, or location — only date provided
	req := htmxRequest(http.MethodGet, "/search?date=2023-05-21")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Web handler surfaces validation errors in the template, not as HTTP errors
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (error in template body), got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "headcode") {
		t.Errorf("expected validation error mentioning headcode in response body, got: %q", body)
	}
}

func TestSearch_NonHtmx_RendersFullPage(t *testing.T) {
	db := setupTestDB(t)
	seedSchedule(t, db, "2A20", "C00206")
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "test")},
		Templates: realisticIndexTemplate(t),
	}
	router := buildWebRouter(wh)

	req := httptest.NewRequest(http.MethodGet, "/search?headcode=2A20&date=2023-05-21", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "template error") {
		t.Errorf("template execution failed: %s", body)
	}
	// Form should be pre-filled with the searched identifier.
	if !strings.Contains(body, "2A20") {
		t.Errorf("expected identifier '2A20' in pre-filled form, got: %q", body)
	}
	// Results should be included (Searched=true renders the partial).
	if !strings.Contains(body, "SCHEDULES") {
		t.Errorf("expected schedules partial in full-page response, got: %q", body)
	}
}

func TestSearch_NonHtmx_MissingDate_RendersFullPage(t *testing.T) {
	db := setupTestDB(t)
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "test")},
		Templates: realisticIndexTemplate(t),
	}
	router := buildWebRouter(wh)

	req := httptest.NewRequest(http.MethodGet, "/search?headcode=2A20", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "date") {
		t.Errorf("expected validation error mentioning date, got: %q", rec.Body.String())
	}
}

func TestSearch_NonHtmx_NoSearchParams_RendersFullPage(t *testing.T) {
	db := setupTestDB(t)
	wh := &api.WebHandler{
		Handler:   api.Handler{Store: store.New(db, "test")},
		Templates: realisticIndexTemplate(t),
	}
	router := buildWebRouter(wh)

	// Only date provided, no headcode/tiploc/toc/location — should render full page with error
	req := httptest.NewRequest(http.MethodGet, "/search?date=2023-05-21", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (error rendered in page), got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "headcode") {
		t.Errorf("expected validation error mentioning headcode in response body, got: %q", rec.Body.String())
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
