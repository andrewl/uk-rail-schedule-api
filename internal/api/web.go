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

// indexData is passed to the index.html template for both the home page and
// direct (non-htmx) permalink loads of /search.
type indexData struct {
	store.APIStatus
	// Form pre-fill values
	Identifier string
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
	identifier := r.FormValue("identifier")
	identifierType := r.FormValue("identifier_type")
	if identifierType == "" {
		identifierType = "headcode"
	}
	date := r.FormValue("date")
	toc := r.FormValue("toc")
	location := r.FormValue("location")
	hidePassedTrains := r.FormValue("hide_passed") == "true"

	// htmx requests get just the results partial; direct browser navigation
	// gets the full page so the permalink is usable.
	isHtmx := r.Header.Get("HX-Request") == "true"

	renderPartialError := func(msg string) {
		data := map[string]interface{}{
			"IdentifierType": identifierType,
			"Identifier":     identifier,
			"Date":           date,
			"Schedules":      nil,
			"Error":          msg,
		}
		if err := h.Templates.ExecuteTemplate(w, "partials/schedules.html", data); err != nil {
			slog.Error("Failed to render schedules partial", "error", err)
			http.Error(w, "template error", 500)
		}
	}

	renderFullError := func(msg string) {
		status, _ := h.Store.GetStatus()
		data := indexData{
			APIStatus:   status,
			Identifier:  identifier,
			Date:        date,
			TOC:         toc,
			Location:    location,
			HidePassed:  hidePassedTrains,
			Searched:    true,
			Error: msg,
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
			renderFullError("Please enter a date.")
		}
		return
	}
	if identifier == "" && toc == "" && location == "" {
		msg := "Please enter at least one of: train identifier, operator, or location."
		if isHtmx {
			renderPartialError(msg)
		} else {
			renderFullError(msg)
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
			"IdentifierType": identifierType,
			"Identifier":     identifier,
			"Date":           date,
			"Schedules":      schedules,
			"Error":          "",
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

	// Full-page response for direct permalink navigation.
	status, _ := h.Store.GetStatus()
	data := indexData{
		APIStatus:   status,
		Identifier:  identifier,
		Date:        date,
		TOC:         toc,
		Location:    location,
		HidePassed:  hidePassedTrains,
		Searched:    true,
		Schedules:   schedules,
	}
	if err != nil {
		data.Error = err.Error()
	}
	if err := h.Templates.ExecuteTemplate(w, "index.html", data); err != nil {
		slog.Error("Failed to render index template", "error", err)
		http.Error(w, "template error", 500)
	}
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
