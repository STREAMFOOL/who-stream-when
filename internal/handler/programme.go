package handler

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"who-live-when/internal/auth"
	"who-live-when/internal/domain"
	"who-live-when/internal/service"
)

// ProgrammeService interface for programme management operations
type ProgrammeService interface {
	CreateCustomProgramme(ctx context.Context, userID string, streamerIDs []string) (*domain.CustomProgramme, error)
	GetCustomProgramme(ctx context.Context, userID string) (*domain.CustomProgramme, error)
	UpdateCustomProgramme(ctx context.Context, userID string, streamerIDs []string) error
	DeleteCustomProgramme(ctx context.Context, userID string) error
	AddStreamerToProgramme(ctx context.Context, userID, streamerID string) error
	RemoveStreamerFromProgramme(ctx context.Context, userID, streamerID string) error
	GetProgrammeView(ctx context.Context, userID string, week time.Time) (*service.ProgrammeCalendarView, error)
}

// StreamerService interface for streamer operations
type StreamerService interface {
	GetStreamer(ctx context.Context, id string) (*domain.Streamer, error)
	ListStreamers(ctx context.Context, limit int) ([]*domain.Streamer, error)
}

// ProgrammeHandler handles programme management routes
type ProgrammeHandler struct {
	programmeService ProgrammeService
	streamerService  StreamerService
	sessionManager   *auth.SessionManager
	templates        *template.Template
}

// NewProgrammeHandler creates a new ProgrammeHandler
func NewProgrammeHandler(
	programmeService ProgrammeService,
	streamerService StreamerService,
	sessionManager *auth.SessionManager,
) *ProgrammeHandler {
	return &ProgrammeHandler{
		programmeService: programmeService,
		streamerService:  streamerService,
		sessionManager:   sessionManager,
		templates:        LoadTemplates(),
	}
}

// HandleProgrammeManagement displays the programme management interface
// GET /programme
func (h *ProgrammeHandler) HandleProgrammeManagement(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if user is authenticated
	userID, err := h.sessionManager.GetSession(r)
	isAuthenticated := err == nil && userID != ""
	isGuest := !isAuthenticated

	var customProgramme *domain.CustomProgramme
	var programmeStreamers []*domain.Streamer

	if isAuthenticated {
		// Try to get custom programme for authenticated user
		customProgramme, err = h.programmeService.GetCustomProgramme(ctx, userID)
		if err != nil && err != service.ErrProgrammeNotFound {
			log.Printf("Error getting custom programme: %v", err)
		}
	} else {
		// Try to get guest programme from session
		guestProgramme, err := h.sessionManager.GetGuestProgramme(r)
		if err == nil && guestProgramme != nil {
			customProgramme = &domain.CustomProgramme{
				StreamerIDs: guestProgramme.StreamerIDs,
			}
		}
	}

	// Load streamers for the custom programme
	if customProgramme != nil && len(customProgramme.StreamerIDs) > 0 {
		for _, streamerID := range customProgramme.StreamerIDs {
			streamer, err := h.streamerService.GetStreamer(ctx, streamerID)
			if err != nil {
				log.Printf("Error getting streamer %s: %v", streamerID, err)
				continue
			}
			programmeStreamers = append(programmeStreamers, streamer)
		}
	}

	// Get all available streamers for selection
	allStreamers, err := h.streamerService.ListStreamers(ctx, 100)
	if err != nil {
		log.Printf("Error listing streamers: %v", err)
		allStreamers = []*domain.Streamer{}
	}

	hasCustomProgramme := customProgramme != nil && len(customProgramme.StreamerIDs) > 0

	data := map[string]any{
		"IsAuthenticated":    isAuthenticated,
		"IsGuest":            isGuest,
		"HasCustomProgramme": hasCustomProgramme,
		"ProgrammeStreamers": programmeStreamers,
		"AllStreamers":       allStreamers,
		"CustomProgramme":    customProgramme,
	}

	// Try to render template, fallback to simple HTML if template not found
	if err := h.templates.ExecuteTemplate(w, "programme.html", data); err != nil {
		h.renderSimpleProgrammeManagement(w, isAuthenticated, isGuest, hasCustomProgramme, programmeStreamers, allStreamers)
	}
}

// HandleCreateProgramme creates a new custom programme
// POST /programme/create
func (h *ProgrammeHandler) HandleCreateProgramme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Get streamer IDs from form
	streamerIDsStr := r.FormValue("streamer_ids")
	var streamerIDs []string
	if streamerIDsStr != "" {
		streamerIDs = strings.Split(streamerIDsStr, ",")
		// Trim whitespace
		for i, id := range streamerIDs {
			streamerIDs[i] = strings.TrimSpace(id)
		}
	}

	// Check if user is authenticated
	userID, err := h.sessionManager.GetSession(r)
	isAuthenticated := err == nil && userID != ""

	if isAuthenticated {
		// Create database-backed programme for authenticated user
		_, err := h.programmeService.CreateCustomProgramme(ctx, userID, streamerIDs)
		if err != nil {
			log.Printf("Error creating custom programme: %v", err)
			http.Error(w, "Failed to create programme", http.StatusInternalServerError)
			return
		}
	} else {
		// Create session-based programme for guest user
		guestProgramme := &auth.CustomProgrammeData{
			StreamerIDs: streamerIDs,
		}
		if err := h.sessionManager.SetGuestProgramme(w, r, guestProgramme); err != nil {
			log.Printf("Error setting guest programme: %v", err)
			http.Error(w, "Failed to create programme", http.StatusInternalServerError)
			return
		}
	}

	// Redirect back to programme management page
	http.Redirect(w, r, "/programme", http.StatusSeeOther)
}

// HandleUpdateProgramme updates an existing custom programme
// POST /programme/update
func (h *ProgrammeHandler) HandleUpdateProgramme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Get streamer IDs from form
	streamerIDsStr := r.FormValue("streamer_ids")
	var streamerIDs []string
	if streamerIDsStr != "" {
		streamerIDs = strings.Split(streamerIDsStr, ",")
		// Trim whitespace
		for i, id := range streamerIDs {
			streamerIDs[i] = strings.TrimSpace(id)
		}
	}

	// Check if user is authenticated
	userID, err := h.sessionManager.GetSession(r)
	isAuthenticated := err == nil && userID != ""

	if isAuthenticated {
		// Update database-backed programme
		err := h.programmeService.UpdateCustomProgramme(ctx, userID, streamerIDs)
		if err != nil {
			log.Printf("Error updating custom programme: %v", err)
			http.Error(w, "Failed to update programme", http.StatusInternalServerError)
			return
		}
	} else {
		// Update session-based programme
		guestProgramme := &auth.CustomProgrammeData{
			StreamerIDs: streamerIDs,
		}
		if err := h.sessionManager.SetGuestProgramme(w, r, guestProgramme); err != nil {
			log.Printf("Error updating guest programme: %v", err)
			http.Error(w, "Failed to update programme", http.StatusInternalServerError)
			return
		}
	}

	// Redirect back to programme management page
	http.Redirect(w, r, "/programme", http.StatusSeeOther)
}

// HandleDeleteProgramme deletes a custom programme (reverts to global)
// POST /programme/delete
func (h *ProgrammeHandler) HandleDeleteProgramme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Check if user is authenticated
	userID, err := h.sessionManager.GetSession(r)
	isAuthenticated := err == nil && userID != ""

	if isAuthenticated {
		// Delete database-backed programme
		err := h.programmeService.DeleteCustomProgramme(ctx, userID)
		if err != nil {
			log.Printf("Error deleting custom programme: %v", err)
			http.Error(w, "Failed to delete programme", http.StatusInternalServerError)
			return
		}
	} else {
		// Clear session-based programme
		h.sessionManager.ClearGuestData(w)
	}

	// Redirect back to programme management page
	http.Redirect(w, r, "/programme", http.StatusSeeOther)
}

// HandleAddStreamer adds a streamer to the custom programme
// POST /programme/add/{id}
func (h *ProgrammeHandler) HandleAddStreamer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Extract streamer ID from URL path
	streamerID := r.PathValue("id")
	if streamerID == "" {
		// Fallback: try to extract from URL path manually
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/programme/add/"), "/")
		if len(parts) > 0 {
			streamerID = parts[0]
		}
	}

	if streamerID == "" {
		http.Error(w, "Streamer ID is required", http.StatusBadRequest)
		return
	}

	// Check if user is authenticated
	userID, err := h.sessionManager.GetSession(r)
	isAuthenticated := err == nil && userID != ""

	if isAuthenticated {
		// Add to database-backed programme
		err := h.programmeService.AddStreamerToProgramme(ctx, userID, streamerID)
		if err != nil {
			log.Printf("Error adding streamer to programme: %v", err)
			http.Error(w, "Failed to add streamer", http.StatusInternalServerError)
			return
		}
	} else {
		// Add to session-based programme
		guestProgramme, err := h.sessionManager.GetGuestProgramme(r)
		if err != nil || guestProgramme == nil {
			guestProgramme = &auth.CustomProgrammeData{
				StreamerIDs: []string{},
			}
		}
		guestProgramme.StreamerIDs = append(guestProgramme.StreamerIDs, streamerID)
		if err := h.sessionManager.SetGuestProgramme(w, r, guestProgramme); err != nil {
			log.Printf("Error updating guest programme: %v", err)
			http.Error(w, "Failed to add streamer", http.StatusInternalServerError)
			return
		}
	}

	// Redirect back to referer or programme page
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/programme"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// HandleRemoveStreamer removes a streamer from the custom programme
// POST /programme/remove/{id}
func (h *ProgrammeHandler) HandleRemoveStreamer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Extract streamer ID from URL path
	streamerID := r.PathValue("id")
	if streamerID == "" {
		// Fallback: try to extract from URL path manually
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/programme/remove/"), "/")
		if len(parts) > 0 {
			streamerID = parts[0]
		}
	}

	if streamerID == "" {
		http.Error(w, "Streamer ID is required", http.StatusBadRequest)
		return
	}

	// Check if user is authenticated
	userID, err := h.sessionManager.GetSession(r)
	isAuthenticated := err == nil && userID != ""

	if isAuthenticated {
		// Remove from database-backed programme
		err := h.programmeService.RemoveStreamerFromProgramme(ctx, userID, streamerID)
		if err != nil {
			log.Printf("Error removing streamer from programme: %v", err)
			http.Error(w, "Failed to remove streamer", http.StatusInternalServerError)
			return
		}
	} else {
		// Remove from session-based programme
		guestProgramme, err := h.sessionManager.GetGuestProgramme(r)
		if err != nil || guestProgramme == nil {
			http.Error(w, "No programme found", http.StatusNotFound)
			return
		}

		var newIDs []string
		for _, id := range guestProgramme.StreamerIDs {
			if id != streamerID {
				newIDs = append(newIDs, id)
			}
		}
		guestProgramme.StreamerIDs = newIDs

		if err := h.sessionManager.SetGuestProgramme(w, r, guestProgramme); err != nil {
			log.Printf("Error updating guest programme: %v", err)
			http.Error(w, "Failed to remove streamer", http.StatusInternalServerError)
			return
		}
	}

	// Redirect back to referer or programme page
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/programme"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// renderSimpleProgrammeManagement renders a simple HTML programme management page
func (h *ProgrammeHandler) renderSimpleProgrammeManagement(w http.ResponseWriter, isAuthenticated, isGuest, hasCustomProgramme bool, programmeStreamers, allStreamers []*domain.Streamer) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>Programme Management - Who Live When</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 20px; }
		.header { background-color: #e7f3ff; padding: 10px; margin-bottom: 20px; }
		.notice { background-color: #fff3cd; padding: 10px; margin: 10px 0; border-radius: 5px; }
		.streamer-list { list-style: none; padding: 0; }
		.streamer-item { border: 1px solid #ccc; padding: 10px; margin: 5px 0; display: flex; justify-content: space-between; align-items: center; }
		.btn { padding: 5px 10px; margin: 0 5px; cursor: pointer; }
		.btn-danger { background-color: #dc3545; color: white; border: none; }
		.btn-primary { background-color: #007bff; color: white; border: none; }
	</style>
</head>
<body>
	<div class="header">
		<h1>Programme Management</h1>
		<p><a href="/">← Back to Home</a>`)

	if isAuthenticated {
		fmt.Fprintf(w, ` | <a href="/dashboard">Dashboard</a>`)
	}

	fmt.Fprintf(w, `</p>
	</div>
`)

	if isGuest {
		fmt.Fprintf(w, `
	<div class="notice">
		<strong>Guest User Notice:</strong> You are browsing as a guest. Your programme is stored in your browser session and will be lost when you close your browser. <a href="/login">Login</a> to save your programme permanently.
	</div>
`)
	}

	if hasCustomProgramme {
		fmt.Fprintf(w, `
	<h2>Your Custom Programme</h2>
	<p>You are currently using a custom programme. The calendar will show only these streamers.</p>
	<ul class="streamer-list">
`)
		for _, streamer := range programmeStreamers {
			fmt.Fprintf(w, `
		<li class="streamer-item">
			<span>%s</span>
			<form action="/programme/remove/%s" method="POST" style="display: inline;">
				<button type="submit" class="btn btn-danger">Remove</button>
			</form>
		</li>
`, streamer.Name, streamer.ID)
		}
		fmt.Fprintf(w, `
	</ul>
	<form action="/programme/delete" method="POST" style="margin-top: 20px;">
		<button type="submit" class="btn btn-danger">Clear Programme (Revert to Global)</button>
	</form>
`)
	} else {
		fmt.Fprintf(w, `
	<h2>Global Programme</h2>
	<p>You are currently using the global programme, which shows the most popular streamers. Create a custom programme to personalize your calendar.</p>
`)
	}

	fmt.Fprintf(w, `
	<h2>Add Streamers to Programme</h2>
	<ul class="streamer-list">
`)
	for _, streamer := range allStreamers {
		// Check if already in programme
		inProgramme := false
		for _, ps := range programmeStreamers {
			if ps.ID == streamer.ID {
				inProgramme = true
				break
			}
		}

		if !inProgramme {
			fmt.Fprintf(w, `
		<li class="streamer-item">
			<span>%s</span>
			<form action="/programme/add/%s" method="POST" style="display: inline;">
				<button type="submit" class="btn btn-primary">Add to Programme</button>
			</form>
		</li>
`, streamer.Name, streamer.ID)
		}
	}
	fmt.Fprintf(w, `
	</ul>
</body>
</html>`)
}
