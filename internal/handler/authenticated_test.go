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

// mockPlatformAdapter is a mock implementation of PlatformAdapter for testing
type mockPlatformAdapter struct {
	platform string
}

func newMockPlatformAdapter(platform string) *mockPlatformAdapter {
	return &mockPlatformAdapter{platform: platform}
}

func (m *mockPlatformAdapter) GetLiveStatus(ctx context.Context, handle string) (*domain.PlatformLiveStatus, error) {
	// Return mock offline status
	return &domain.PlatformLiveStatus{
		IsLive:      false,
		StreamURL:   "",
		Title:       "",
		Thumbnail:   "",
		ViewerCount: 0,
	}, nil
}

func (m *mockPlatformAdapter) SearchStreamer(ctx context.Context, query string) ([]*domain.PlatformStreamer, error) {
	// Return mock search results based on query
	if query == "" {
		return []*domain.PlatformStreamer{}, nil
	}

	// Return a mock result
	return []*domain.PlatformStreamer{
		{
			Handle:    query + "_handle",
			Name:      query + " Streamer",
			Platform:  m.platform,
			Thumbnail: "https://example.com/thumb.jpg",
		},
	}, nil
}

func (m *mockPlatformAdapter) GetChannelInfo(ctx context.Context, handle string) (*domain.PlatformChannelInfo, error) {
	// Return mock channel info
	return &domain.PlatformChannelInfo{
		Handle:      handle,
		Name:        handle + " Channel",
		Description: "Mock channel description",
		Thumbnail:   "https://example.com/thumb.jpg",
		Platform:    m.platform,
	}, nil
}

// setupTestAuthenticatedHandler creates a test handler with temporary database
func setupTestAuthenticatedHandler(t *testing.T) (*AuthenticatedHandler, *domain.User, *sqlite.DB, func()) {
	// Create temporary database file
	tmpFile := t.TempDir() + "/test.db"

	// Create database
	db, err := sqlite.NewDB(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Run migrations
	if err := sqlite.Migrate(db.DB); err != nil {
		db.Close()
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	streamerRepo := sqlite.NewStreamerRepository(db)
	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	liveStatusRepo := sqlite.NewLiveStatusRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)

	// Initialize mock platform adapters (no external API calls)
	platformAdapters := map[string]domain.PlatformAdapter{
		"youtube": newMockPlatformAdapter("youtube"),
		"kick":    newMockPlatformAdapter("kick"),
		"twitch":  newMockPlatformAdapter("twitch"),
	}

	// Initialize services
	streamerService := service.NewStreamerService(streamerRepo)
	heatmapService := service.NewHeatmapService(activityRepo, heatmapRepo)
	liveStatusService := service.NewLiveStatusService(streamerRepo, liveStatusRepo, platformAdapters)
	userService := service.NewUserService(userRepo, followRepo, activityRepo)
	tvProgrammeService := service.NewTVProgrammeService(heatmapService, userRepo, followRepo, streamerRepo, activityRepo)
	searchService := service.NewSearchService(
		platformAdapters["youtube"],
		platformAdapters["kick"],
		platformAdapters["twitch"],
	)

	// Initialize session manager
	sessionManager := auth.NewSessionManager("test-session", false, 3600)

	// Create handler
	handler := NewAuthenticatedHandler(
		tvProgrammeService,
		streamerService,
		liveStatusService,
		heatmapService,
		userService,
		searchService,
		sessionManager,
	)

	// Create a test user
	ctx := context.Background()
	user, err := userService.CreateUser(ctx, "test-google-id", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return handler, user, db, cleanup
}

// createAuthenticatedRequest creates a request with a valid session cookie
func createAuthenticatedRequest(_ *testing.T, handler *AuthenticatedHandler, user *domain.User, method, path string, body string) (*http.Request, *httptest.ResponseRecorder) {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	// Set session cookie
	w := httptest.NewRecorder()
	handler.sessionManager.SetSession(w, user.ID)

	// Copy cookie to request
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	// Add user ID to context (simulating middleware)
	ctx := context.WithValue(req.Context(), "userID", user.ID)
	req = req.WithContext(ctx)

	return req, httptest.NewRecorder()
}

// TestRequireAuthMiddleware tests that authentication middleware works correctly
func TestRequireAuthMiddleware(t *testing.T) {
	handler, user, _, cleanup := setupTestAuthenticatedHandler(t)
	defer cleanup()

	t.Run("redirects unauthenticated requests to login", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		w := httptest.NewRecorder()

		// Call handler with middleware
		handler.RequireAuth(handler.HandleDashboard)(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status 303, got %d", w.Code)
		}

		location := w.Header().Get("Location")
		if location != "/login" {
			t.Errorf("Expected redirect to '/login', got: %s", location)
		}
	})

	t.Run("allows authenticated requests", func(t *testing.T) {
		req, w := createAuthenticatedRequest(t, handler, user, http.MethodGet, "/dashboard", "")

		handler.HandleDashboard(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

// TestHandleDashboard tests the dashboard handler
func TestHandleDashboard(t *testing.T) {
	handler, user, _, cleanup := setupTestAuthenticatedHandler(t)
	defer cleanup()

	ctx := context.Background()

	// Create test streamers
	streamer1 := &domain.Streamer{
		ID:        "streamer-1",
		Name:      "Test Streamer 1",
		Handles:   map[string]string{"youtube": "teststreamer1"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	streamer2 := &domain.Streamer{
		ID:        "streamer-2",
		Name:      "Test Streamer 2",
		Handles:   map[string]string{"twitch": "teststreamer2"},
		Platforms: []string{"twitch"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := handler.streamerService.AddStreamer(ctx, streamer1); err != nil {
		t.Fatalf("Failed to create streamer 1: %v", err)
	}
	if err := handler.streamerService.AddStreamer(ctx, streamer2); err != nil {
		t.Fatalf("Failed to create streamer 2: %v", err)
	}

	// Follow streamers
	if err := handler.userService.FollowStreamer(ctx, user.ID, streamer1.ID); err != nil {
		t.Fatalf("Failed to follow streamer 1: %v", err)
	}
	if err := handler.userService.FollowStreamer(ctx, user.ID, streamer2.ID); err != nil {
		t.Fatalf("Failed to follow streamer 2: %v", err)
	}

	t.Run("displays followed streamers", func(t *testing.T) {
		req, w := createAuthenticatedRequest(t, handler, user, http.MethodGet, "/dashboard", "")

		handler.HandleDashboard(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Check that both streamers appear
		if !contains(body, streamer1.Name) {
			t.Errorf("Expected dashboard to contain streamer '%s'", streamer1.Name)
		}
		if !contains(body, streamer2.Name) {
			t.Errorf("Expected dashboard to contain streamer '%s'", streamer2.Name)
		}
	})

	t.Run("shows empty state when no streamers followed", func(t *testing.T) {
		// Create a new user with no follows
		newUser, err := handler.userService.CreateUser(ctx, "new-google-id", "new@example.com")
		if err != nil {
			t.Fatalf("Failed to create new user: %v", err)
		}

		req, w := createAuthenticatedRequest(t, handler, newUser, http.MethodGet, "/dashboard", "")

		handler.HandleDashboard(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Check for empty state message
		if !contains(body, "haven't followed") && !contains(body, "no streamers") {
			t.Error("Expected dashboard to show empty state message")
		}
	})
}

// TestHandleSearch tests the search handler
func TestHandleSearch(t *testing.T) {
	handler, user, _, cleanup := setupTestAuthenticatedHandler(t)
	defer cleanup()

	t.Run("requires authentication", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader("query=test"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		// Call with middleware
		handler.RequireAuth(handler.HandleSearch)(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected redirect status 303, got %d", w.Code)
		}

		location := w.Header().Get("Location")
		if location != "/login" {
			t.Errorf("Expected redirect to '/login', got: %s", location)
		}
	})

	t.Run("performs search with valid query", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("query", "test")

		req, w := createAuthenticatedRequest(t, handler, user, http.MethodPost, "/search", formData.Encode())

		handler.HandleSearch(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Check that search results page is displayed
		if !contains(body, "Search") || !contains(body, "test") {
			t.Error("Expected search results page with query")
		}
	})

	t.Run("rejects empty query", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("query", "")

		req, w := createAuthenticatedRequest(t, handler, user, http.MethodPost, "/search", formData.Encode())

		handler.HandleSearch(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("rejects non-POST requests", func(t *testing.T) {
		req, w := createAuthenticatedRequest(t, handler, user, http.MethodGet, "/search", "")

		handler.HandleSearch(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})
}

// TestHandleFollowUnfollow tests the follow and unfollow handlers
func TestHandleFollowUnfollow(t *testing.T) {
	handler, user, _, cleanup := setupTestAuthenticatedHandler(t)
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

	t.Run("follow operation adds streamer to user follows", func(t *testing.T) {
		req, w := createAuthenticatedRequest(t, handler, user, http.MethodPost, "/follow/"+streamer.ID, "")
		req.SetPathValue("id", streamer.ID)

		handler.HandleFollow(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status 303, got %d", w.Code)
		}

		// Verify streamer is now followed
		follows, err := handler.userService.GetUserFollows(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get user follows: %v", err)
		}

		found := false
		for _, f := range follows {
			if f.ID == streamer.ID {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected streamer to be in user's follows after follow operation")
		}
	})

	t.Run("unfollow operation removes streamer from user follows", func(t *testing.T) {
		// First ensure streamer is followed
		if err := handler.userService.FollowStreamer(ctx, user.ID, streamer.ID); err != nil {
			t.Fatalf("Failed to follow streamer: %v", err)
		}

		req, w := createAuthenticatedRequest(t, handler, user, http.MethodPost, "/unfollow/"+streamer.ID, "")
		req.SetPathValue("id", streamer.ID)

		handler.HandleUnfollow(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status 303, got %d", w.Code)
		}

		// Verify streamer is no longer followed
		follows, err := handler.userService.GetUserFollows(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get user follows: %v", err)
		}

		for _, f := range follows {
			if f.ID == streamer.ID {
				t.Error("Expected streamer to be removed from user's follows after unfollow operation")
			}
		}
	})

	t.Run("follow requires authentication", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/follow/"+streamer.ID, nil)
		req.SetPathValue("id", streamer.ID)
		w := httptest.NewRecorder()

		handler.RequireAuth(handler.HandleFollow)(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected redirect status 303, got %d", w.Code)
		}

		location := w.Header().Get("Location")
		if location != "/login" {
			t.Errorf("Expected redirect to '/login', got: %s", location)
		}
	})

	t.Run("follow returns 404 for non-existent streamer", func(t *testing.T) {
		req, w := createAuthenticatedRequest(t, handler, user, http.MethodPost, "/follow/non-existent", "")
		req.SetPathValue("id", "non-existent")

		handler.HandleFollow(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
}

// TestHandleCalendar tests the calendar handler
func TestHandleCalendar(t *testing.T) {
	handler, user, _, cleanup := setupTestAuthenticatedHandler(t)
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

	// Follow streamer
	if err := handler.userService.FollowStreamer(ctx, user.ID, streamer.ID); err != nil {
		t.Fatalf("Failed to follow streamer: %v", err)
	}

	// Create activity records for predictions
	for i := 0; i < 30; i++ {
		timestamp := time.Now().AddDate(0, 0, -i).Add(time.Hour * 14) // 2 PM each day
		if err := handler.heatmapService.RecordActivity(ctx, streamer.ID, timestamp); err != nil {
			t.Fatalf("Failed to record activity: %v", err)
		}
	}

	t.Run("displays calendar for current week", func(t *testing.T) {
		req, w := createAuthenticatedRequest(t, handler, user, http.MethodGet, "/calendar", "")

		handler.HandleCalendar(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Check that calendar is displayed
		if !contains(body, "Calendar") || !contains(body, "Week") {
			t.Error("Expected calendar page to be displayed")
		}
	})

	t.Run("displays calendar for specific week", func(t *testing.T) {
		// Request calendar for next week
		nextWeek := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
		req, w := createAuthenticatedRequest(t, handler, user, http.MethodGet, "/calendar?week="+nextWeek, "")

		handler.HandleCalendar(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Check that the specific week is displayed
		if !contains(body, nextWeek) {
			t.Errorf("Expected calendar to display week %s", nextWeek)
		}
	})

	t.Run("requires authentication", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/calendar", nil)
		w := httptest.NewRecorder()

		handler.RequireAuth(handler.HandleCalendar)(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected redirect status 303, got %d", w.Code)
		}

		location := w.Header().Get("Location")
		if location != "/login" {
			t.Errorf("Expected redirect to '/login', got: %s", location)
		}
	})
}

// TestSearchRequiresAuthentication tests that search requires authentication (Requirement 5.2, 5.4)
func TestSearchRequiresAuthentication(t *testing.T) {
	handler, _, _, cleanup := setupTestAuthenticatedHandler(t)
	defer cleanup()

	t.Run("unauthenticated search request is rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader("query=test"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		// Call with middleware (simulating the actual route setup)
		handler.RequireAuth(handler.HandleSearch)(w, req)

		// Should redirect to login
		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected redirect status 303, got %d", w.Code)
		}

		location := w.Header().Get("Location")
		if location != "/login" {
			t.Errorf("Expected redirect to '/login', got: %s", location)
		}
	})
}

// TestFollowUnfollowOperations tests follow/unfollow operations (Requirements 8.1, 8.2)
func TestFollowUnfollowOperations(t *testing.T) {
	handler, user, _, cleanup := setupTestAuthenticatedHandler(t)
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

	t.Run("follow operation is successful", func(t *testing.T) {
		req, w := createAuthenticatedRequest(t, handler, user, http.MethodPost, "/follow/"+streamer.ID, "")
		req.SetPathValue("id", streamer.ID)

		handler.HandleFollow(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status 303, got %d", w.Code)
		}

		// Verify follow was recorded
		follows, err := handler.userService.GetUserFollows(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get user follows: %v", err)
		}

		found := false
		for _, f := range follows {
			if f.ID == streamer.ID {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected streamer to be followed")
		}
	})

	t.Run("unfollow operation is successful", func(t *testing.T) {
		// Ensure streamer is followed first
		if err := handler.userService.FollowStreamer(ctx, user.ID, streamer.ID); err != nil {
			t.Fatalf("Failed to follow streamer: %v", err)
		}

		req, w := createAuthenticatedRequest(t, handler, user, http.MethodPost, "/unfollow/"+streamer.ID, "")
		req.SetPathValue("id", streamer.ID)

		handler.HandleUnfollow(w, req)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status 303, got %d", w.Code)
		}

		// Verify unfollow was recorded
		follows, err := handler.userService.GetUserFollows(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get user follows: %v", err)
		}

		for _, f := range follows {
			if f.ID == streamer.ID {
				t.Error("Expected streamer to be unfollowed")
			}
		}
	})
}

// TestCalendarDisplaysCorrectWeek tests that calendar displays the correct week (Requirement 9.4)
func TestCalendarDisplaysCorrectWeek(t *testing.T) {
	handler, user, _, cleanup := setupTestAuthenticatedHandler(t)
	defer cleanup()

	ctx := context.Background()

	// Create and follow a test streamer
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

	if err := handler.userService.FollowStreamer(ctx, user.ID, streamer.ID); err != nil {
		t.Fatalf("Failed to follow streamer: %v", err)
	}

	t.Run("displays current week by default", func(t *testing.T) {
		req, w := createAuthenticatedRequest(t, handler, user, http.MethodGet, "/calendar", "")

		handler.HandleCalendar(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Check that current week is displayed
		if !contains(body, "Week") {
			t.Error("Expected calendar to display week information")
		}
	})

	t.Run("displays specified week when week parameter is provided", func(t *testing.T) {
		// Request a specific week (2 weeks from now)
		targetWeek := time.Now().AddDate(0, 0, 14)
		weekParam := targetWeek.Format("2006-01-02")

		req, w := createAuthenticatedRequest(t, handler, user, http.MethodGet, "/calendar?week="+weekParam, "")

		handler.HandleCalendar(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Check that the specified week is displayed
		if !contains(body, weekParam) {
			t.Errorf("Expected calendar to display week %s", weekParam)
		}
	})

	t.Run("provides navigation to previous and next weeks", func(t *testing.T) {
		req, w := createAuthenticatedRequest(t, handler, user, http.MethodGet, "/calendar", "")

		handler.HandleCalendar(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Check for navigation links
		if !contains(body, "Previous") && !contains(body, "Next") {
			t.Error("Expected calendar to provide week navigation")
		}
	})
}
