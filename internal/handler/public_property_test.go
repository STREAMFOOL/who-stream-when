package handler

import (
	"context"
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
