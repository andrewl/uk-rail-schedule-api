package api

import (
	"context"
	"net/http"
	"time"
	"uk-rail-schedule-api/internal/schedule"
	"uk-rail-schedule-api/internal/store"
	internalsync "uk-rail-schedule-api/internal/sync"

	"github.com/go-chi/render"
)

// ScheduleAPIResponse is the JSON envelope returned by the schedules endpoint.
type ScheduleAPIResponse struct {
	Headcode  string              `json:"headcode,omitempty"`
	Tiploc    string              `json:"tiploc,omitempty"`
	TrainUID  string              `json:"trainuid,omitempty"`
	Date      string              `json:"date"`
	Schedules []schedule.Schedule `json:"schedules"`
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
		headcode := r.URL.Query().Get("headcode")
		tiploc := r.URL.Query().Get("tiploc")
		trainUID := r.URL.Query().Get("trainuid")

		date := time.Now().Format("2006-01-02")
		if r.URL.Query().Has("date") {
			date = r.URL.Query().Get("date")
		}

		toc := "any"
		if r.URL.Query().Has("toc") {
			toc = r.URL.Query().Get("toc")
		}

		schedules, err := h.Store.GetSchedules(headcode, date, toc, tiploc, r.URL.Query().Get("hide_passed") == "true")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if len(schedules) == 0 {
			http.Error(w, http.StatusText(404), 404)
			return
		}

		resp := ScheduleAPIResponse{
			Headcode:  headcode,
			Tiploc:    tiploc,
			TrainUID:  trainUID,
			Date:      date,
			Schedules: schedules,
		}
		ctx := context.WithValue(r.Context(), "schedules", resp)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolveIdentifier maps the named identifier query parameters to the internal
// identifierType/identifier pair used by the store. Precedence: headcode →
// tiploc → trainuid. Returns ("headcode", "") if none are set.
func resolveIdentifier(headcode, tiploc, trainuid string) (identifierType, identifier string) {
	switch {
	case headcode != "":
		return "headcode", headcode
	case tiploc != "":
		return "tiploc", tiploc
	case trainuid != "":
		return "trainuid", trainuid
	default:
		return "headcode", ""
	}
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
