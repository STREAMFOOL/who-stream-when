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
)

// emptySearchMockAdapter returns empty results for all searches
type emptySearchMockAdapter struct{}

func (m *emptySearchMockAdapter) GetLiveStatus(ctx context.Context, handle string) (*domain.PlatformLiveStatus, error) {
	return nil, nil
}

func (m *emptySearchMockAdapter) SearchStreamer(ctx context.Context, query string) ([]*domain.PlatformStreamer, error) {
	return []*domain.PlatformStreamer{}, nil
}

func (m *emptySearchMockAdapter) GetChannelInfo(ctx context.Context, handle string) (*domain.PlatformChannelInfo, error) {
	return nil, nil
}

// setupTestHandler creates a test handler with in-memory database
func setupTestHandler(t *testing.T) (*PublicHandler, *sqlite.DB, func()) {
	// Create in-memory database with shared cache to work with connection pooling
	// Using file::memory:?cache=shared allows multiple connections to share the same in-memory database
	db, err := sqlite.NewDB("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Run migrations
	if err := sqlite.Migrate(db.DB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Verify tables exist
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='streamer_platforms'").Scan(&count)
	if err != nil || count == 0 {
		t.Fatalf("streamer_platforms table not created: err=%v, count=%d", err, count)
	}

	// Initialize repositories
	streamerRepo := sqlite.NewStreamerRepository(db)
	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	liveStatusRepo := sqlite.NewLiveStatusRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)

	// Initialize services
	streamerService := service.NewStreamerService(streamerRepo)
	heatmapService := service.NewHeatmapService(activityRepo, heatmapRepo)
	liveStatusService := service.NewLiveStatusService(streamerRepo, liveStatusRepo, make(map[string]domain.PlatformAdapter))
	programmeRepo := sqlite.NewCustomProgrammeRepository(db)
	userService := service.NewUserService(userRepo, followRepo, activityRepo, streamerRepo, programmeRepo)
	tvProgrammeService := service.NewTVProgrammeService(heatmapService, userRepo, followRepo, streamerRepo, activityRepo)
	programmeService := service.NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	// Initialize mock platform adapters for search
	mockYouTube := newMockPlatformAdapter("youtube")
	mockKick := newMockPlatformAdapter("kick")
	mockTwitch := newMockPlatformAdapter("twitch")
	searchService := service.NewSearchService(mockYouTube, mockKick, mockTwitch)

	// Initialize OAuth configuration (with dummy values for testing)
	oauthConfig := auth.NewGoogleOAuthConfig("test-client-id", "test-client-secret", "http://localhost:8080/auth/google/callback")
	sessionManager := auth.NewSessionManager("test-session", false, 3600)
	stateStore := auth.NewStateStore()

	// Create handler
	handler := NewPublicHandler(
		tvProgrammeService,
		streamerService,
		liveStatusService,
		heatmapService,
		userService,
		searchService,
		programmeService,
		oauthConfig,
		sessionManager,
		stateStore,
	)

	cleanup := func() {
		db.Close()
	}

	return handler, db, cleanup
}

// TestHandleHome tests the home page handler
func TestHandleHome(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	t.Run("displays home page for unauthenticated user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		handler.HandleHome(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Expected non-empty response body")
		}
	})

	t.Run("displays home page for authenticated user", func(t *testing.T) {
		// Create a test user
		ctx := context.Background()
		user, err := handler.userService.CreateUser(ctx, "test-google-id", "test@example.com")
		if err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		// Set session cookie
		handler.sessionManager.SetSession(w, user.ID)

		// Copy cookie to request
		for _, cookie := range w.Result().Cookies() {
			req.AddCookie(cookie)
		}

		// Reset recorder
		w = httptest.NewRecorder()
		handler.HandleHome(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

// TestHandleStreamerDetail tests the streamer detail page handler
func TestHandleStreamerDetail(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a test streamer
	ctx := context.Background()
	streamer := &domain.Streamer{
		ID:        "test-streamer-1",
		Name:      "Test Streamer",
		Handles:   map[string]string{"youtube": "teststreamer"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := handler.streamerService.AddStreamer(ctx, streamer); err != nil {
		t.Fatalf("Failed to create test streamer: %v", err)
	}

	// Create some activity records for heatmap
	for i := 0; i < 10; i++ {
		timestamp := time.Now().AddDate(0, 0, -i)
		if err := handler.heatmapService.RecordActivity(ctx, streamer.ID, timestamp); err != nil {
			t.Fatalf("Failed to record activity: %v", err)
		}
	}

	t.Run("displays streamer detail page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/streamer/"+streamer.ID, nil)
		req.SetPathValue("id", streamer.ID)
		w := httptest.NewRecorder()

		handler.HandleStreamerDetail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if body == "" {
			t.Error("Expected non-empty response body")
		}

		// Check that streamer name appears in response
		if !contains(body, streamer.Name) {
			t.Errorf("Expected response to contain streamer name '%s'", streamer.Name)
		}
	})

	t.Run("shows heatmap when available", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/streamer/"+streamer.ID, nil)
		req.SetPathValue("id", streamer.ID)
		w := httptest.NewRecorder()

		handler.HandleStreamerDetail(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		// Check for heatmap-related content
		if !contains(body, "Heatmap") && !contains(body, "heatmap") {
			t.Error("Expected response to contain heatmap information")
		}
	})

	t.Run("returns 404 for non-existent streamer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/streamer/non-existent", nil)
		req.SetPathValue("id", "non-existent")
		w := httptest.NewRecorder()

		handler.HandleStreamerDetail(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
}

// TestHandleLogin tests the login handler
func TestHandleLogin(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	t.Run("redirects to Google OAuth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		w := httptest.NewRecorder()

		handler.HandleLogin(w, req)

		if w.Code != http.StatusTemporaryRedirect {
			t.Errorf("Expected status 307, got %d", w.Code)
		}

		location := w.Header().Get("Location")
		if location == "" {
			t.Error("Expected Location header to be set")
		}

		// Check that it's a Google OAuth URL
		if !contains(location, "accounts.google.com") {
			t.Errorf("Expected redirect to Google OAuth, got: %s", location)
		}
	})
}

// TestHandleAuthCallback tests the OAuth callback handler
func TestHandleAuthCallback(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	t.Run("rejects invalid state token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=invalid&code=test", nil)
		w := httptest.NewRecorder()

		handler.HandleAuthCallback(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("requires state parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test", nil)
		w := httptest.NewRecorder()

		handler.HandleAuthCallback(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
}

// TestHandleLogout tests the logout handler
func TestHandleLogout(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	t.Run("clears session and redirects", func(t *testing.T) {
		// Create a test user and set session
		ctx := context.Background()
		user, err := handler.userService.CreateUser(ctx, "test-google-id", "test@example.com")
		if err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/logout", nil)
		w := httptest.NewRecorder()

		// Set session
		handler.sessionManager.SetSession(w, user.ID)

		// Copy cookie to request
		for _, cookie := range w.Result().Cookies() {
			req.AddCookie(cookie)
		}

		// Reset recorder and call logout
		w = httptest.NewRecorder()
		handler.HandleLogout(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status 303, got %d", w.Code)
		}

		location := w.Header().Get("Location")
		if location != "/" {
			t.Errorf("Expected redirect to '/', got: %s", location)
		}

		// Check that session cookie is cleared
		cookies := w.Result().Cookies()
		sessionCleared := false
		for _, cookie := range cookies {
			if cookie.Name == "test-session" && cookie.MaxAge < 0 {
				sessionCleared = true
				break
			}
		}

		if !sessionCleared {
			t.Error("Expected session cookie to be cleared")
		}
	})
}

// TestStreamerDetailShowsLiveStatusAndHeatmap tests that streamer detail page displays live status and heatmap
func TestStreamerDetailShowsLiveStatusAndHeatmap(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()

	// Create test streamer
	streamer := &domain.Streamer{
		ID:        "test-streamer",
		Name:      "Test Streamer",
		Handles:   map[string]string{"youtube": "teststreamer"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := handler.streamerService.AddStreamer(ctx, streamer); err != nil {
		t.Fatalf("Failed to create streamer: %v", err)
	}

	// Create activity records for heatmap
	for i := 0; i < 10; i++ {
		timestamp := time.Now().AddDate(0, 0, -i)
		if err := handler.heatmapService.RecordActivity(ctx, streamer.ID, timestamp); err != nil {
			t.Fatalf("Failed to record activity: %v", err)
		}
	}

	// Request streamer detail page
	req := httptest.NewRequest(http.MethodGet, "/streamer/"+streamer.ID, nil)
	req.SetPathValue("id", streamer.ID)
	w := httptest.NewRecorder()

	handler.HandleStreamerDetail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Verify streamer name is displayed
	if !contains(body, streamer.Name) {
		t.Errorf("Expected page to contain streamer name '%s'", streamer.Name)
	}

	// Verify heatmap information is displayed
	if !contains(body, "Heatmap") && !contains(body, "heatmap") && !contains(body, "data points") {
		t.Error("Expected page to contain heatmap information")
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

// TestErrorHandling tests graceful error handling
func TestErrorHandling(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	t.Run("handles non-existent streamer gracefully", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/streamer/non-existent-id", nil)
		req.SetPathValue("id", "non-existent-id")
		w := httptest.NewRecorder()

		handler.HandleStreamerDetail(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}

		body := w.Body.String()
		// Check for user-friendly error message
		if !contains(body, "not found") && !contains(body, "Not Found") {
			t.Error("Expected user-friendly error message for not found")
		}
	})

	t.Run("handles missing streamer ID gracefully", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/streamer/", nil)
		req.SetPathValue("id", "")
		w := httptest.NewRecorder()

		handler.HandleStreamerDetail(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("handles invalid OAuth state gracefully", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=invalid-state&code=test-code", nil)
		w := httptest.NewRecorder()

		handler.HandleAuthCallback(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}

		body := w.Body.String()
		// Check for user-friendly error message
		if !contains(body, "authentication") || !contains(body, "try") {
			t.Error("Expected user-friendly error message for invalid state")
		}
	})
}

// TestPlatformAPIFailureHandling tests handling of platform API failures
func TestPlatformAPIFailureHandling(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()

	// Create test streamer
	streamer := &domain.Streamer{
		ID:        "test-streamer-api-fail",
		Name:      "Test Streamer API Fail",
		Handles:   map[string]string{"youtube": "teststreamer"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := handler.streamerService.AddStreamer(ctx, streamer); err != nil {
		t.Fatalf("Failed to create streamer: %v", err)
	}

	t.Run("continues rendering page when live status fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/streamer/"+streamer.ID, nil)
		req.SetPathValue("id", streamer.ID)
		w := httptest.NewRecorder()

		handler.HandleStreamerDetail(w, req)

		// Should still return 200 even if live status fails
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		// Page should still show streamer name
		if !contains(body, streamer.Name) {
			t.Error("Expected page to show streamer name even when live status fails")
		}
	})

	t.Run("continues rendering page when heatmap generation fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/streamer/"+streamer.ID, nil)
		req.SetPathValue("id", streamer.ID)
		w := httptest.NewRecorder()

		handler.HandleStreamerDetail(w, req)

		// Should still return 200 even if heatmap fails
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		// Page should still show streamer information
		if !contains(body, streamer.Name) {
			t.Error("Expected page to show streamer info even when heatmap fails")
		}
	})
}

// TestUserFriendlyErrorMessages tests that error messages are user-friendly
func TestUserFriendlyErrorMessages(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	testCases := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedStatus int
		checkMessage   func(body string) bool
	}{
		{
			name: "streamer not found shows friendly message",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/streamer/missing", nil)
				req.SetPathValue("id", "missing")
				return req
			},
			expectedStatus: http.StatusNotFound,
			checkMessage: func(body string) bool {
				return contains(body, "not found") || contains(body, "Not Found")
			},
		},
		{
			name: "invalid OAuth state shows friendly message",
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/auth/callback?state=bad&code=test", nil)
			},
			expectedStatus: http.StatusBadRequest,
			checkMessage: func(body string) bool {
				return contains(body, "authentication") && contains(body, "try")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.setupRequest()
			w := httptest.NewRecorder()

			// Route to appropriate handler based on path
			if contains(req.URL.Path, "/streamer/") {
				handler.HandleStreamerDetail(w, req)
			} else if contains(req.URL.Path, "/auth/callback") {
				handler.HandleAuthCallback(w, req)
			}

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			body := w.Body.String()
			if !tc.checkMessage(body) {
				t.Errorf("Error message not user-friendly. Body: %s", body)
			}
		})
	}
}

// TestHandleSearch_UnauthenticatedAccess tests that search is accessible without authentication
func TestHandleSearch_UnauthenticatedAccess(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	t.Run("allows unauthenticated search access", func(t *testing.T) {
		form := url.Values{}
		form.Add("query", "test streamer")

		req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handler.HandleSearch(w, req)

		// Should return 200 OK, not redirect to login
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for unauthenticated search, got %d", w.Code)
		}
	})

	t.Run("rejects GET method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/search?query=test", nil)
		w := httptest.NewRecorder()

		handler.HandleSearch(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405 for GET request, got %d", w.Code)
		}
	})

	t.Run("requires query parameter", func(t *testing.T) {
		form := url.Values{}
		form.Add("query", "")

		req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handler.HandleSearch(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for empty query, got %d", w.Code)
		}
	})
}

// TestHandleSearch_NoResults tests search with no results
func TestHandleSearch_NoResults(t *testing.T) {
	// Create handler with mock adapters that return empty results
	db, err := sqlite.NewDB(":memory:")
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

	// Initialize services
	streamerService := service.NewStreamerService(streamerRepo)
	heatmapService := service.NewHeatmapService(activityRepo, heatmapRepo)
	liveStatusService := service.NewLiveStatusService(streamerRepo, liveStatusRepo, make(map[string]domain.PlatformAdapter))
	programmeRepo := sqlite.NewCustomProgrammeRepository(db)
	userService := service.NewUserService(userRepo, followRepo, activityRepo, streamerRepo, programmeRepo)
	tvProgrammeService := service.NewTVProgrammeService(heatmapService, userRepo, followRepo, streamerRepo, activityRepo)

	// Create mock adapters that return empty results
	emptyMock := &emptySearchMockAdapter{}
	searchService := service.NewSearchService(emptyMock, emptyMock, emptyMock)
	programmeService := service.NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	oauthConfig := auth.NewGoogleOAuthConfig("test-client-id", "test-client-secret", "http://localhost:8080/auth/google/callback")
	sessionManager := auth.NewSessionManager("test-session", false, 3600)
	stateStore := auth.NewStateStore()

	handler := NewPublicHandler(
		tvProgrammeService,
		streamerService,
		liveStatusService,
		heatmapService,
		userService,
		searchService,
		programmeService,
		oauthConfig,
		sessionManager,
		stateStore,
	)

	form := url.Values{}
	form.Add("query", "nonexistent streamer xyz123")

	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	// Should show "no results" message
	if !contains(body, "No") || !contains(body, "found") {
		t.Error("Expected 'no results found' message in response")
	}
}

// TestHandleSearch_ResultDisplay tests that search results are displayed correctly
func TestHandleSearch_ResultDisplay(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// The mock adapters return results based on query (query + "_handle", query + " Streamer")
	form := url.Values{}
	form.Add("query", "gamer")

	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should display search results with streamer names (mock returns "gamer Streamer")
	if !contains(body, "gamer Streamer") && !contains(body, "Streamer") {
		t.Error("Expected search results to contain streamer names")
	}

	// Should show login prompt for guest users
	if !contains(body, "Login") && !contains(body, "login") {
		t.Error("Expected login prompt for guest users")
	}
}

// TestHandleSearch_AuthenticatedUser tests search for authenticated users
func TestHandleSearch_AuthenticatedUser(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a test user
	ctx := context.Background()
	user, err := handler.userService.CreateUser(ctx, "test-google-id", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Set up session
	w := httptest.NewRecorder()
	handler.sessionManager.SetSession(w, user.ID)

	form := url.Values{}
	form.Add("query", "test streamer")

	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Copy session cookie to request
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	w = httptest.NewRecorder()
	handler.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	// Authenticated users should see "Back to Dashboard" link
	if !contains(body, "Dashboard") && !contains(body, "dashboard") {
		t.Error("Expected dashboard link for authenticated users")
	}
}

// TestHandleHome_CustomProgramme tests home page displays custom programme when it exists
func TestHandleHome_CustomProgramme(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user
	user, err := handler.userService.CreateUser(ctx, "test-google-id", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create test streamers
	streamer1 := &domain.Streamer{
		ID:        "streamer-1",
		Name:      "Custom Streamer 1",
		Handles:   map[string]string{"youtube": "custom1"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	streamer2 := &domain.Streamer{
		ID:        "streamer-2",
		Name:      "Custom Streamer 2",
		Handles:   map[string]string{"kick": "custom2"},
		Platforms: []string{"kick"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := handler.streamerService.AddStreamer(ctx, streamer1); err != nil {
		t.Fatalf("Failed to add streamer1: %v", err)
	}
	if err := handler.streamerService.AddStreamer(ctx, streamer2); err != nil {
		t.Fatalf("Failed to add streamer2: %v", err)
	}

	// Create custom programme for user
	_, err = handler.programmeService.CreateCustomProgramme(ctx, user.ID, []string{streamer1.ID, streamer2.ID})
	if err != nil {
		t.Fatalf("Failed to create custom programme: %v", err)
	}

	// Set up authenticated request
	w := httptest.NewRecorder()
	handler.sessionManager.SetSession(w, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	w = httptest.NewRecorder()
	handler.HandleHome(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should display custom programme streamers
	if !contains(body, streamer1.Name) {
		t.Errorf("Expected custom programme to show streamer '%s'", streamer1.Name)
	}
	if !contains(body, streamer2.Name) {
		t.Errorf("Expected custom programme to show streamer '%s'", streamer2.Name)
	}
}

// TestHandleHome_GlobalProgrammeFallback tests home page falls back to global programme
func TestHandleHome_GlobalProgrammeFallback(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user without custom programme
	user, err := handler.userService.CreateUser(ctx, "test-google-id-2", "test2@example.com")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create test streamers and have them followed by other users to rank them
	streamer1 := &domain.Streamer{
		ID:        "global-streamer-1",
		Name:      "Popular Streamer 1",
		Handles:   map[string]string{"youtube": "popular1"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := handler.streamerService.AddStreamer(ctx, streamer1); err != nil {
		t.Fatalf("Failed to add streamer: %v", err)
	}

	// Add some activity records for heatmap generation
	for i := 0; i < 10; i++ {
		timestamp := time.Now().AddDate(0, 0, -i)
		if err := handler.heatmapService.RecordActivity(ctx, streamer1.ID, timestamp); err != nil {
			t.Fatalf("Failed to record activity: %v", err)
		}
	}

	// Create another user to follow the streamer (for ranking)
	otherUser, err := handler.userService.CreateUser(ctx, "other-google-id", "other@example.com")
	if err != nil {
		t.Fatalf("Failed to create other user: %v", err)
	}
	if err := handler.userService.FollowStreamer(ctx, otherUser.ID, streamer1.ID); err != nil {
		t.Fatalf("Failed to follow streamer: %v", err)
	}

	// Set up authenticated request
	w := httptest.NewRecorder()
	handler.sessionManager.SetSession(w, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	w = httptest.NewRecorder()
	handler.HandleHome(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should display global programme (most followed streamers)
	if !contains(body, streamer1.Name) {
		t.Errorf("Expected global programme to show popular streamer '%s'", streamer1.Name)
	}
}

// TestHandleHome_ProgrammeTypeIndicator tests that programme type is indicated
func TestHandleHome_ProgrammeTypeIndicator(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("shows custom programme indicator", func(t *testing.T) {
		// Create test user with custom programme
		user, err := handler.userService.CreateUser(ctx, "test-google-id-3", "test3@example.com")
		if err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		// Create test streamer
		streamer := &domain.Streamer{
			ID:        "indicator-streamer-1",
			Name:      "Indicator Streamer",
			Handles:   map[string]string{"youtube": "indicator1"},
			Platforms: []string{"youtube"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := handler.streamerService.AddStreamer(ctx, streamer); err != nil {
			t.Fatalf("Failed to add streamer: %v", err)
		}

		// Add some activity records for heatmap generation
		for i := 0; i < 10; i++ {
			timestamp := time.Now().AddDate(0, 0, -i)
			if err := handler.heatmapService.RecordActivity(ctx, streamer.ID, timestamp); err != nil {
				t.Fatalf("Failed to record activity: %v", err)
			}
		}

		// Create custom programme
		_, err = handler.programmeService.CreateCustomProgramme(ctx, user.ID, []string{streamer.ID})
		if err != nil {
			t.Fatalf("Failed to create custom programme: %v", err)
		}

		// Set up authenticated request
		w := httptest.NewRecorder()
		handler.sessionManager.SetSession(w, user.ID)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, cookie := range w.Result().Cookies() {
			req.AddCookie(cookie)
		}

		w = httptest.NewRecorder()
		handler.HandleHome(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Should indicate custom programme
		if !contains(body, "Custom") && !contains(body, "custom") && !contains(body, "Your") && !contains(body, "your") {
			t.Error("Expected indicator showing custom programme")
		}
	})

	t.Run("shows global programme indicator", func(t *testing.T) {
		// Create test user without custom programme
		user, err := handler.userService.CreateUser(ctx, "test-google-id-4", "test4@example.com")
		if err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		// Set up authenticated request
		w := httptest.NewRecorder()
		handler.sessionManager.SetSession(w, user.ID)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, cookie := range w.Result().Cookies() {
			req.AddCookie(cookie)
		}

		w = httptest.NewRecorder()
		handler.HandleHome(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Should indicate global programme
		if !contains(body, "Most Viewed") && !contains(body, "Popular") && !contains(body, "Global") {
			t.Error("Expected indicator showing global programme")
		}
	})
}
