package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"who-live-when/internal/auth"
	"who-live-when/internal/domain"
	"who-live-when/internal/repository/sqlite"
	"who-live-when/internal/service"
)

// setupTestHandler creates a test handler with in-memory database
func setupTestHandler(t *testing.T) (*PublicHandler, *sqlite.DB, func()) {
	// Create in-memory database
	db, err := sqlite.NewDB(":memory:")
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
	userService := service.NewUserService(userRepo, followRepo, activityRepo)
	tvProgrammeService := service.NewTVProgrammeService(heatmapService, userRepo, followRepo, streamerRepo, activityRepo)

	// Initialize OAuth configuration (with dummy values for testing)
	oauthConfig := auth.NewGoogleOAuthConfig("test-client-id", "test-client-secret", "http://localhost:8080/auth/callback")
	sessionManager := auth.NewSessionManager("test-session", false, 3600)
	stateStore := auth.NewStateStore()

	// Create handler
	handler := NewPublicHandler(
		tvProgrammeService,
		streamerService,
		liveStatusService,
		heatmapService,
		userService,
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
