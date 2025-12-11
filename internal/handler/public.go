package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

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
	oauthConfig        *auth.GoogleOAuthConfig
	sessionManager     *auth.SessionManager
	stateStore         *auth.StateStore
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
	oauthConfig *auth.GoogleOAuthConfig,
	sessionManager *auth.SessionManager,
	stateStore *auth.StateStore,
) *PublicHandler {
	return &PublicHandler{
		tvProgrammeService: tvProgrammeService,
		streamerService:    streamerService,
		liveStatusService:  liveStatusService,
		heatmapService:     heatmapService,
		userService:        userService,
		searchService:      searchService,
		oauthConfig:        oauthConfig,
		sessionManager:     sessionManager,
		stateStore:         stateStore,
		templates:          LoadTemplates(),
		logger:             logger.Default(),
	}
}

// HandleHome displays the home page with default week view showing most viewed streamers
// GET /
func (h *PublicHandler) HandleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get default week view with most viewed streamers
	weekView, err := h.tvProgrammeService.GetDefaultWeekView(ctx)
	if err != nil {
		h.logger.Error("Failed to get default week view", map[string]interface{}{
			"error": err.Error(),
		})
		h.renderError(w, "Unable to load home page. Please try again later.", http.StatusInternalServerError)
		return
	}

	// Get live status for all streamers in the week view
	liveStatuses := make(map[string]*domain.LiveStatus)
	for _, streamer := range weekView.Streamers {
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

	// Check if user is authenticated
	userID, _ := h.sessionManager.GetSession(r)
	isAuthenticated := userID != ""

	data := map[string]interface{}{
		"WeekView":        weekView,
		"LiveStatuses":    liveStatuses,
		"IsAuthenticated": isAuthenticated,
	}

	// Try to render template, fallback to simple HTML if template not found
	if err := h.templates.ExecuteTemplate(w, "home.html", data); err != nil {
		// Fallback to simple HTML response
		h.renderSimpleHome(w, weekView, liveStatuses, isAuthenticated)
	}
}

// renderSimpleHome renders a simple HTML home page when templates are not available
func (h *PublicHandler) renderSimpleHome(w http.ResponseWriter, weekView *domain.WeekView, liveStatuses map[string]*domain.LiveStatus, isAuthenticated bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
	<h2>Most Viewed Streamers</h2>
`, func() string {
		if isAuthenticated {
			return `<p>You are logged in. <a href="/dashboard">Go to Dashboard</a> | <a href="/logout">Logout</a></p>`
		}
		return `<p>You are browsing as a guest. <a href="/login">Login with Google</a> to follow streamers.</p>`
	}())

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

// HandleLogin initiates Google OAuth flow
// GET /login
func (h *PublicHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	// Generate state token for CSRF protection
	state, err := auth.GenerateStateToken()
	if err != nil {
		h.logger.Error("Failed to generate state token", map[string]interface{}{
			"error": err.Error(),
		})
		h.renderError(w, "Unable to initiate login. Please try again.", http.StatusInternalServerError)
		return
	}

	// Store state token
	h.stateStore.Store(state)

	// Get OAuth URL
	authURL := h.oauthConfig.GetAuthURL(state)

	// Redirect to Google OAuth
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// HandleAuthCallback handles the OAuth callback from Google
// GET /auth/google/callback
func (h *PublicHandler) HandleAuthCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get state and code from query parameters
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	// Verify state token
	if !h.stateStore.Verify(state) {
		h.logger.Warn("Invalid OAuth state token", map[string]interface{}{
			"state": state,
		})
		h.renderError(w, "Invalid authentication request. Please try logging in again.", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	token, err := h.oauthConfig.Exchange(ctx, code)
	if err != nil {
		h.logger.Error("Failed to exchange OAuth code for token", map[string]interface{}{
			"error": err.Error(),
		})
		h.renderError(w, "Authentication failed. Please try again.", http.StatusInternalServerError)
		return
	}

	// Get user info from Google
	userInfo, err := h.oauthConfig.GetUserInfo(ctx, token)
	if err != nil {
		h.logger.Error("Failed to get user info from Google", map[string]interface{}{
			"error": err.Error(),
		})
		h.renderError(w, "Unable to retrieve your account information. Please try again.", http.StatusInternalServerError)
		return
	}

	// Create or get user
	user, err := h.userService.CreateUser(ctx, userInfo.ID, userInfo.Email)
	if err != nil {
		h.logger.Error("Failed to create user account", map[string]interface{}{
			"google_id": userInfo.ID,
			"email":     userInfo.Email,
			"error":     err.Error(),
		})
		h.renderError(w, "Unable to create your account. Please try again.", http.StatusInternalServerError)
		return
	}

	// Migrate guest data if exists
	guestFollows, _ := h.sessionManager.GetGuestFollows(r)
	guestProgrammeData, _ := h.sessionManager.GetGuestProgramme(r)

	var guestProgramme *domain.CustomProgramme
	if guestProgrammeData != nil {
		guestProgramme = &domain.CustomProgramme{
			StreamerIDs: guestProgrammeData.StreamerIDs,
		}
	}

	if len(guestFollows) > 0 || guestProgramme != nil {
		if err := h.userService.MigrateGuestData(ctx, user.ID, guestFollows, guestProgramme); err != nil {
			h.logger.Error("Failed to migrate guest data", map[string]interface{}{
				"user_id": user.ID,
				"error":   err.Error(),
			})
			// Don't fail the login, just log the error
		} else {
			// Clear guest data after successful migration
			h.sessionManager.ClearGuestData(w)
		}
	}

	// Set session
	h.sessionManager.SetSession(w, user.ID)

	// Redirect to home page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HandleLogout clears the user session
// GET /logout
func (h *PublicHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Clear session
	h.sessionManager.ClearSession(w)

	// Redirect to home page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HandleSearch handles streamer search requests (accessible to all users)
// POST /search
func (h *PublicHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
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
