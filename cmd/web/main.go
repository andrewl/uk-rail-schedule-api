// web is the HTTP server that serves the JSON API and an htmx-powered web UI.
// It reads schedule data from the shared SQLite database maintained by syncd.
package main

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
	"uk-rail-schedule-api/internal/api"
	"uk-rail-schedule-api/internal/config"
	"uk-rail-schedule-api/internal/db"
	"uk-rail-schedule-api/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/joho/godotenv"
	slogchi "github.com/samber/slog-chi"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// version is set at build time via -ldflags.
var version string

func main() {
	_ = godotenv.Load()

	logger := setupLogger()
	slog.SetDefault(logger)

	database, err := db.Open(config.GetDatabaseFilename())
	if err != nil {
		slog.Error("Failed to open database", "error", err)
		os.Exit(1)
	}

	s := store.New(database, version)

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"now":     func() string { return time.Now().Format("2006-01-02") },
		"daysRun": formatDaysRun,
	}).ParseFS(templateFS,
		"templates/*.html",
		"templates/partials/*.html",
	)
	if err != nil {
		slog.Error("Failed to parse templates", "error", err)
		os.Exit(1)
	}

	h := &api.Handler{
		Store:            s,
		ScheduleFeedFile: config.GetScheduleFeedFilename(),
		DataDir:          config.GetDataDir(),
	}
	wh := &api.WebHandler{Handler: *h, Templates: tmpl}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(slogchi.New(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Static assets
	r.Handle("/static/*", http.FileServer(http.FS(staticFS)))

	// htmx web UI
	r.Get("/", wh.GetIndex)
	r.Post("/search", wh.Search)
	r.Get("/status/partial", wh.GetStatusPartial)

	// JSON API
	r.Route("/schedules/{identifierType}/{identifier}", func(r chi.Router) {
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Use(h.SchedulesCtx)
		r.Get("/", h.GetSchedules)
	})
	r.Route("/status", func(r chi.Router) {
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Use(h.StatusCtx)
		r.Get("/", h.GetStatus)
	})
	r.Route("/refresh", func(r chi.Router) {
		r.Use(render.SetContentType(render.ContentTypeJSON))
		r.Get("/", h.RunRefresh)
	})

	addr := config.GetHTTPListenAddress()
	slog.Info("Serving requests", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("Failure to serve requests", "error", err)
		os.Exit(1)
	}
}

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

// formatDaysRun converts a CIF schedule_days_runs string (7 chars, Mon–Sun)
// into a human-readable description such as "Mon–Fri", "Daily", or "Mon, Wed, Fri".
func formatDaysRun(days string) string {
	if len(days) != 7 {
		return days
	}

	names := [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

	if days == "1111111" {
		return "Daily"
	}
	if days == "1111100" {
		return "Mon–Fri"
	}
	if days == "0000011" {
		return "Sat & Sun"
	}
	if days == "1111110" {
		return "Mon–Sat"
	}

	var active []string
	for i, ch := range days {
		if ch == '1' {
			active = append(active, names[i])
		}
	}
	if len(active) == 0 {
		return "No days"
	}
	// Check for a contiguous range worth abbreviating (3+ days)
	if len(active) >= 3 {
		first := strings.Index(days, "1")
		last := strings.LastIndex(days, "1")
		run := last - first + 1
		if run == len(active) {
			return names[first] + "–" + names[last]
		}
	}
	return strings.Join(active, ", ")
}
