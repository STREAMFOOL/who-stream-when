package handler

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"who-live-when/internal/auth"
	"who-live-when/internal/domain"
)

// PublicHandler handles public routes
type PublicHandler struct {
	tvProgrammeService domain.TVProgrammeService
	streamerService    domain.StreamerService
	liveStatusService  domain.LiveStatusService
	heatmapService     domain.HeatmapService
	userService        domain.UserService
	oauthConfig        *auth.GoogleOAuthConfig
	sessionManager     *auth.SessionManager
	stateStore         *auth.StateStore
	templates          *template.Template
}

// NewPublicHandler creates a new PublicHandler
func NewPublicHandler(
	tvProgrammeService domain.TVProgrammeService,
	streamerService domain.StreamerService,
	liveStatusService domain.LiveStatusService,
	heatmapService domain.HeatmapService,
	userService domain.UserService,
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
		oauthConfig:        oauthConfig,
		sessionManager:     sessionManager,
		stateStore:         stateStore,
		templates:          LoadTemplates(),
	}
}

// HandleHome displays the home page with default week view showing most viewed streamers
// GET /
func (h *PublicHandler) HandleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get default week view with most viewed streamers
	weekView, err := h.tvProgrammeService.GetDefaultWeekView(ctx)
	if err != nil {
		log.Printf("Error getting default week view: %v", err)
		http.Error(w, "Failed to load home page", http.StatusInternalServerError)
		return
	}

	// Get live status for all streamers in the week view
	liveStatuses := make(map[string]*domain.LiveStatus)
	for _, streamer := range weekView.Streamers {
		status, err := h.liveStatusService.GetLiveStatus(ctx, streamer.ID)
		if err != nil {
			log.Printf("Error getting live status for streamer %s: %v", streamer.ID, err)
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
		log.Printf("Error getting streamer: %v", err)
		http.Error(w, "Streamer not found", http.StatusNotFound)
		return
	}

	// Get live status
	liveStatus, err := h.liveStatusService.GetLiveStatus(ctx, streamerID)
	if err != nil {
		log.Printf("Error getting live status: %v", err)
		// Continue without live status
	}

	// Get heatmap
	heatmap, err := h.heatmapService.GenerateHeatmap(ctx, streamerID)
	if err != nil {
		log.Printf("Error generating heatmap: %v", err)
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
		log.Printf("Error generating state token: %v", err)
		http.Error(w, "Failed to initiate login", http.StatusInternalServerError)
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
// GET /auth/callback
func (h *PublicHandler) HandleAuthCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get state and code from query parameters
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	// Verify state token
	if !h.stateStore.Verify(state) {
		log.Printf("Invalid state token")
		http.Error(w, "Invalid state token", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	token, err := h.oauthConfig.Exchange(ctx, code)
	if err != nil {
		log.Printf("Error exchanging code for token: %v", err)
		http.Error(w, "Failed to authenticate", http.StatusInternalServerError)
		return
	}

	// Get user info from Google
	userInfo, err := h.oauthConfig.GetUserInfo(ctx, token)
	if err != nil {
		log.Printf("Error getting user info: %v", err)
		http.Error(w, "Failed to get user information", http.StatusInternalServerError)
		return
	}

	// Create or get user
	user, err := h.userService.CreateUser(ctx, userInfo.ID, userInfo.Email)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		http.Error(w, "Failed to create user account", http.StatusInternalServerError)
		return
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
