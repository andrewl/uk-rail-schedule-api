package api

import (
	"html/template"
	"log/slog"
	"net/http"
	"uk-rail-schedule-api/internal/schedule"
	"uk-rail-schedule-api/internal/store"
)

// WebHandler renders htmx-compatible HTML responses using the provided template set.
type WebHandler struct {
	Handler
	Templates *template.Template
}

// indexData is passed to the index.html template for the home page and for
// direct (non-HTMX) permalink loads of /search.
type indexData struct {
	store.APIStatus
	// Form pre-fill values
	Headcode   string
	Tiploc     string
	TrainUID   string
	Date       string
	TOC        string
	Location   string
	HidePassed bool
	// Populated on permalink loads
	Searched  bool
	Schedules []schedule.Schedule
	Error     string
}

func (h *WebHandler) GetIndex(w http.ResponseWriter, r *http.Request) {
	status, err := h.Store.GetStatus()
	if err != nil {
		slog.Error("Failed to get status for index page", "error", err)
	}
	data := indexData{APIStatus: status}
	if err := h.Templates.ExecuteTemplate(w, "index.html", data); err != nil {
		slog.Error("Failed to render index template", "error", err)
		http.Error(w, "template error", 500)
	}
}

func (h *WebHandler) Search(w http.ResponseWriter, r *http.Request) {
	headcode := r.FormValue("headcode")
	tiploc := r.FormValue("tiploc")
	trainUID := r.FormValue("trainuid")
	identifierType, identifier := resolveIdentifier(headcode, tiploc, trainUID)
	date := r.FormValue("date")
	toc := r.FormValue("toc")
	location := r.FormValue("location")
	hidePassedTrains := r.FormValue("hide_passed") == "true"

	// htmx requests get just the results partial; direct browser navigation
	// gets the full page so the permalink is usable.
	isHtmx := r.Header.Get("HX-Request") == "true"

	renderPartialError := func(msg string) {
		data := map[string]interface{}{
			"Identifier": identifier,
			"Date":       date,
			"Schedules":  nil,
			"Error":      msg,
		}
		if err := h.Templates.ExecuteTemplate(w, "partials/schedules.html", data); err != nil {
			slog.Error("Failed to render schedules partial", "error", err)
			http.Error(w, "template error", 500)
		}
	}

	renderFullPage := func(schedules []schedule.Schedule, errMsg string) {
		status, _ := h.Store.GetStatus()
		data := indexData{
			APIStatus:  status,
			Headcode:   headcode,
			Tiploc:     tiploc,
			TrainUID:   trainUID,
			Date:       date,
			TOC:        toc,
			Location:   location,
			HidePassed: hidePassedTrains,
			Searched:   true,
			Schedules:  schedules,
			Error:      errMsg,
		}
		if err := h.Templates.ExecuteTemplate(w, "index.html", data); err != nil {
			slog.Error("Failed to render index template", "error", err)
			http.Error(w, "template error", 500)
		}
	}

	if date == "" {
		if isHtmx {
			renderPartialError("Please enter a date.")
		} else {
			renderFullPage(nil, "Please enter a date.")
		}
		return
	}
	if headcode == "" && tiploc == "" && trainUID == "" && toc == "" && location == "" {
		msg := "Please enter at least one of: headcode, TIPLOC, operator, or location."
		if isHtmx {
			renderPartialError(msg)
		} else {
			renderFullPage(nil, msg)
		}
		return
	}

	tocFilter := toc
	if tocFilter == "" {
		tocFilter = "any"
	}
	locationFilter := location
	if locationFilter == "" {
		locationFilter = "any"
	}

	schedules, err := h.Store.GetSchedules(identifierType, identifier, date, tocFilter, locationFilter, hidePassedTrains)

	if isHtmx {
		data := map[string]interface{}{
			"Identifier": identifier,
			"Date":       date,
			"Schedules":  schedules,
			"Error":      "",
		}
		if err != nil {
			data["Error"] = err.Error()
		}
		if err := h.Templates.ExecuteTemplate(w, "partials/schedules.html", data); err != nil {
			slog.Error("Failed to render schedules partial", "error", err)
			http.Error(w, "template error", 500)
		}
		return
	}

	// Non-HTMX requests (direct browser navigation / shared permalink) get the
	// full page so the form is pre-filled and results are visible immediately.
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	renderFullPage(schedules, errMsg)
}

func (h *WebHandler) GetStatusPartial(w http.ResponseWriter, r *http.Request) {
	status, err := h.Store.GetStatus()
	if err != nil {
		slog.Error("Failed to get status", "error", err)
		http.Error(w, "failed to get status", 500)
		return
	}
	if err := h.Templates.ExecuteTemplate(w, "partials/status.html", status); err != nil {
		slog.Error("Failed to render status partial", "error", err)
		http.Error(w, "template error", 500)
	}
}
