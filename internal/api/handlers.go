package api

import (
	"context"
	"net/http"
	"time"
	"uk-rail-schedule-api/internal/schedule"
	"uk-rail-schedule-api/internal/store"
	internalsync "uk-rail-schedule-api/internal/sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// ScheduleAPIResponse is the JSON envelope returned by the schedules endpoint.
type ScheduleAPIResponse struct {
	IdentifierType string              `json:"identifierType"`
	Identifier     string              `json:"identifier"`
	Date           string              `json:"date"`
	Schedules      []schedule.Schedule `json:"schedules"`
}

// ErrResponse is a renderable error for chi/render.
type ErrResponse struct {
	Err            error `json:"-"`
	HTTPStatusCode int   `json:"-"`
	StatusText     string `json:"status"`
	AppCode        int64  `json:"code,omitempty"`
	ErrorText      string `json:"error,omitempty"`
}

func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

var ErrNotFound = &ErrResponse{HTTPStatusCode: 404, StatusText: "Resource not found."}
var ErrUnprocessable = &ErrResponse{HTTPStatusCode: 422, StatusText: "Unprocessable entity."}

// Handler holds dependencies for the JSON API handlers.
type Handler struct {
	Store            *store.Store
	ScheduleFeedFile string
	DataDir          string
}

func (h *Handler) SchedulesCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identifierType := chi.URLParam(r, "identifierType")
		identifier := chi.URLParam(r, "identifier")

		date := time.Now().Format("2006-01-02")
		if r.URL.Query().Has("date") {
			date = r.URL.Query().Get("date")
		}

		toc := "any"
		if r.URL.Query().Has("toc") {
			toc = r.URL.Query().Get("toc")
		}

		location := "any"
		if r.URL.Query().Has("location") {
			location = r.URL.Query().Get("location")
		}

		schedules, err := h.Store.GetSchedules(identifierType, identifier, date, toc, location, r.URL.Query().Get("hidePassed") == "true")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if len(schedules) == 0 {
			http.Error(w, http.StatusText(404), 404)
			return
		}

		resp := ScheduleAPIResponse{
			IdentifierType: identifierType,
			Identifier:     identifier,
			Date:           date,
			Schedules:      schedules,
		}
		ctx := context.WithValue(r.Context(), "schedules", resp)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) StatusCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, err := h.Store.GetStatus()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		ctx := context.WithValue(r.Context(), "status", status)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) GetSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, ok := r.Context().Value("schedules").(ScheduleAPIResponse)
	if !ok {
		render.Render(w, r, ErrUnprocessable)
		return
	}
	render.JSON(w, r, schedules)
}

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status, ok := r.Context().Value("status").(store.APIStatus)
	if !ok {
		render.Render(w, r, ErrUnprocessable)
		return
	}
	render.JSON(w, r, status)
}

func (h *Handler) RunRefresh(w http.ResponseWriter, r *http.Request) {
	if internalsync.IsRefreshingDatabase() {
		w.WriteHeader(409)
		render.JSON(w, r, "Database already being refreshed. Please try again later")
		return
	}
	go internalsync.RefreshSchedules(h.ScheduleFeedFile, h.Store.DB, h.DataDir, false)
	w.WriteHeader(201)
	render.JSON(w, r, "Refreshing")
}
