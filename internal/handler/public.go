package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"who-live-when/internal/auth"
	"who-live-when/internal/domain"
	"who-live-when/internal/logger"
	"who-live-when/internal/service"
)

// PublicHandler handles public routes
type PublicHandler struct {
	tvProgrammeService domain.TVProgrammeService
	streamerService    domain.StreamerService
	liveStatusService  domain.LiveStatusService
	heatmapService     domain.HeatmapService
	userService        domain.UserService
	searchService      *service.SearchService
	programmeService   *service.ProgrammeService
	kickAdapter        domain.PlatformAdapter
	sessionManager     *auth.SessionManager
	templates          *template.Template
	logger             *logger.Logger
}

// NewPublicHandler creates a new PublicHandler
func NewPublicHandler(
	tvProgrammeService domain.TVProgrammeService,
	streamerService domain.StreamerService,
	liveStatusService domain.LiveStatusService,
	heatmapService domain.HeatmapService,
	userService domain.UserService,
	searchService *service.SearchService,
	programmeService *service.ProgrammeService,
	kickAdapter domain.PlatformAdapter,
	sessionManager *auth.SessionManager,
) *PublicHandler {
	return &PublicHandler{
		tvProgrammeService: tvProgrammeService,
		streamerService:    streamerService,
		liveStatusService:  liveStatusService,
		heatmapService:     heatmapService,
		userService:        userService,
		searchService:      searchService,
		programmeService:   programmeService,
		kickAdapter:        kickAdapter,
		sessionManager:     sessionManager,
		templates:          LoadTemplates(),
		logger:             logger.Default(),
	}
}

// HandleHome displays the home page with custom or global programme
// GET /
func (h *PublicHandler) HandleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if user is authenticated
	userID, _ := h.sessionManager.GetSession(r)
	isAuthenticated := userID != ""

	// Try to get custom programme first, fall back to global programme
	var calendarView *service.ProgrammeCalendarView
	var err error
	var programmeType string

	if isAuthenticated {
		// Try to get custom programme for authenticated user
		customProgramme, err := h.programmeService.GetCustomProgramme(ctx, userID)
		if err == nil && customProgramme != nil && len(customProgramme.StreamerIDs) > 0 {
			// User has a custom programme
			calendarView, err = h.programmeService.GenerateCalendarFromProgramme(ctx, customProgramme, time.Now())
			if err == nil {
				programmeType = "custom"
			}
		}
	} else {
		// Check for guest programme in session
		guestProgramme, err := h.sessionManager.GetGuestProgramme(r)
		if err == nil && guestProgramme != nil && len(guestProgramme.StreamerIDs) > 0 {
			// Guest has a custom programme
			customProgramme := &domain.CustomProgramme{
				StreamerIDs: guestProgramme.StreamerIDs,
			}
			calendarView, err = h.programmeService.GenerateCalendarFromProgramme(ctx, customProgramme, time.Now())
			if err == nil {
				programmeType = "custom"
			}
		}
	}

	// Fall back to global programme if no custom programme or error
	if calendarView == nil {
		calendarView, err = h.programmeService.GenerateGlobalProgramme(ctx, time.Now(), 10)
		if err != nil {
			h.logger.Error("Failed to generate global programme", map[string]interface{}{
				"error": err.Error(),
			})
			h.renderError(w, "Unable to load home page. Please try again later.", http.StatusInternalServerError)
			return
		}
		programmeType = "global"
	}

	// Get live status for all streamers in the calendar view
	liveStatuses := make(map[string]*domain.LiveStatus)
	for _, streamer := range calendarView.Streamers {
		status, err := h.liveStatusService.GetLiveStatus(ctx, streamer.ID)
		if err != nil {
			h.logger.Warn("Failed to get live status for streamer", map[string]interface{}{
				"streamer_id": streamer.ID,
				"error":       err.Error(),
			})
			continue
		}
		liveStatuses[streamer.ID] = status
	}

	// Build view count map for compatibility with template
	viewCount := make(map[string]int)
	for _, streamer := range calendarView.Streamers {
		// For now, we'll use 0 as placeholder - could be enhanced to show follower count
		viewCount[streamer.ID] = 0
	}

	// Create WeekView for template compatibility
	weekView := &domain.WeekView{
		Week:      calendarView.Week,
		Streamers: calendarView.Streamers,
		Entries:   calendarView.Entries,
		ViewCount: viewCount,
	}

	data := map[string]interface{}{
		"WeekView":        weekView,
		"LiveStatuses":    liveStatuses,
		"IsAuthenticated": isAuthenticated,
		"ProgrammeType":   programmeType,
		"IsCustom":        programmeType == "custom",
	}

	// Try to render template, fallback to simple HTML if template not found
	if err := h.templates.ExecuteTemplate(w, "home.html", data); err != nil {
		// Fallback to simple HTML response
		h.renderSimpleHome(w, weekView, liveStatuses, isAuthenticated, programmeType == "custom")
	}
}

// renderSimpleHome renders a simple HTML home page when templates are not available
func (h *PublicHandler) renderSimpleHome(w http.ResponseWriter, weekView *domain.WeekView, liveStatuses map[string]*domain.LiveStatus, isAuthenticated bool, isCustom bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	programmeTitle := "Most Viewed Streamers"
	if isCustom {
		programmeTitle = "Your Custom Programme"
	}

	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>Who Live When - Home</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 20px; }
		.streamer { border: 1px solid #ccc; padding: 10px; margin: 10px 0; }
		.live { background-color: #d4edda; }
		.offline { background-color: #f8d7da; }
		.auth-status { background-color: #e7f3ff; padding: 10px; margin-bottom: 20px; }
	</style>
</head>
<body>
	<h1>Who Live When</h1>
	<div class="auth-status">
		%s
	</div>
	<h2>%s</h2>
`, func() string {
		if isAuthenticated {
			return `<p>You are logged in. <a href="/dashboard">Go to Dashboard</a> | <a href="/logout">Logout</a></p>`
		}
		return `<p>You are browsing as a guest. <a href="/login">Login with Google</a> to follow streamers.</p>`
	}(), programmeTitle)

	for _, streamer := range weekView.Streamers {
		status := liveStatuses[streamer.ID]
		liveClass := "offline"
		liveText := "Offline"
		streamLink := ""

		if status != nil && status.IsLive {
			liveClass = "live"
			liveText = fmt.Sprintf("Live on %s", status.Platform)
			if status.StreamURL != "" {
				streamLink = fmt.Sprintf(` - <a href="%s" target="_blank">Watch Stream</a>`, status.StreamURL)
			}
		}

		viewCount := weekView.ViewCount[streamer.ID]
		fmt.Fprintf(w, `
	<div class="streamer %s">
		<h3><a href="/streamer/%s">%s</a></h3>
		<p>Status: %s%s</p>
		<p>Followers: %d</p>
		<p>Platforms: %v</p>
	</div>
`, liveClass, streamer.ID, streamer.Name, liveText, streamLink, viewCount, streamer.Platforms)
	}

	fmt.Fprintf(w, `
</body>
</html>`)
}

// HandleStreamerDetail displays the streamer detail page with live status and heatmap
// GET /streamer/:id
func (h *PublicHandler) HandleStreamerDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	streamerID := r.PathValue("id")

	if streamerID == "" {
		http.Error(w, "Streamer ID is required", http.StatusBadRequest)
		return
	}

	// Get streamer information
	streamer, err := h.streamerService.GetStreamer(ctx, streamerID)
	if err != nil {
		// Check if it's a not found error (from service package)
		if streamer == nil {
			h.logger.Warn("Streamer not found", map[string]interface{}{
				"streamer_id": streamerID,
			})
			h.renderError(w, "Streamer not found", http.StatusNotFound)
			return
		}
		h.logger.Error("Failed to get streamer", map[string]interface{}{
			"streamer_id": streamerID,
			"error":       err.Error(),
		})
		h.renderError(w, "Unable to load streamer information. Please try again later.", http.StatusInternalServerError)
		return
	}

	// Get live status
	liveStatus, err := h.liveStatusService.GetLiveStatus(ctx, streamerID)
	if err != nil {
		h.logger.Warn("Failed to get live status", map[string]interface{}{
			"streamer_id": streamerID,
			"error":       err.Error(),
		})
		// Continue without live status
	}

	// Get heatmap
	heatmap, err := h.heatmapService.GenerateHeatmap(ctx, streamerID)
	if err != nil {
		h.logger.Warn("Failed to generate heatmap", map[string]interface{}{
			"streamer_id": streamerID,
			"error":       err.Error(),
		})
		// Continue without heatmap
	}

	// Check if user is authenticated
	userID, _ := h.sessionManager.GetSession(r)
	isAuthenticated := userID != ""

	// Check if user is following this streamer
	isFollowing := false
	if isAuthenticated {
		follows, err := h.userService.GetUserFollows(ctx, userID)
		if err == nil {
			for _, f := range follows {
				if f.ID == streamerID {
					isFollowing = true
					break
				}
			}
		}
	}

	data := map[string]interface{}{
		"Streamer":        streamer,
		"LiveStatus":      liveStatus,
		"Heatmap":         heatmap,
		"IsAuthenticated": isAuthenticated,
		"IsFollowing":     isFollowing,
	}

	// Try to render template, fallback to simple HTML if template not found
	if err := h.templates.ExecuteTemplate(w, "streamer.html", data); err != nil {
		// Fallback to simple HTML response
		h.renderSimpleStreamerDetail(w, streamer, liveStatus, heatmap, isAuthenticated, isFollowing)
	}
}

// renderSimpleStreamerDetail renders a simple HTML streamer detail page
func (h *PublicHandler) renderSimpleStreamerDetail(w http.ResponseWriter, streamer *domain.Streamer, liveStatus *domain.LiveStatus, heatmap *domain.Heatmap, isAuthenticated, isFollowing bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>%s - Who Live When</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 20px; }
		.live { background-color: #d4edda; padding: 10px; margin: 10px 0; }
		.offline { background-color: #f8d7da; padding: 10px; margin: 10px 0; }
		.heatmap { margin: 20px 0; }
		.heatmap-row { display: flex; }
		.heatmap-cell { width: 30px; height: 30px; margin: 2px; text-align: center; line-height: 30px; }
	</style>
</head>
<body>
	<h1>%s</h1>
	<p><a href="/">← Back to Home</a></p>
`, streamer.Name, streamer.Name)

	// Live status
	if liveStatus != nil {
		if liveStatus.IsLive {
			fmt.Fprintf(w, `
	<div class="live">
		<h2>🔴 Live on %s</h2>
		<p>%s</p>
		<p><a href="%s" target="_blank">Watch Stream</a></p>
		<p>Viewers: %d</p>
	</div>
`, liveStatus.Platform, liveStatus.Title, liveStatus.StreamURL, liveStatus.ViewerCount)
		} else {
			fmt.Fprintf(w, `
	<div class="offline">
		<h2>Offline</h2>
		<p>This streamer is not currently live.</p>
	</div>
`)
		}
	}

	// Platforms
	fmt.Fprintf(w, `
	<h2>Platforms</h2>
	<ul>
`)
	for _, platform := range streamer.Platforms {
		handle := streamer.Handles[platform]
		fmt.Fprintf(w, `		<li>%s: %s</li>
`, platform, handle)
	}
	fmt.Fprintf(w, `	</ul>
`)

	// Heatmap
	if heatmap != nil {
		fmt.Fprintf(w, `
	<h2>Activity Heatmap</h2>
	<p>Based on %d data points</p>
	<div class="heatmap">
		<h3>Hours of Day</h3>
		<div class="heatmap-row">
`, heatmap.DataPoints)
		for hour := 0; hour < 24; hour++ {
			prob := heatmap.Hours[hour]
			intensity := int(prob * 255)
			color := fmt.Sprintf("rgb(%d, %d, %d)", 255-intensity, 255, 255-intensity)
			fmt.Fprintf(w, `			<div class="heatmap-cell" style="background-color: %s;" title="Hour %d: %.2f%%">%d</div>
`, color, hour, prob*100, hour)
		}
		fmt.Fprintf(w, `		</div>
		<h3>Days of Week</h3>
		<div class="heatmap-row">
`)
		days := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
		for day := 0; day < 7; day++ {
			prob := heatmap.DaysOfWeek[day]
			intensity := int(prob * 255)
			color := fmt.Sprintf("rgb(%d, %d, %d)", 255-intensity, 255, 255-intensity)
			fmt.Fprintf(w, `			<div class="heatmap-cell" style="background-color: %s; width: 50px;" title="%s: %.2f%%">%s</div>
`, color, days[day], prob*100, days[day])
		}
		fmt.Fprintf(w, `		</div>
	</div>
`)
	} else {
		fmt.Fprintf(w, `
	<h2>Activity Heatmap</h2>
	<p>Insufficient historical data to generate heatmap.</p>
`)
	}

	// Follow button (only for authenticated users)
	if isAuthenticated {
		if isFollowing {
			fmt.Fprintf(w, `
	<form action="/unfollow/%s" method="POST">
		<button type="submit">Unfollow</button>
	</form>
`, streamer.ID)
		} else {
			fmt.Fprintf(w, `
	<form action="/follow/%s" method="POST">
		<button type="submit">Follow</button>
	</form>
`, streamer.ID)
		}
	} else {
		fmt.Fprintf(w, `
	<p><a href="/login">Login</a> to follow this streamer.</p>
`)
	}

	fmt.Fprintf(w, `
</body>
</html>`)
}

// HandleSearch handles streamer search requests (accessible to all users)
// GET/POST /search
func (h *PublicHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var query string

	if r.Method == http.MethodGet {
		// GET request - show search page with optional query from URL
		query = r.URL.Query().Get("q")
	} else if r.Method == http.MethodPost {
		// POST request - process search form
		if err := r.ParseForm(); err != nil {
			log.Printf("Error parsing form: %v", err)
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}
		query = r.FormValue("query")
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If no query, show empty search page
	if query == "" {
		data := map[string]any{
			"Query":           "",
			"Results":         []*service.SearchResult{},
			"FollowedHandles": make(map[string]bool),
			"IsAuthenticated": false,
		}
		if err := h.templates.ExecuteTemplate(w, "search.html", data); err != nil {
			h.renderSimpleSearch(w, "", nil, nil, false)
		}
		return
	}

	// Perform search across all platforms
	results, err := h.searchService.SearchStreamers(ctx, query)
	if err != nil {
		log.Printf("Error searching streamers: %v", err)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	// Check if user is authenticated
	userID, _ := h.sessionManager.GetSession(r)
	isAuthenticated := userID != ""

	// Get user's followed streamers to check which ones are already followed
	followedHandles := make(map[string]bool)
	if isAuthenticated {
		followedStreamers, err := h.userService.GetUserFollows(ctx, userID)
		if err != nil {
			log.Printf("Error getting user follows: %v", err)
		} else {
			for _, streamer := range followedStreamers {
				for _, handle := range streamer.Handles {
					followedHandles[handle] = true
				}
			}
		}
	}

	data := map[string]any{
		"Query":           query,
		"Results":         results,
		"FollowedHandles": followedHandles,
		"IsAuthenticated": isAuthenticated,
	}

	// Try to render template, fallback to simple HTML if template not found
	if err := h.templates.ExecuteTemplate(w, "search.html", data); err != nil {
		h.renderSimpleSearch(w, query, results, followedHandles, isAuthenticated)
	}
}

// renderSimpleSearch renders a simple HTML search results page
func (h *PublicHandler) renderSimpleSearch(w http.ResponseWriter, query string, results []*service.SearchResult, followedHandles map[string]bool, isAuthenticated bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	backLink := "/"
	if isAuthenticated {
		backLink = "/dashboard"
	}

	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>Search Results - Who Live When</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 20px; }
		.result { border: 1px solid #ccc; padding: 10px; margin: 10px 0; }
		.header { background-color: #e7f3ff; padding: 10px; margin-bottom: 20px; }
	</style>
</head>
<body>
	<div class="header">
		<h1>Search Results</h1>
		<p><a href="%s">← Back</a></p>
	</div>
	<h2>Results for "%s"</h2>
	<form action="/search" method="POST" style="margin-bottom: 20px;">
		<input type="text" name="query" placeholder="Search for streamers..." value="%s" required>
		<button type="submit">Search</button>
	</form>
`, backLink, query, query)

	if len(results) == 0 {
		fmt.Fprintf(w, `<p>No streamers found matching your search.</p>`)
	} else {
		for _, result := range results {
			isFollowed := false
			for _, handle := range result.Handles {
				if followedHandles[handle] {
					isFollowed = true
					break
				}
			}

			fmt.Fprintf(w, `
	<div class="result">
		<h3>%s</h3>
		<p>Platforms: %v</p>
		<p>Handles: %v</p>
`, result.Name, result.Platforms, result.Handles)

			if isFollowed {
				fmt.Fprintf(w, `<p><em>Already following</em></p>`)
			} else if isAuthenticated {
				fmt.Fprintf(w, `<p><em>To follow this streamer, they need to be added to the system first.</em></p>`)
			} else {
				fmt.Fprintf(w, `<p><em><a href="/login">Login</a> to follow streamers.</em></p>`)
			}

			fmt.Fprintf(w, `
	</div>
`)
		}
	}

	fmt.Fprintf(w, `
</body>
</html>`)
}

// HandleSearchAPI handles streamer search requests via JSON API (accessible to all users)
// POST /api/search
func (h *PublicHandler) HandleSearchAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Parse JSON request
	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "Search query is required", http.StatusBadRequest)
		return
	}

	// Perform search across all platforms
	results, err := h.searchService.SearchStreamers(ctx, req.Query)
	if err != nil {
		log.Printf("Error searching streamers: %v", err)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"query":   req.Query,
		"results": results,
	})
}

// GetUserFromSession retrieves the user from the session
func (h *PublicHandler) GetUserFromSession(ctx context.Context, r *http.Request) (*domain.User, error) {
	userID, err := h.sessionManager.GetSession(r)
	if err != nil {
		return nil, err
	}

	user, err := h.userService.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// renderError renders a user-friendly error page
func (h *PublicHandler) renderError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)

	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>Error - Who Live When</title>
	<style>
		body { 
			font-family: Arial, sans-serif; 
			margin: 40px auto; 
			max-width: 600px;
			text-align: center;
		}
		.error-container {
			background-color: #f8d7da;
			border: 1px solid #f5c6cb;
			border-radius: 5px;
			padding: 20px;
			margin: 20px 0;
		}
		.error-title {
			color: #721c24;
			font-size: 24px;
			margin-bottom: 10px;
		}
		.error-message {
			color: #721c24;
			font-size: 16px;
		}
		.back-link {
			margin-top: 20px;
		}
		.back-link a {
			color: #007bff;
			text-decoration: none;
		}
		.back-link a:hover {
			text-decoration: underline;
		}
	</style>
</head>
<body>
	<div class="error-container">
		<div class="error-title">Oops! Something went wrong</div>
		<div class="error-message">%s</div>
	</div>
	<div class="back-link">
		<a href="/">← Return to Home</a>
	</div>
</body>
</html>`, message)
}

// HandleDashboard displays the dashboard with programme streamers (public access)
// GET /dashboard
func (h *PublicHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get guest programme from session
	guestProgramme, _ := h.sessionManager.GetGuestProgramme(r)

	var programmeStreamers []*domain.Streamer
	hasCustomProgramme := false

	if guestProgramme != nil && len(guestProgramme.StreamerIDs) > 0 {
		hasCustomProgramme = true
		for _, streamerID := range guestProgramme.StreamerIDs {
			streamer, err := h.streamerService.GetStreamer(ctx, streamerID)
			if err != nil {
				h.logger.Warn("Failed to get streamer for programme", map[string]interface{}{
					"streamer_id": streamerID,
					"error":       err.Error(),
				})
				continue
			}
			programmeStreamers = append(programmeStreamers, streamer)
		}
	}

	// Get live status for programme streamers
	liveStatuses := make(map[string]*domain.LiveStatus)
	for _, streamer := range programmeStreamers {
		status, err := h.liveStatusService.GetLiveStatus(ctx, streamer.ID)
		if err != nil {
			h.logger.Warn("Failed to get live status", map[string]interface{}{
				"streamer_id": streamer.ID,
				"error":       err.Error(),
			})
			continue
		}
		liveStatuses[streamer.ID] = status
	}

	data := map[string]interface{}{
		"ProgrammeStreamers": programmeStreamers,
		"LiveStatuses":       liveStatuses,
		"HasCustomProgramme": hasCustomProgramme,
		"IsAuthenticated":    false,
	}

	if err := h.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		h.renderSimpleDashboard(w, programmeStreamers, liveStatuses, hasCustomProgramme)
	}
}

// renderSimpleDashboard renders a simple HTML dashboard page
func (h *PublicHandler) renderSimpleDashboard(w http.ResponseWriter, programmeStreamers []*domain.Streamer, liveStatuses map[string]*domain.LiveStatus, hasCustomProgramme bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>Dashboard - Who Live When</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 20px; }
		.streamer { border: 1px solid #ccc; padding: 10px; margin: 10px 0; }
		.live { background-color: #d4edda; }
		.offline { background-color: #f8d7da; }
		.header { background-color: #e7f3ff; padding: 10px; margin-bottom: 20px; }
		.programme-notice { background-color: #fff3cd; padding: 10px; margin: 10px 0; border-radius: 5px; }
	</style>
</head>
<body>
	<div class="header">
		<h1>Dashboard</h1>
		<p><a href="/">Home</a> | <a href="/calendar">Calendar</a> | <a href="/programme">Manage Programme</a></p>
	</div>
`)

	if hasCustomProgramme {
		fmt.Fprintf(w, `
	<div class="programme-notice" style="background-color: #d1ecf1; border-left: 4px solid #0c5460;">
		<h3>📅 Custom Programme Active</h3>
		<p>You're using a custom programme with %d streamer(s).</p>
		<a href="/programme">Manage Programme</a>
	</div>
`, len(programmeStreamers))
	} else {
		fmt.Fprintf(w, `
	<div class="programme-notice">
		<h3>🌍 Global Programme</h3>
		<p>Create a custom programme to personalize your calendar.</p>
		<a href="/programme">Create Custom Programme</a>
	</div>
`)
	}

	fmt.Fprintf(w, `
	<h2>Your Programme Streamers</h2>
	<form action="/search" method="POST" style="margin-bottom: 20px;">
		<input type="text" name="query" placeholder="Search for streamers..." required>
		<button type="submit">Search</button>
	</form>
`)

	if len(programmeStreamers) == 0 {
		fmt.Fprintf(w, `<p>No streamers in your programme yet. Use the search above to find streamers!</p>`)
	} else {
		for _, streamer := range programmeStreamers {
			status := liveStatuses[streamer.ID]
			liveClass := "offline"
			liveText := "Offline"
			streamLink := ""

			if status != nil && status.IsLive {
				liveClass = "live"
				liveText = fmt.Sprintf("Live on %s", status.Platform)
				if status.StreamURL != "" {
					streamLink = fmt.Sprintf(` - <a href="%s" target="_blank">Watch Stream</a>`, status.StreamURL)
				}
			}

			fmt.Fprintf(w, `
	<div class="streamer %s">
		<h3><a href="/streamer/%s">%s</a></h3>
		<p>Status: %s%s</p>
		<p>Platforms: %v</p>
		<form action="/programme/remove/%s" method="POST" style="display: inline;">
			<button type="submit">Remove from Programme</button>
		</form>
	</div>
`, liveClass, streamer.ID, streamer.Name, liveText, streamLink, streamer.Platforms, streamer.ID)
		}
	}

	fmt.Fprintf(w, `
</body>
</html>`)
}

// HandleCalendar displays the TV programme calendar view (public access)
// GET /calendar
func (h *PublicHandler) HandleCalendar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse week parameter (optional)
	weekParam := r.URL.Query().Get("week")
	var week time.Time
	if weekParam != "" {
		parsedWeek, err := time.Parse("2006-01-02", weekParam)
		if err != nil {
			h.logger.Warn("Error parsing week parameter", map[string]interface{}{
				"error": err.Error(),
			})
			week = time.Now()
		} else {
			week = parsedWeek
		}
	} else {
		week = time.Now()
	}

	// Get guest programme from session
	guestProgramme, _ := h.sessionManager.GetGuestProgramme(r)

	var calendarView *service.ProgrammeCalendarView
	var err error

	if guestProgramme != nil && len(guestProgramme.StreamerIDs) > 0 {
		customProgramme := &domain.CustomProgramme{
			StreamerIDs: guestProgramme.StreamerIDs,
		}
		calendarView, err = h.programmeService.GenerateCalendarFromProgramme(ctx, customProgramme, week)
	}

	// Fall back to global programme
	if calendarView == nil {
		calendarView, err = h.programmeService.GenerateGlobalProgramme(ctx, week, 10)
		if err != nil {
			h.logger.Error("Failed to generate programme", map[string]interface{}{
				"error": err.Error(),
			})
			h.renderError(w, "Unable to load calendar. Please try again later.", http.StatusInternalServerError)
			return
		}
	}

	// Create streamer map for template
	streamerMap := make(map[string]*domain.Streamer)
	for _, streamer := range calendarView.Streamers {
		streamerMap[streamer.ID] = streamer
	}

	// Calculate previous and next week dates
	prevWeek := week.AddDate(0, 0, -7)
	nextWeek := week.AddDate(0, 0, 7)

	// Convert to TVProgramme for template compatibility
	programme := &domain.TVProgramme{
		Entries: calendarView.Entries,
	}

	data := map[string]interface{}{
		"Programme":       programme,
		"StreamerMap":     streamerMap,
		"Week":            week,
		"PrevWeek":        prevWeek,
		"NextWeek":        nextWeek,
		"IsAuthenticated": false,
	}

	if err := h.templates.ExecuteTemplate(w, "calendar.html", data); err != nil {
		h.renderSimpleCalendar(w, programme, streamerMap, week, prevWeek, nextWeek)
	}
}

// renderSimpleCalendar renders a simple HTML calendar page
func (h *PublicHandler) renderSimpleCalendar(w http.ResponseWriter, programme *domain.TVProgramme, streamerMap map[string]*domain.Streamer, week, prevWeek, nextWeek time.Time) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>Calendar - Who Live When</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 20px; }
		.header { background-color: #e7f3ff; padding: 10px; margin-bottom: 20px; }
		.calendar { border-collapse: collapse; width: 100%%; }
		.calendar th, .calendar td { border: 1px solid #ccc; padding: 10px; text-align: left; vertical-align: top; }
		.calendar th { background-color: #f0f0f0; }
		.entry { background-color: #e7f3ff; padding: 5px; margin: 5px 0; border-radius: 3px; font-size: 0.9em; }
		.nav { margin: 20px 0; }
	</style>
</head>
<body>
	<div class="header">
		<h1>TV Programme Calendar</h1>
		<p><a href="/">Home</a> | <a href="/dashboard">Dashboard</a> | <a href="/programme">Manage Programme</a></p>
	</div>
	<div class="nav">
		<a href="/calendar?week=%s">← Previous Week</a> | 
		<strong>Week of %s</strong> | 
		<a href="/calendar?week=%s">Next Week →</a>
	</div>
`, prevWeek.Format("2006-01-02"), week.Format("2006-01-02"), nextWeek.Format("2006-01-02"))

	if len(programme.Entries) == 0 {
		fmt.Fprintf(w, `<p>No predictions available for this week. Add streamers to your programme to see their predicted live times!</p>`)
	} else {
		days := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

		grid := make(map[int]map[int][]*domain.ProgrammeEntry)
		for i := range programme.Entries {
			entry := &programme.Entries[i]
			if grid[entry.DayOfWeek] == nil {
				grid[entry.DayOfWeek] = make(map[int][]*domain.ProgrammeEntry)
			}
			grid[entry.DayOfWeek][entry.Hour] = append(grid[entry.DayOfWeek][entry.Hour], entry)
		}

		fmt.Fprintf(w, `
	<table class="calendar">
		<thead>
			<tr>
				<th>Time</th>
`)
		for _, day := range days {
			fmt.Fprintf(w, `				<th>%s</th>
`, day)
		}
		fmt.Fprintf(w, `			</tr>
		</thead>
		<tbody>
`)

		for hour := 0; hour < 24; hour++ {
			fmt.Fprintf(w, `			<tr>
				<td>%02d:00</td>
`, hour)
			for day := 0; day < 7; day++ {
				fmt.Fprintf(w, `				<td>
`)
				if entries, ok := grid[day][hour]; ok {
					for _, entry := range entries {
						streamer := streamerMap[entry.StreamerID]
						if streamer != nil {
							fmt.Fprintf(w, `					<div class="entry">
						<strong>%s</strong><br>
						%.0f%% likely
					</div>
`, streamer.Name, entry.Probability*100)
						}
					}
				}
				fmt.Fprintf(w, `				</td>
`)
			}
			fmt.Fprintf(w, `			</tr>
`)
		}

		fmt.Fprintf(w, `		</tbody>
	</table>
`)
	}

	fmt.Fprintf(w, `
</body>
</html>`)
}

// HandleAddStreamerFromSearch adds a streamer from search results to the database
// POST /streamer/add
func (h *PublicHandler) HandleAddStreamerFromSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.logger.Error("Failed to parse form", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	platform := r.FormValue("platform")
	handle := r.FormValue("handle")

	if platform == "" || handle == "" {
		http.Error(w, "Platform and handle are required", http.StatusBadRequest)
		return
	}

	// For now, only support Kick
	if platform != "kick" {
		http.Error(w, "Only Kick platform is currently supported", http.StatusBadRequest)
		return
	}

	// Get channel info from Kick API
	channelInfo, err := h.kickAdapter.GetChannelInfo(ctx, handle)
	if err != nil {
		h.logger.Error("Failed to get channel info from Kick", map[string]interface{}{
			"handle": handle,
			"error":  err.Error(),
		})
		h.renderError(w, "Could not find streamer on Kick. Please check the handle and try again.", http.StatusNotFound)
		return
	}

	// Use GetOrCreateStreamer to avoid duplicates
	streamer, err := h.streamerService.GetOrCreateStreamer(ctx, "kick", handle, channelInfo.Name)
	if err != nil {
		h.logger.Error("Failed to create streamer", map[string]interface{}{
			"handle": handle,
			"error":  err.Error(),
		})
		h.renderError(w, "Failed to add streamer. Please try again.", http.StatusInternalServerError)
		return
	}

	// Redirect to the streamer's page
	http.Redirect(w, r, "/streamer/"+streamer.ID, http.StatusSeeOther)
}
