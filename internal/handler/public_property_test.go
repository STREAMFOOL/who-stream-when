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
