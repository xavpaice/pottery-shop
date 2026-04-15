package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"pottery-shop/internal/middleware"
	"pottery-shop/internal/models"
)

// FiringLogHandler handles CRUD operations for seller firing logs.
type FiringLogHandler struct {
	logs      *models.FiringLogStore
	sessions  *middleware.SessionManager
	templates *template.Template
}

// NewFiringLogHandler creates a new FiringLogHandler.
func NewFiringLogHandler(logs *models.FiringLogStore, sessions *middleware.SessionManager, templates *template.Template) *FiringLogHandler {
	return &FiringLogHandler{
		logs:      logs,
		sessions:  sessions,
		templates: templates,
	}
}

// List handles GET /dashboard/firings — lists the logged-in seller's firing logs.
func (h *FiringLogHandler) List(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)

	logs, err := h.logs.ListBySeller(r.Context(), session.SellerID)
	if err != nil {
		log.Printf("FiringLogHandler.List: error listing logs: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"PageTitle": "My Firing Logs",
		"Logs":      logs,
		"Flash":     session.Flash,
	}
	session.Flash = ""
	h.render(w, "firings_list.html", data)
}

// New handles GET /dashboard/firings/new — renders an empty create form.
func (h *FiringLogHandler) New(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	data := map[string]interface{}{
		"PageTitle": "New Firing Log",
		"Log":       &models.FiringLog{},
		"IsNew":     true,
		"Flash":     session.Flash,
	}
	session.Flash = ""
	h.render(w, "firings_form.html", data)
}

// Create handles POST /dashboard/firings/create — creates a new firing log and redirects to its detail page.
func (h *FiringLogHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session := middleware.GetSession(r)

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		session.Flash = "Title is required"
		http.Redirect(w, r, "/dashboard/firings/new", http.StatusSeeOther)
		return
	}

	var firingDate *string
	if fd := strings.TrimSpace(r.FormValue("firing_date")); fd != "" {
		firingDate = &fd
	}

	fl, err := h.logs.Create(
		r.Context(),
		session.SellerID,
		title,
		strings.TrimSpace(r.FormValue("clay_body")),
		strings.TrimSpace(r.FormValue("glaze_notes")),
		strings.TrimSpace(r.FormValue("outcome")),
		strings.TrimSpace(r.FormValue("notes")),
		firingDate,
	)
	if err != nil {
		log.Printf("FiringLogHandler.Create: error creating log: %v", err)
		http.Error(w, "Error creating firing log", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/dashboard/firings/%d", fl.ID), http.StatusSeeOther)
}

// View handles GET /dashboard/firings/{id} — renders the detail page for a firing log.
func (h *FiringLogHandler) View(w http.ResponseWriter, r *http.Request) {
	id, ok := extractFiringLogID(w, r, "/dashboard/firings/")
	if !ok {
		return
	}

	session := middleware.GetSession(r)

	fl, err := h.logs.GetByID(r.Context(), id, session.SellerID)
	if err != nil {
		log.Printf("FiringLogHandler.View: error fetching log %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if fl == nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data := map[string]interface{}{
		"PageTitle": fl.Title,
		"Log":       fl,
		"Flash":     session.Flash,
	}
	session.Flash = ""
	h.render(w, "firings_detail.html", data)
}

// Edit handles GET /dashboard/firings/{id}/edit — renders the edit form pre-populated with existing data.
func (h *FiringLogHandler) Edit(w http.ResponseWriter, r *http.Request) {
	// Path: /dashboard/firings/{id}/edit
	path := strings.TrimPrefix(r.URL.Path, "/dashboard/firings/")
	path = strings.TrimSuffix(path, "/edit")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	session := middleware.GetSession(r)

	fl, err := h.logs.GetByID(r.Context(), id, session.SellerID)
	if err != nil {
		log.Printf("FiringLogHandler.Edit: error fetching log %d: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if fl == nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data := map[string]interface{}{
		"PageTitle": "Edit Firing Log",
		"Log":       fl,
		"IsNew":     false,
		"Flash":     session.Flash,
	}
	session.Flash = ""
	h.render(w, "firings_form.html", data)
}

// Update handles POST /dashboard/firings/{id}/update — updates log fields and replaces all readings.
func (h *FiringLogHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Path: /dashboard/firings/{id}/update
	path := strings.TrimPrefix(r.URL.Path, "/dashboard/firings/")
	path = strings.TrimSuffix(path, "/update")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	session := middleware.GetSession(r)

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		session.Flash = "Title is required"
		http.Redirect(w, r, fmt.Sprintf("/dashboard/firings/%d/edit", id), http.StatusSeeOther)
		return
	}

	var firingDate *string
	if fd := strings.TrimSpace(r.FormValue("firing_date")); fd != "" {
		firingDate = &fd
	}

	if err := h.logs.Update(
		r.Context(),
		id,
		session.SellerID,
		title,
		strings.TrimSpace(r.FormValue("clay_body")),
		strings.TrimSpace(r.FormValue("glaze_notes")),
		strings.TrimSpace(r.FormValue("outcome")),
		strings.TrimSpace(r.FormValue("notes")),
		firingDate,
	); err != nil {
		log.Printf("FiringLogHandler.Update: error updating log %d: %v", id, err)
		http.Error(w, "Error updating firing log", http.StatusInternalServerError)
		return
	}

	// Parse readings from form — readings[N][field] pattern.
	readings := parseReadings(r)

	if err := h.logs.SaveReadings(r.Context(), id, session.SellerID, readings); err != nil {
		log.Printf("FiringLogHandler.Update: error saving readings for log %d: %v", id, err)
		http.Error(w, "Error saving readings", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/dashboard/firings/%d", id), http.StatusSeeOther)
}

// Delete handles POST /dashboard/firings/{id}/delete — deletes a firing log (CASCADE clears readings).
func (h *FiringLogHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Path: /dashboard/firings/{id}/delete
	path := strings.TrimPrefix(r.URL.Path, "/dashboard/firings/")
	path = strings.TrimSuffix(path, "/delete")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	session := middleware.GetSession(r)

	if err := h.logs.Delete(r.Context(), id, session.SellerID); err != nil {
		log.Printf("FiringLogHandler.Delete: error deleting log %d: %v", id, err)
		http.Error(w, "Error deleting firing log", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dashboard/firings", http.StatusSeeOther)
}

// readingsResponse is the JSON shape returned by ReadingsAPI.
type readingsResponse struct {
	Readings []readingItem `json:"readings"`
}

// readingItem represents a single temperature reading in the JSON API response.
type readingItem struct {
	ElapsedMinutes int     `json:"elapsed_minutes"`
	Temperature    float64 `json:"temperature"`
	GasSetting     string  `json:"gas_setting"`
	FlueSetting    string  `json:"flue_setting"`
}

// ReadingsAPI handles GET /api/firings/{id}/readings — returns JSON readings for a firing log.
// Authentication is enforced via session SellerID (returns JSON 401/403 rather than HTML redirects).
func (h *FiringLogHandler) ReadingsAPI(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session.SellerID == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	// Parse {id} from path: /api/firings/{id}/readings
	path := strings.TrimPrefix(r.URL.Path, "/api/firings/")
	path = strings.TrimSuffix(path, "/readings")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid id"})
		return
	}

	readings, err := h.logs.GetReadingsForAPI(r.Context(), id, session.SellerID)
	if err != nil {
		log.Printf("ReadingsAPI: error fetching readings for log %d: %v", id, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	if readings == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
		return
	}

	items := make([]readingItem, len(readings))
	for i, r := range readings {
		items[i] = readingItem{
			ElapsedMinutes: r.ElapsedMinutes,
			Temperature:    r.Temperature,
			GasSetting:     r.GasSetting,
			FlueSetting:    r.FlueSetting,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(readingsResponse{Readings: items})
}

// parseReadings reads readings[N][field] values from the form until no more rows are found.
// Rows where elapsed_minutes or temperature is blank are skipped.
func parseReadings(r *http.Request) []models.FiringReading {
	var readings []models.FiringReading
	for i := 0; ; i++ {
		elapsedStr := r.FormValue(fmt.Sprintf("readings[%d][elapsed_minutes]", i))
		if elapsedStr == "" {
			break
		}
		tempStr := r.FormValue(fmt.Sprintf("readings[%d][temperature]", i))
		if tempStr == "" {
			continue
		}

		elapsed, err := strconv.Atoi(strings.TrimSpace(elapsedStr))
		if err != nil {
			continue
		}
		temp, err := strconv.ParseFloat(strings.TrimSpace(tempStr), 64)
		if err != nil {
			continue
		}

		readings = append(readings, models.FiringReading{
			ElapsedMinutes: elapsed,
			Temperature:    temp,
			GasSetting:     strings.TrimSpace(r.FormValue(fmt.Sprintf("readings[%d][gas_setting]", i))),
			FlueSetting:    strings.TrimSpace(r.FormValue(fmt.Sprintf("readings[%d][flue_setting]", i))),
			Notes:          strings.TrimSpace(r.FormValue(fmt.Sprintf("readings[%d][notes]", i))),
		})
	}
	return readings
}

// extractFiringLogID parses the numeric ID from a path after a given prefix.
func extractFiringLogID(w http.ResponseWriter, r *http.Request, prefix string) (int64, bool) {
	path := strings.TrimPrefix(r.URL.Path, prefix)
	// Strip any trailing path segments (e.g. /edit, /update, /delete).
	if idx := strings.Index(path, "/"); idx != -1 {
		path = path[:idx]
	}
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return 0, false
	}
	return id, true
}

func (h *FiringLogHandler) render(w http.ResponseWriter, name string, data interface{}) {
	err := h.templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Printf("FiringLogHandler template error (%s): %v", name, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
