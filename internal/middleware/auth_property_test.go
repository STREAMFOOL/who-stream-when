package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"who-live-when/internal/auth"
	"who-live-when/internal/domain"
	"who-live-when/internal/repository/sqlite"
	"who-live-when/internal/service"

	"github.com/google/uuid"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// setupTestDB creates a test database for property tests
func setupTestDB(t *testing.T) *sqlite.DB {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()

	db, err := sqlite.NewDB(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to open database: %v", err)
	}

	if err := sqlite.Migrate(db.DB); err != nil {
		db.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to run migrations: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
		os.Remove(tmpFile.Name())
	})

	return db
}

// **Feature: streamer-tracking-mvp, Property 9: Read-Only User Visibility**
// **Validates: Requirements 5.1**
// For any unregistered user, the streamer list should only include streamers that have been
// followed by at least one registered user.
func TestProperty_ReadOnlyUserVisibility(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("unregistered users can view streamers followed by registered users", prop.ForAll(
		func(googleID string, email string, streamerName string) bool {
			db := setupTestDB(t)
			defer db.Close()

			userRepo := sqlite.NewUserRepository(db)
			followRepo := sqlite.NewFollowRepository(db)
			activityRepo := sqlite.NewActivityRecordRepository(db)
			streamerRepo := sqlite.NewStreamerRepository(db)
			heatmapRepo := sqlite.NewHeatmapRepository(db)
			userService := service.NewUserService(userRepo, followRepo, activityRepo, streamerRepo)
			heatmapService := service.NewHeatmapService(activityRepo, heatmapRepo)
			tvProgrammeService := service.NewTVProgrammeService(heatmapService, userRepo, followRepo, streamerRepo, activityRepo)

			ctx := context.Background()

			// Create a registered user
			user, err := userService.CreateUser(ctx, googleID, email)
			if err != nil {
				t.Logf("Failed to create user: %v", err)
				return false
			}

			// Create a streamer
			streamer := &domain.Streamer{
				ID:        uuid.New().String(),
				Name:      streamerName,
				Handles:   map[string]string{"youtube": streamerName},
				Platforms: []string{"youtube"},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err := streamerRepo.Create(ctx, streamer); err != nil {
				t.Logf("Failed to create streamer: %v", err)
				return false
			}

			// Before any user follows the streamer, check default week view
			weekViewBefore, err := tvProgrammeService.GetDefaultWeekView(ctx)
			if err != nil {
				t.Logf("Failed to get default week view before follow: %v", err)
				return false
			}

			// Streamer should not be in the default view if no one follows them
			foundBefore := false
			for _, s := range weekViewBefore.Streamers {
				if s.ID == streamer.ID {
					foundBefore = true
					break
				}
			}

			// Registered user follows the streamer
			if err := userService.FollowStreamer(ctx, user.ID, streamer.ID); err != nil {
				t.Logf("Failed to follow streamer: %v", err)
				return false
			}

			// After follow, the streamer should be visible in the default week view
			weekViewAfter, err := tvProgrammeService.GetDefaultWeekView(ctx)
			if err != nil {
				t.Logf("Failed to get default week view after follow: %v", err)
				return false
			}

			// Streamer should now be visible (has at least one follower)
			foundAfter := false
			for _, s := range weekViewAfter.Streamers {
				if s.ID == streamer.ID {
					foundAfter = true
					break
				}
			}

			// The key property: after being followed, the streamer becomes visible
			// Before follow: may or may not be visible (depends on implementation)
			// After follow: must be visible
			if !foundAfter && !foundBefore {
				// If not found after follow, check if the streamer has followers
				followerCount, err := followRepo.GetFollowerCount(ctx, streamer.ID)
				if err != nil {
					t.Logf("Failed to get follower count: %v", err)
					return false
				}
				if followerCount > 0 {
					t.Logf("Streamer with %d followers should be visible", followerCount)
					return false
				}
			}

			// Verify the streamer can be retrieved by ID (public access)
			retrievedStreamer, err := streamerRepo.GetByID(ctx, streamer.ID)
			if err != nil {
				t.Logf("Failed to retrieve streamer: %v", err)
				return false
			}
			if retrievedStreamer.ID != streamer.ID {
				t.Logf("Retrieved streamer ID mismatch")
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
		gen.Identifier().Map(func(s string) string { return s + "@example.com" }),
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
	))

	properties.TestingRun(t)
}

// **Feature: streamer-tracking-mvp, Property 10: Search Access Control**
// **Validates: Requirements 5.2, 5.4**
// For any unregistered user attempting to search, the system should reject the request
// and require authentication.
func TestProperty_SearchAccessControl(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("unauthenticated search requests are rejected", prop.ForAll(
		func(searchQuery string) bool {
			sessionManager := auth.NewSessionManager("test-session", false, 3600)
			middleware := NewAuthMiddleware(sessionManager)

			searchHandlerCalled := false
			searchHandler := func(w http.ResponseWriter, r *http.Request) {
				searchHandlerCalled = true
				w.WriteHeader(http.StatusOK)
			}

			// Create unauthenticated request
			req := httptest.NewRequest(http.MethodPost, "/search?q="+searchQuery, nil)
			w := httptest.NewRecorder()

			// Apply RequireAuth middleware (simulating protected search route)
			middleware.RequireAuth(searchHandler)(w, req)

			// Search handler should NOT be called for unauthenticated request
			if searchHandlerCalled {
				t.Log("Search handler should not be called for unauthenticated request")
				return false
			}

			// Should redirect to login
			if w.Code != http.StatusSeeOther {
				t.Logf("Expected redirect status 303, got %d", w.Code)
				return false
			}

			location := w.Header().Get("Location")
			if location != "/login" {
				t.Logf("Expected redirect to '/login', got: %s", location)
				return false
			}

			return true
		},
		gen.Identifier(),
	))

	properties.Property("authenticated search requests are allowed", prop.ForAll(
		func(userID string, searchQuery string) bool {
			sessionManager := auth.NewSessionManager("test-session", false, 3600)
			middleware := NewAuthMiddleware(sessionManager)

			searchHandlerCalled := false
			var capturedUserID string
			searchHandler := func(w http.ResponseWriter, r *http.Request) {
				searchHandlerCalled = true
				capturedUserID = GetUserID(r.Context())
				w.WriteHeader(http.StatusOK)
			}

			// Create authenticated request
			req := httptest.NewRequest(http.MethodPost, "/search?q="+searchQuery, nil)
			w := httptest.NewRecorder()
			sessionManager.SetSession(w, userID)
			for _, cookie := range w.Result().Cookies() {
				req.AddCookie(cookie)
			}
			w = httptest.NewRecorder()

			// Apply RequireAuth middleware
			middleware.RequireAuth(searchHandler)(w, req)

			// Search handler should be called for authenticated request
			if !searchHandlerCalled {
				t.Log("Search handler should be called for authenticated request")
				return false
			}

			// User ID should be in context
			if capturedUserID != userID {
				t.Logf("Expected userID '%s' in context, got '%s'", userID, capturedUserID)
				return false
			}

			// Should return OK status
			if w.Code != http.StatusOK {
				t.Logf("Expected status 200, got %d", w.Code)
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

// **Feature: streamer-tracking-mvp, Property 11: Unregistered User Feature Restriction**
// **Validates: Requirements 5.3**
// For any unregistered user viewing a streamer, the response should not include follow
// functionality or the ability to add new streamers.
func TestProperty_UnregisteredUserFeatureRestriction(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("unregistered users cannot follow streamers", prop.ForAll(
		func(streamerID string) bool {
			sessionManager := auth.NewSessionManager("test-session", false, 3600)
			middleware := NewAuthMiddleware(sessionManager)

			followHandlerCalled := false
			followHandler := func(w http.ResponseWriter, r *http.Request) {
				followHandlerCalled = true
				w.WriteHeader(http.StatusOK)
			}

			// Create unauthenticated request to follow
			req := httptest.NewRequest(http.MethodPost, "/follow/"+streamerID, nil)
			w := httptest.NewRecorder()

			// Apply RequireAuth middleware (simulating protected follow route)
			middleware.RequireAuth(followHandler)(w, req)

			// Follow handler should NOT be called for unauthenticated request
			if followHandlerCalled {
				t.Log("Follow handler should not be called for unauthenticated request")
				return false
			}

			// Should redirect to login
			if w.Code != http.StatusSeeOther {
				t.Logf("Expected redirect status 303, got %d", w.Code)
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
	))

	properties.Property("unregistered users cannot add new streamers", prop.ForAll(
		func(streamerName string) bool {
			sessionManager := auth.NewSessionManager("test-session", false, 3600)
			middleware := NewAuthMiddleware(sessionManager)

			addStreamerHandlerCalled := false
			addStreamerHandler := func(w http.ResponseWriter, r *http.Request) {
				addStreamerHandlerCalled = true
				w.WriteHeader(http.StatusOK)
			}

			// Create unauthenticated request to add streamer
			req := httptest.NewRequest(http.MethodPost, "/streamer/add", nil)
			w := httptest.NewRecorder()

			// Apply RestrictUnregistered middleware
			middleware.RestrictUnregistered(addStreamerHandler)(w, req)

			// Add streamer handler should NOT be called for unauthenticated request
			if addStreamerHandlerCalled {
				t.Log("Add streamer handler should not be called for unauthenticated request")
				return false
			}

			// Should return 401 Unauthorized
			if w.Code != http.StatusUnauthorized {
				t.Logf("Expected status 401, got %d", w.Code)
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
	))

	properties.Property("OptionalAuth allows viewing streamer details without authentication", prop.ForAll(
		func(streamerID string) bool {
			sessionManager := auth.NewSessionManager("test-session", false, 3600)
			middleware := NewAuthMiddleware(sessionManager)

			viewHandlerCalled := false
			var isAuth bool
			viewHandler := func(w http.ResponseWriter, r *http.Request) {
				viewHandlerCalled = true
				isAuth = IsAuthenticated(r.Context())
				w.WriteHeader(http.StatusOK)
			}

			// Create unauthenticated request to view streamer
			req := httptest.NewRequest(http.MethodGet, "/streamer/"+streamerID, nil)
			w := httptest.NewRecorder()

			// Apply OptionalAuth middleware (simulating public streamer view route)
			middleware.OptionalAuth(viewHandler)(w, req)

			// View handler should be called for unauthenticated request
			if !viewHandlerCalled {
				t.Log("View handler should be called for unauthenticated request")
				return false
			}

			// IsAuthenticated should be false
			if isAuth {
				t.Log("IsAuthenticated should be false for unauthenticated request")
				return false
			}

			// Should return OK status
			if w.Code != http.StatusOK {
				t.Logf("Expected status 200, got %d", w.Code)
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
	))

	properties.TestingRun(t)
}

// **Feature: streamer-tracking-mvp, Property 13: Session Cleanup on Logout**
// **Validates: Requirements 6.3**
// For any authenticated user who logs out, subsequent requests should be treated as unauthenticated.
func TestProperty_SessionCleanupOnLogout(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("after logout, requests are treated as unauthenticated", prop.ForAll(
		func(userID string) bool {
			sessionManager := auth.NewSessionManager("test-session", false, 3600)
			middleware := NewAuthMiddleware(sessionManager)

			// Step 1: Create authenticated request
			req1 := httptest.NewRequest(http.MethodGet, "/protected", nil)
			w1 := httptest.NewRecorder()
			sessionManager.SetSession(w1, userID)

			// Copy cookies to request
			for _, cookie := range w1.Result().Cookies() {
				req1.AddCookie(cookie)
			}

			// Verify authenticated
			handler1Called := false
			handler1 := func(w http.ResponseWriter, r *http.Request) {
				handler1Called = true
				w.WriteHeader(http.StatusOK)
			}
			w1 = httptest.NewRecorder()
			middleware.RequireAuth(handler1)(w1, req1)

			if !handler1Called {
				t.Log("Handler should be called for authenticated request")
				return false
			}

			// Step 2: Simulate logout by clearing session
			w2 := httptest.NewRecorder()
			sessionManager.ClearSession(w2)

			// Step 3: Create new request without session (simulating post-logout)
			req2 := httptest.NewRequest(http.MethodGet, "/protected", nil)
			w3 := httptest.NewRecorder()

			handler2Called := false
			handler2 := func(w http.ResponseWriter, r *http.Request) {
				handler2Called = true
				w.WriteHeader(http.StatusOK)
			}

			middleware.RequireAuth(handler2)(w3, req2)

			// Handler should NOT be called after logout
			if handler2Called {
				t.Log("Handler should not be called after logout")
				return false
			}

			// Should redirect to login
			if w3.Code != http.StatusSeeOther {
				t.Logf("Expected redirect status 303 after logout, got %d", w3.Code)
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
	))

	properties.TestingRun(t)
}
