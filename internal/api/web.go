package api

import (
	"html/template"
	"log/slog"
	"net/http"
)

// WebHandler renders htmx-compatible HTML responses using the provided template set.
type WebHandler struct {
	Handler
	Templates *template.Template
}

func (h *WebHandler) GetIndex(w http.ResponseWriter, r *http.Request) {
	status, err := h.Store.GetStatus()
	if err != nil {
		slog.Error("Failed to get status for index page", "error", err)
	}
	if err := h.Templates.ExecuteTemplate(w, "index.html", status); err != nil {
		slog.Error("Failed to render index template", "error", err)
		http.Error(w, "template error", 500)
	}
}

type searchResult struct {
	IdentifierType string
	Identifier     string
	Date           string
	Schedules      []interface{}
	Error          string
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

	renderError := func(msg string) {
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

	if date == "" {
		renderError("Please enter a date.")
		return
	}
	if identifier == "" && toc == "" && location == "" {
		renderError("Please enter at least one of: train identifier, operator, or location.")
		return
	}

	if toc == "" {
		toc = "any"
	}
	if location == "" {
		location = "any"
	}

	hidePassedTrains := r.FormValue("hide_passed") == "true"

	schedules, err := h.Store.GetSchedules(identifierType, identifier, date, toc, location, hidePassedTrains)

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
