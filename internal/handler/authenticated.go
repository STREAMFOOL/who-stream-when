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
	"who-live-when/internal/service"
)

// AuthenticatedHandler handles authenticated routes
type AuthenticatedHandler struct {
	tvProgrammeService domain.TVProgrammeService
	streamerService    domain.StreamerService
	liveStatusService  domain.LiveStatusService
	heatmapService     domain.HeatmapService
	userService        domain.UserService
	searchService      *service.SearchService
	sessionManager     *auth.SessionManager
	templates          *template.Template
}

// NewAuthenticatedHandler creates a new AuthenticatedHandler
func NewAuthenticatedHandler(
	tvProgrammeService domain.TVProgrammeService,
	streamerService domain.StreamerService,
	liveStatusService domain.LiveStatusService,
	heatmapService domain.HeatmapService,
	userService domain.UserService,
	searchService *service.SearchService,
	sessionManager *auth.SessionManager,
) *AuthenticatedHandler {
	// Load templates
	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		log.Printf("Warning: failed to load templates: %v", err)
		// Create empty template to avoid nil pointer
		tmpl = template.New("empty")
	}

	return &AuthenticatedHandler{
		tvProgrammeService: tvProgrammeService,
		streamerService:    streamerService,
		liveStatusService:  liveStatusService,
		heatmapService:     heatmapService,
		userService:        userService,
		searchService:      searchService,
		sessionManager:     sessionManager,
		templates:          tmpl,
	}
}

// HandleDashboard displays the user dashboard with followed streamers
// GET /dashboard
func (h *AuthenticatedHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := h.getUserIDFromContext(ctx)

	// Get user information
	user, err := h.userService.GetUser(ctx, userID)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		http.Error(w, "Failed to load user information", http.StatusInternalServerError)
		return
	}

	// Get followed streamers
	followedStreamers, err := h.userService.GetUserFollows(ctx, userID)
	if err != nil {
		log.Printf("Error getting user follows: %v", err)
		http.Error(w, "Failed to load followed streamers", http.StatusInternalServerError)
		return
	}

	// Get live status for all followed streamers
	liveStatuses := make(map[string]*domain.LiveStatus)
	for _, streamer := range followedStreamers {
		status, err := h.liveStatusService.GetLiveStatus(ctx, streamer.ID)
		if err != nil {
			log.Printf("Error getting live status for streamer %s: %v", streamer.ID, err)
			continue
		}
		liveStatuses[streamer.ID] = status
	}

	data := map[string]interface{}{
		"User":              user,
		"FollowedStreamers": followedStreamers,
		"LiveStatuses":      liveStatuses,
		"IsAuthenticated":   true,
	}

	// Try to render template, fallback to simple HTML if template not found
	if err := h.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		// Fallback to simple HTML response
		h.renderSimpleDashboard(w, user, followedStreamers, liveStatuses)
	}
}

// renderSimpleDashboard renders a simple HTML dashboard page
func (h *AuthenticatedHandler) renderSimpleDashboard(w http.ResponseWriter, user *domain.User, followedStreamers []*domain.Streamer, liveStatuses map[string]*domain.LiveStatus) {
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
	</style>
</head>
<body>
	<div class="header">
		<h1>Dashboard</h1>
		<p>Welcome, %s | <a href="/">Home</a> | <a href="/calendar">Calendar</a> | <a href="/logout">Logout</a></p>
	</div>
	<h2>Your Followed Streamers</h2>
	<form action="/search" method="POST" style="margin-bottom: 20px;">
		<input type="text" name="query" placeholder="Search for streamers..." required>
		<button type="submit">Search</button>
	</form>
`, user.Email)

	if len(followedStreamers) == 0 {
		fmt.Fprintf(w, `<p>You haven't followed any streamers yet. Use the search above to find streamers!</p>`)
	} else {
		for _, streamer := range followedStreamers {
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
		<form action="/unfollow/%s" method="POST" style="display: inline;">
			<button type="submit">Unfollow</button>
		</form>
	</div>
`, liveClass, streamer.ID, streamer.Name, liveText, streamLink, streamer.Platforms, streamer.ID)
		}
	}

	fmt.Fprintf(w, `
</body>
</html>`)
}

// HandleSearch handles streamer search requests
// POST /search
func (h *AuthenticatedHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	userID := h.getUserIDFromContext(ctx)

	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	query := r.FormValue("query")
	if query == "" {
		http.Error(w, "Search query is required", http.StatusBadRequest)
		return
	}

	// Perform search across all platforms
	results, err := h.searchService.SearchStreamers(ctx, query)
	if err != nil {
		log.Printf("Error searching streamers: %v", err)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	// Get user's followed streamers to check which ones are already followed
	followedStreamers, err := h.userService.GetUserFollows(ctx, userID)
	if err != nil {
		log.Printf("Error getting user follows: %v", err)
		// Continue without follow information
	}

	// Create a map of followed streamer handles for quick lookup
	followedHandles := make(map[string]bool)
	for _, streamer := range followedStreamers {
		for _, handle := range streamer.Handles {
			followedHandles[handle] = true
		}
	}

	data := map[string]interface{}{
		"Query":           query,
		"Results":         results,
		"FollowedHandles": followedHandles,
		"IsAuthenticated": true,
	}

	// Try to render template, fallback to simple HTML if template not found
	if err := h.templates.ExecuteTemplate(w, "search.html", data); err != nil {
		// Fallback to simple HTML response
		h.renderSimpleSearch(w, query, results, followedHandles)
	}
}

// renderSimpleSearch renders a simple HTML search results page
func (h *AuthenticatedHandler) renderSimpleSearch(w http.ResponseWriter, query string, results []*service.SearchResult, followedHandles map[string]bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
		<p><a href="/dashboard">← Back to Dashboard</a></p>
	</div>
	<h2>Results for "%s"</h2>
	<form action="/search" method="POST" style="margin-bottom: 20px;">
		<input type="text" name="query" placeholder="Search for streamers..." value="%s" required>
		<button type="submit">Search</button>
	</form>
`, query, query)

	if len(results) == 0 {
		fmt.Fprintf(w, `<p>No streamers found matching your search.</p>`)
	} else {
		for _, result := range results {
			// Check if any handle is already followed
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

			if !isFollowed {
				// For now, we'll need to add the streamer first before following
				// This is a simplified version - in production, we'd handle this better
				fmt.Fprintf(w, `<p><em>To follow this streamer, they need to be added to the system first.</em></p>`)
			} else {
				fmt.Fprintf(w, `<p><em>Already following</em></p>`)
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

// HandleFollow handles following a streamer
// POST /follow/:id
func (h *AuthenticatedHandler) HandleFollow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	userID := h.getUserIDFromContext(ctx)
	streamerID := r.PathValue("id")

	if streamerID == "" {
		http.Error(w, "Streamer ID is required", http.StatusBadRequest)
		return
	}

	// Verify streamer exists
	_, err := h.streamerService.GetStreamer(ctx, streamerID)
	if err != nil {
		log.Printf("Error getting streamer: %v", err)
		http.Error(w, "Streamer not found", http.StatusNotFound)
		return
	}

	// Follow the streamer
	if err := h.userService.FollowStreamer(ctx, userID, streamerID); err != nil {
		log.Printf("Error following streamer: %v", err)
		http.Error(w, "Failed to follow streamer", http.StatusInternalServerError)
		return
	}

	// Redirect back to the streamer page or dashboard
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/dashboard"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// HandleUnfollow handles unfollowing a streamer
// POST /unfollow/:id
func (h *AuthenticatedHandler) HandleUnfollow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	userID := h.getUserIDFromContext(ctx)
	streamerID := r.PathValue("id")

	if streamerID == "" {
		http.Error(w, "Streamer ID is required", http.StatusBadRequest)
		return
	}

	// Unfollow the streamer
	if err := h.userService.UnfollowStreamer(ctx, userID, streamerID); err != nil {
		log.Printf("Error unfollowing streamer: %v", err)
		http.Error(w, "Failed to unfollow streamer", http.StatusInternalServerError)
		return
	}

	// Redirect back to the streamer page or dashboard
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/dashboard"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// HandleCalendar displays the TV programme calendar view
// GET /calendar
func (h *AuthenticatedHandler) HandleCalendar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := h.getUserIDFromContext(ctx)

	// Parse week parameter (optional)
	weekParam := r.URL.Query().Get("week")
	var week time.Time
	if weekParam != "" {
		parsedWeek, err := time.Parse("2006-01-02", weekParam)
		if err != nil {
			log.Printf("Error parsing week parameter: %v", err)
			week = time.Now()
		} else {
			week = parsedWeek
		}
	} else {
		week = time.Now()
	}

	// Generate TV programme for the user
	programme, err := h.tvProgrammeService.GenerateProgramme(ctx, userID, week)
	if err != nil {
		log.Printf("Error generating TV programme: %v", err)
		http.Error(w, "Failed to load calendar", http.StatusInternalServerError)
		return
	}

	// Get followed streamers for display
	followedStreamers, err := h.userService.GetUserFollows(ctx, userID)
	if err != nil {
		log.Printf("Error getting user follows: %v", err)
		http.Error(w, "Failed to load followed streamers", http.StatusInternalServerError)
		return
	}

	// Create a map of streamer ID to streamer for quick lookup
	streamerMap := make(map[string]*domain.Streamer)
	for _, streamer := range followedStreamers {
		streamerMap[streamer.ID] = streamer
	}

	// Calculate previous and next week dates
	prevWeek := week.AddDate(0, 0, -7)
	nextWeek := week.AddDate(0, 0, 7)

	data := map[string]interface{}{
		"Programme":       programme,
		"StreamerMap":     streamerMap,
		"Week":            week,
		"PrevWeek":        prevWeek,
		"NextWeek":        nextWeek,
		"IsAuthenticated": true,
	}

	// Try to render template, fallback to simple HTML if template not found
	if err := h.templates.ExecuteTemplate(w, "calendar.html", data); err != nil {
		// Fallback to simple HTML response
		h.renderSimpleCalendar(w, programme, streamerMap, week, prevWeek, nextWeek)
	}
}

// renderSimpleCalendar renders a simple HTML calendar page
func (h *AuthenticatedHandler) renderSimpleCalendar(w http.ResponseWriter, programme *domain.TVProgramme, streamerMap map[string]*domain.Streamer, week, prevWeek, nextWeek time.Time) {
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
		<p><a href="/dashboard">← Back to Dashboard</a> | <a href="/">Home</a> | <a href="/logout">Logout</a></p>
	</div>
	<div class="nav">
		<a href="/calendar?week=%s">← Previous Week</a> | 
		<strong>Week of %s</strong> | 
		<a href="/calendar?week=%s">Next Week →</a>
	</div>
`, prevWeek.Format("2006-01-02"), week.Format("2006-01-02"), nextWeek.Format("2006-01-02"))

	if len(programme.Entries) == 0 {
		fmt.Fprintf(w, `<p>No predictions available for this week. Follow more streamers to see their predicted live times!</p>`)
	} else {
		// Create a grid structure for the calendar
		days := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

		// Group entries by day and hour
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

		// Display hours from 0 to 23
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

// getUserIDFromContext retrieves the user ID from the request context
func (h *AuthenticatedHandler) getUserIDFromContext(ctx context.Context) string {
	userID, ok := ctx.Value("userID").(string)
	if !ok {
		return ""
	}
	return userID
}

// RequireAuth is a middleware that ensures the user is authenticated
func (h *AuthenticatedHandler) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session
		userID, err := h.sessionManager.GetSession(r)
		if err != nil || userID == "" {
			// Not authenticated - redirect to login
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Add user ID to context
		ctx := context.WithValue(r.Context(), "userID", userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// SearchResponse represents the JSON response for search API
type SearchResponse struct {
	Query   string                  `json:"query"`
	Results []*service.SearchResult `json:"results"`
}

// HandleSearchAPI handles streamer search requests via JSON API
// POST /api/search
func (h *AuthenticatedHandler) HandleSearchAPI(w http.ResponseWriter, r *http.Request) {
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
	json.NewEncoder(w).Encode(SearchResponse{
		Query:   req.Query,
		Results: results,
	})
}
