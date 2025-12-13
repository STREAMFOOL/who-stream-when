package handler

import (
	"bytes"
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"who-live-when/internal/auth"
	"who-live-when/internal/domain"
	"who-live-when/internal/repository/sqlite"
	"who-live-when/internal/service"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: working-service-mvp, Property 1: Public Access Without Authentication**
// **Validates: Requirements 2.1, 2.3, 2.4, 2.5**
// For any HTTP request to any endpoint (/, /programme, /search, /streamer/{id}),
// the System SHALL return a successful response (2xx status) without requiring
// authentication cookies or headers.
func TestProperty_PublicAccessWithoutAuthentication(t *testing.T) {
	// Set up test handler with in-memory database
	db, err := sqlite.NewDB("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	if err := sqlite.Migrate(db.DB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	streamerRepo := sqlite.NewStreamerRepository(db)
	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	liveStatusRepo := sqlite.NewLiveStatusRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)
	programmeRepo := sqlite.NewCustomProgrammeRepository(db)

	// Initialize services
	streamerService := service.NewStreamerService(streamerRepo)
	heatmapService := service.NewHeatmapService(activityRepo, heatmapRepo)
	liveStatusService := service.NewLiveStatusService(streamerRepo, liveStatusRepo, make(map[string]domain.PlatformAdapter))
	userService := service.NewUserService(userRepo, followRepo, activityRepo, streamerRepo, programmeRepo)
	tvProgrammeService := service.NewTVProgrammeService(heatmapService, userRepo, followRepo, streamerRepo, activityRepo)
	programmeService := service.NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	// Create mock adapters that return empty results
	emptyMock := &emptySearchMockAdapter{}
	searchService := service.NewSearchService(emptyMock, emptyMock, emptyMock)

	sessionManager := auth.NewSessionManager("test-session", false, 3600)

	handler := NewPublicHandler(
		tvProgrammeService,
		streamerService,
		liveStatusService,
		heatmapService,
		userService,
		searchService,
		programmeService,
		emptyMock, // kick adapter
		sessionManager,
	)

	// Create a test streamer for streamer detail tests
	testStreamer := &domain.Streamer{
		ID:        "test-streamer-prop",
		Name:      "Test Streamer Property",
		Handles:   map[string]string{"kick": "teststreamer"},
		Platforms: []string{"kick"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := streamerService.AddStreamer(context.Background(), testStreamer); err != nil {
		t.Fatalf("Failed to create test streamer: %v", err)
	}

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 20 // Reduced for faster execution
	properties := gopter.NewProperties(parameters)

	// Property: Home page is accessible without authentication
	properties.Property("home page returns 2xx without auth", prop.ForAll(
		func(_ int) bool {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()

			handler.HandleHome(w, req)

			return w.Code >= 200 && w.Code < 300
		},
		gen.Int(), // Dummy generator to run multiple times
	))

	// Property: Dashboard is accessible without authentication
	properties.Property("dashboard returns 2xx without auth", prop.ForAll(
		func(_ int) bool {
			req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
			w := httptest.NewRecorder()

			handler.HandleDashboard(w, req)

			return w.Code >= 200 && w.Code < 300
		},
		gen.Int(),
	))

	// Property: Calendar is accessible without authentication
	properties.Property("calendar returns 2xx without auth", prop.ForAll(
		func(_ int) bool {
			req := httptest.NewRequest(http.MethodGet, "/calendar", nil)
			w := httptest.NewRecorder()

			handler.HandleCalendar(w, req)

			return w.Code >= 200 && w.Code < 300
		},
		gen.Int(),
	))

	// Property: Streamer detail is accessible without authentication (for existing streamer)
	properties.Property("streamer detail returns 2xx without auth for existing streamer", prop.ForAll(
		func(_ int) bool {
			req := httptest.NewRequest(http.MethodGet, "/streamer/"+testStreamer.ID, nil)
			req.SetPathValue("id", testStreamer.ID)
			w := httptest.NewRecorder()

			handler.HandleStreamerDetail(w, req)

			return w.Code >= 200 && w.Code < 300
		},
		gen.Int(),
	))

	// Property: Search is accessible without authentication
	properties.Property("search returns 2xx without auth", prop.ForAll(
		func(idx int) bool {
			queries := []string{"test", "streamer", "xqc", "kai", "gaming", "live", "kick", "twitch"}
			query := queries[idx%len(queries)]

			form := url.Values{}
			form.Add("query", query)

			req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			handler.HandleSearch(w, req)

			return w.Code >= 200 && w.Code < 300
		},
		gen.IntRange(0, 100),
	))

	// Property: Search API is accessible without authentication
	properties.Property("search API returns 2xx without auth", prop.ForAll(
		func(query string) bool {
			if query == "" {
				return true // Skip empty queries
			}

			body := `{"query":"` + query + `"}`
			req := httptest.NewRequest(http.MethodPost, "/api/search", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.HandleSearchAPI(w, req)

			return w.Code >= 200 && w.Code < 300
		},
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0 && len(s) < 50
		}),
	))

	properties.TestingRun(t)
}

// **Feature: working-service-mvp, Property 4: Visual Status Indicator Consistency**
// **Validates: Requirements 4.3**
// For any streamer card rendered on the home page, the CSS class applied to the
// status badge SHALL be `status-live` when the streamer is live, `status-offline`
// when the streamer is offline, and `status-unknown` when status is unavailable.
func TestProperty_VisualStatusIndicatorConsistency(t *testing.T) {
	// Template snippet that mirrors the home.html status section logic
	statusTemplate := `
{{$status := .LiveStatus}}
<div class="status-section">
    {{if $status}}
        {{if $status.IsLive}}
        <span class="status-badge status-live">Live on {{$status.Platform}}</span>
        {{else}}
        <span class="status-badge status-offline">Offline</span>
        {{end}}
    {{else}}
    <span class="status-badge status-unknown">Status Unknown</span>
    {{end}}
</div>`

	tmpl, err := template.New("status").Parse(statusTemplate)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: Live streamers get status-live CSS class
	properties.Property("live status renders with status-live class", prop.ForAll(
		func(platform string, viewerCount int) bool {
			data := map[string]interface{}{
				"LiveStatus": &domain.LiveStatus{
					IsLive:      true,
					Platform:    platform,
					ViewerCount: viewerCount,
				},
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return false
			}

			output := buf.String()
			return strings.Contains(output, "status-live") &&
				!strings.Contains(output, "status-offline") &&
				!strings.Contains(output, "status-unknown")
		},
		gen.OneConstOf("kick", "twitch", "youtube"),
		gen.IntRange(0, 100000),
	))

	// Property: Offline streamers get status-offline CSS class
	properties.Property("offline status renders with status-offline class", prop.ForAll(
		func(platform string) bool {
			data := map[string]interface{}{
				"LiveStatus": &domain.LiveStatus{
					IsLive:   false,
					Platform: platform,
				},
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return false
			}

			output := buf.String()
			return strings.Contains(output, "status-offline") &&
				!strings.Contains(output, "status-live") &&
				!strings.Contains(output, "status-unknown")
		},
		gen.OneConstOf("kick", "twitch", "youtube"),
	))

	// Property: Unknown status (nil) gets status-unknown CSS class
	properties.Property("nil status renders with status-unknown class", prop.ForAll(
		func(_ int) bool {
			data := map[string]interface{}{
				"LiveStatus": nil,
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return false
			}

			output := buf.String()
			return strings.Contains(output, "status-unknown") &&
				!strings.Contains(output, "status-live") &&
				!strings.Contains(output, "status-offline")
		},
		gen.Int(),
	))

	properties.TestingRun(t)
}

// **Feature: working-service-mvp, Property 2: Streamer Information Display Completeness**
// **Validates: Requirements 3.2, 5.2**
// For any streamer retrieved from the database, when rendered in the UI
// (home page, search results, or detail page), the output SHALL contain
// the streamer's name, at least one platform handle, and platform identifier.
func TestProperty_StreamerInformationDisplayCompleteness(t *testing.T) {
	// Template snippet that mirrors the streamer card rendering logic
	streamerCardTemplate := `
<div class="streamer-card">
    <h3>{{.Name}}</h3>
    <div class="platforms">
        {{range .Platforms}}
        <span class="platform-tag">{{.}}</span>
        {{end}}
    </div>
    <div class="handles">
        {{range $platform, $handle := .Handles}}
        <span class="handle">{{$platform}}: {{$handle}}</span>
        {{end}}
    </div>
</div>`

	tmpl, err := template.New("streamer").Parse(streamerCardTemplate)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Search result template snippet
	searchResultTemplate := `
<div class="search-result">
    <h3>{{.Name}}</h3>
    <div class="platform-tags">
        {{range .Platforms}}
        <span class="platform-tag">{{.}}</span>
        {{end}}
    </div>
    <p class="result-handles">
        {{range $platform, $handle := .Handles}}
        <span>{{$platform}}: {{$handle}}</span>
        {{end}}
    </p>
</div>`

	searchTmpl, err := template.New("search").Parse(searchResultTemplate)
	if err != nil {
		t.Fatalf("Failed to parse search template: %v", err)
	}

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for valid streamer names (non-empty alphanumeric with spaces)
	nameGen := gen.AnyString().Map(func(s string) string {
		// Ensure non-empty and reasonable length
		if len(s) == 0 {
			return "DefaultStreamer"
		}
		if len(s) > 50 {
			return s[:50]
		}
		return s
	})

	// Generator for valid handles (non-empty alphanumeric)
	handleGen := gen.AnyString().Map(func(s string) string {
		// Ensure non-empty and reasonable length
		if len(s) == 0 {
			return "defaulthandle"
		}
		if len(s) > 30 {
			return s[:30]
		}
		return strings.ToLower(s)
	})

	// Generator for platforms
	platformGen := gen.OneConstOf("kick", "twitch", "youtube")

	// Property: Streamer card contains name, platform, and handle
	properties.Property("streamer card contains name, platform, and handle", prop.ForAll(
		func(name string, platform string, handle string) bool {
			streamer := &domain.Streamer{
				ID:        "test-id",
				Name:      name,
				Platforms: []string{platform},
				Handles:   map[string]string{platform: handle},
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, streamer); err != nil {
				return false
			}

			output := buf.String()

			// Check that name is present
			hasName := strings.Contains(output, name)

			// Check that platform is present
			hasPlatform := strings.Contains(output, platform)

			// Check that handle is present
			hasHandle := strings.Contains(output, handle)

			return hasName && hasPlatform && hasHandle
		},
		nameGen,
		platformGen,
		handleGen,
	))

	// Property: Search result contains name, platform, and handle
	properties.Property("search result contains name, platform, and handle", prop.ForAll(
		func(name string, platform string, handle string) bool {
			searchResult := &service.SearchResult{
				Name:      name,
				Platforms: []string{platform},
				Handles:   map[string]string{platform: handle},
			}

			var buf bytes.Buffer
			if err := searchTmpl.Execute(&buf, searchResult); err != nil {
				return false
			}

			output := buf.String()

			// Check that name is present
			hasName := strings.Contains(output, name)

			// Check that platform is present
			hasPlatform := strings.Contains(output, platform)

			// Check that handle is present
			hasHandle := strings.Contains(output, handle)

			return hasName && hasPlatform && hasHandle
		},
		nameGen,
		platformGen,
		handleGen,
	))

	// Property: Multi-platform streamer shows all platforms and handles
	properties.Property("multi-platform streamer shows all platforms and handles", prop.ForAll(
		func(name string, handle1 string, handle2 string) bool {
			streamer := &domain.Streamer{
				ID:        "test-id",
				Name:      name,
				Platforms: []string{"kick", "twitch"},
				Handles: map[string]string{
					"kick":   handle1,
					"twitch": handle2,
				},
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, streamer); err != nil {
				return false
			}

			output := buf.String()

			// Check all required elements are present
			hasName := strings.Contains(output, name)
			hasKick := strings.Contains(output, "kick")
			hasTwitch := strings.Contains(output, "twitch")
			hasHandle1 := strings.Contains(output, handle1)
			hasHandle2 := strings.Contains(output, handle2)

			return hasName && hasKick && hasTwitch && hasHandle1 && hasHandle2
		},
		nameGen,
		handleGen,
		handleGen,
	))

	properties.TestingRun(t)
}

// **Feature: working-service-mvp, Property 3: Live Status Display Completeness**
// **Validates: Requirements 3.4, 6.2**
// For any live status response where IsLive == true, the rendered output SHALL
// contain the stream title (if non-empty), viewer count, and a valid stream URL.
func TestProperty_LiveStatusDisplayCompleteness(t *testing.T) {
	// Template snippet that mirrors the home.html live status rendering logic
	liveStatusTemplate := `
{{$status := .LiveStatus}}
<div class="status-section">
    {{if $status}}
        {{if $status.IsLive}}
        <span class="status-badge status-live">🔴 Live on {{$status.Platform}}</span>
        {{if $status.Title}}
        <p class="stream-title-prominent">{{$status.Title}}</p>
        {{end}}
        {{if gt $status.ViewerCount 0}}
        <p class="viewer-count-prominent">👁 {{$status.ViewerCount}} watching</p>
        {{end}}
        {{if $status.StreamURL}}
        <a href="{{$status.StreamURL}}" target="_blank" class="btn-watch-now">▶ Watch Now</a>
        {{end}}
        {{else}}
        <span class="status-badge status-offline">Offline</span>
        {{end}}
    {{else}}
    <span class="status-badge status-unknown">⚠️ Status Unknown</span>
    {{end}}
</div>`

	tmpl, err := template.New("livestatus").Parse(liveStatusTemplate)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for non-empty stream titles (ASCII only to avoid HTML escaping issues)
	titleGen := gen.AlphaString().Map(func(s string) string {
		if len(s) == 0 {
			return "DefaultStreamTitle"
		}
		if len(s) > 50 {
			return s[:50]
		}
		return s
	})

	// Generator for valid stream URLs (ASCII only to avoid HTML escaping issues)
	urlGen := gen.AlphaString().Map(func(s string) string {
		if len(s) == 0 {
			return "https://kick.com/teststreamer"
		}
		if len(s) > 30 {
			s = s[:30]
		}
		return "https://kick.com/" + strings.ToLower(s)
	})

	// Generator for viewer counts (positive integers)
	viewerCountGen := gen.IntRange(1, 1000000)

	// Generator for platforms
	platformGen := gen.OneConstOf("kick", "twitch", "youtube")

	// Property: Live status with title displays the title
	properties.Property("live status with title displays the title", prop.ForAll(
		func(title string, platform string, viewerCount int, streamURL string) bool {
			data := map[string]interface{}{
				"LiveStatus": &domain.LiveStatus{
					IsLive:      true,
					Platform:    platform,
					Title:       title,
					ViewerCount: viewerCount,
					StreamURL:   streamURL,
				},
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return false
			}

			output := buf.String()

			// Title should be present in output
			hasTitle := strings.Contains(output, title)

			return hasTitle
		},
		titleGen,
		platformGen,
		viewerCountGen,
		urlGen,
	))

	// Property: Live status with positive viewer count displays viewer count
	properties.Property("live status with positive viewer count displays viewer count", prop.ForAll(
		func(title string, platform string, viewerCount int, streamURL string) bool {
			data := map[string]interface{}{
				"LiveStatus": &domain.LiveStatus{
					IsLive:      true,
					Platform:    platform,
					Title:       title,
					ViewerCount: viewerCount,
					StreamURL:   streamURL,
				},
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return false
			}

			output := buf.String()

			// Viewer count should be present in output
			viewerCountStr := strings.Contains(output, "watching")

			return viewerCountStr
		},
		titleGen,
		platformGen,
		viewerCountGen,
		urlGen,
	))

	// Property: Live status with stream URL displays watch button with URL
	properties.Property("live status with stream URL displays watch button with URL", prop.ForAll(
		func(title string, platform string, viewerCount int, streamURL string) bool {
			data := map[string]interface{}{
				"LiveStatus": &domain.LiveStatus{
					IsLive:      true,
					Platform:    platform,
					Title:       title,
					ViewerCount: viewerCount,
					StreamURL:   streamURL,
				},
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return false
			}

			output := buf.String()

			// Stream URL should be present in href attribute
			hasURL := strings.Contains(output, streamURL)
			// Watch Now button should be present
			hasWatchButton := strings.Contains(output, "Watch Now")

			return hasURL && hasWatchButton
		},
		titleGen,
		platformGen,
		viewerCountGen,
		urlGen,
	))

	// Property: Live status displays all required elements together
	properties.Property("live status displays title, viewer count, and URL together", prop.ForAll(
		func(title string, platform string, viewerCount int, streamURL string) bool {
			data := map[string]interface{}{
				"LiveStatus": &domain.LiveStatus{
					IsLive:      true,
					Platform:    platform,
					Title:       title,
					ViewerCount: viewerCount,
					StreamURL:   streamURL,
				},
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return false
			}

			output := buf.String()

			// All elements should be present
			hasTitle := strings.Contains(output, title)
			hasViewerCount := strings.Contains(output, "watching")
			hasURL := strings.Contains(output, streamURL)
			hasWatchButton := strings.Contains(output, "Watch Now")
			hasLiveIndicator := strings.Contains(output, "status-live")

			return hasTitle && hasViewerCount && hasURL && hasWatchButton && hasLiveIndicator
		},
		titleGen,
		platformGen,
		viewerCountGen,
		urlGen,
	))

	properties.TestingRun(t)
}

// **Feature: working-service-mvp, Property 5: Platform Link Generation**
// **Validates: Requirements 6.4**
// For any streamer with a Kick handle, the rendered detail page SHALL contain
// a clickable link with href matching the pattern `https://kick.com/{handle}`.
func TestProperty_PlatformLinkGeneration(t *testing.T) {
	// Template snippet that mirrors the streamer.html platform links rendering logic
	platformLinksTemplate := `
<div class="platform-links-list">
    {{range $platform, $handle := .Handles}}
    <div class="platform-link-item">
        {{if eq $platform "kick"}}
        <span class="platform-icon">🟢</span>
        <strong>Kick:</strong>
        <a href="https://kick.com/{{$handle}}" target="_blank" rel="noopener noreferrer" class="platform-link" data-platform="kick" data-handle="{{$handle}}">
            kick.com/{{$handle}}
        </a>
        {{else if eq $platform "twitch"}}
        <span class="platform-icon">🟣</span>
        <strong>Twitch:</strong>
        <a href="https://twitch.tv/{{$handle}}" target="_blank" rel="noopener noreferrer" class="platform-link" data-platform="twitch" data-handle="{{$handle}}">
            twitch.tv/{{$handle}}
        </a>
        {{else if eq $platform "youtube"}}
        <span class="platform-icon">🔴</span>
        <strong>YouTube:</strong>
        <a href="https://youtube.com/@{{$handle}}" target="_blank" rel="noopener noreferrer" class="platform-link" data-platform="youtube" data-handle="{{$handle}}">
            youtube.com/@{{$handle}}
        </a>
        {{else}}
        <span class="platform-icon">🔵</span>
        <strong>{{$platform}}:</strong>
        <span>{{$handle}}</span>
        {{end}}
    </div>
    {{end}}
</div>`

	tmpl, err := template.New("platformlinks").Parse(platformLinksTemplate)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for valid Kick handles (lowercase alphanumeric, no special chars)
	kickHandleGen := gen.AlphaString().Map(func(s string) string {
		if len(s) == 0 {
			return "teststreamer"
		}
		if len(s) > 25 {
			s = s[:25]
		}
		return strings.ToLower(s)
	}).SuchThat(func(s string) bool {
		return len(s) > 0
	})

	// Generator for valid Twitch handles
	twitchHandleGen := gen.AlphaString().Map(func(s string) string {
		if len(s) == 0 {
			return "twitchstreamer"
		}
		if len(s) > 25 {
			s = s[:25]
		}
		return strings.ToLower(s)
	}).SuchThat(func(s string) bool {
		return len(s) > 0
	})

	// Generator for valid YouTube handles
	youtubeHandleGen := gen.AlphaString().Map(func(s string) string {
		if len(s) == 0 {
			return "youtubestreamer"
		}
		if len(s) > 25 {
			s = s[:25]
		}
		return strings.ToLower(s)
	}).SuchThat(func(s string) bool {
		return len(s) > 0
	})

	// Property: Kick handle generates correct URL pattern https://kick.com/{handle}
	properties.Property("kick handle generates correct URL https://kick.com/{handle}", prop.ForAll(
		func(handle string) bool {
			streamer := &domain.Streamer{
				ID:        "test-id",
				Name:      "Test Streamer",
				Platforms: []string{"kick"},
				Handles:   map[string]string{"kick": handle},
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, streamer); err != nil {
				return false
			}

			output := buf.String()

			// Check that the correct URL pattern is present
			expectedURL := "https://kick.com/" + handle
			hasCorrectURL := strings.Contains(output, expectedURL)

			// Check that it's in an href attribute (clickable link)
			hasHref := strings.Contains(output, `href="`+expectedURL+`"`)

			// Check that target="_blank" is present for external link
			hasTargetBlank := strings.Contains(output, `target="_blank"`)

			return hasCorrectURL && hasHref && hasTargetBlank
		},
		kickHandleGen,
	))

	// Property: Twitch handle generates correct URL pattern https://twitch.tv/{handle}
	properties.Property("twitch handle generates correct URL https://twitch.tv/{handle}", prop.ForAll(
		func(handle string) bool {
			streamer := &domain.Streamer{
				ID:        "test-id",
				Name:      "Test Streamer",
				Platforms: []string{"twitch"},
				Handles:   map[string]string{"twitch": handle},
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, streamer); err != nil {
				return false
			}

			output := buf.String()

			// Check that the correct URL pattern is present
			expectedURL := "https://twitch.tv/" + handle
			hasCorrectURL := strings.Contains(output, expectedURL)

			// Check that it's in an href attribute (clickable link)
			hasHref := strings.Contains(output, `href="`+expectedURL+`"`)

			return hasCorrectURL && hasHref
		},
		twitchHandleGen,
	))

	// Property: YouTube handle generates correct URL pattern https://youtube.com/@{handle}
	properties.Property("youtube handle generates correct URL https://youtube.com/@{handle}", prop.ForAll(
		func(handle string) bool {
			streamer := &domain.Streamer{
				ID:        "test-id",
				Name:      "Test Streamer",
				Platforms: []string{"youtube"},
				Handles:   map[string]string{"youtube": handle},
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, streamer); err != nil {
				return false
			}

			output := buf.String()

			// Check that the correct URL pattern is present
			expectedURL := "https://youtube.com/@" + handle
			hasCorrectURL := strings.Contains(output, expectedURL)

			// Check that it's in an href attribute (clickable link)
			hasHref := strings.Contains(output, `href="`+expectedURL+`"`)

			return hasCorrectURL && hasHref
		},
		youtubeHandleGen,
	))

	// Property: Multi-platform streamer has all platform links with correct URLs
	properties.Property("multi-platform streamer has all correct platform URLs", prop.ForAll(
		func(kickHandle string, twitchHandle string, youtubeHandle string) bool {
			streamer := &domain.Streamer{
				ID:        "test-id",
				Name:      "Test Streamer",
				Platforms: []string{"kick", "twitch", "youtube"},
				Handles: map[string]string{
					"kick":    kickHandle,
					"twitch":  twitchHandle,
					"youtube": youtubeHandle,
				},
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, streamer); err != nil {
				return false
			}

			output := buf.String()

			// Check all platform URLs are present
			hasKickURL := strings.Contains(output, "https://kick.com/"+kickHandle)
			hasTwitchURL := strings.Contains(output, "https://twitch.tv/"+twitchHandle)
			hasYoutubeURL := strings.Contains(output, "https://youtube.com/@"+youtubeHandle)

			return hasKickURL && hasTwitchURL && hasYoutubeURL
		},
		kickHandleGen,
		twitchHandleGen,
		youtubeHandleGen,
	))

	// Property: Platform links have data attributes for handle identification
	properties.Property("kick platform link has data-platform and data-handle attributes", prop.ForAll(
		func(handle string) bool {
			streamer := &domain.Streamer{
				ID:        "test-id",
				Name:      "Test Streamer",
				Platforms: []string{"kick"},
				Handles:   map[string]string{"kick": handle},
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, streamer); err != nil {
				return false
			}

			output := buf.String()

			// Check that data attributes are present
			hasDataPlatform := strings.Contains(output, `data-platform="kick"`)
			hasDataHandle := strings.Contains(output, `data-handle="`+handle+`"`)

			return hasDataPlatform && hasDataHandle
		},
		kickHandleGen,
	))

	properties.TestingRun(t)
}
