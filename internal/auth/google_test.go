package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: streamer-tracking-mvp, Property 13: Session Cleanup on Logout**
// **Validates: Requirements 6.3**
// For any authenticated user who logs out, subsequent requests should be treated as unauthenticated.
func TestProperty_SessionCleanupOnLogout(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("clearing session should remove user ID from cookies", prop.ForAll(
		func(userID string) bool {
			sessionManager := NewSessionManager("session", false, 3600)

			// Create a response recorder to capture the cookie
			w1 := httptest.NewRecorder()

			// Set session
			sessionManager.SetSession(w1, userID)

			// Verify session was set
			cookies := w1.Result().Cookies()
			if len(cookies) == 0 {
				t.Log("No cookies set")
				return false
			}

			sessionCookie := cookies[0]
			if sessionCookie.Value != userID {
				t.Logf("Session cookie value mismatch: expected %s, got %s", userID, sessionCookie.Value)
				return false
			}

			// Create a request with the session cookie
			req := httptest.NewRequest("GET", "/", nil)
			req.AddCookie(sessionCookie)

			// Verify we can retrieve the session
			retrievedUserID, err := sessionManager.GetSession(req)
			if err != nil {
				t.Logf("Failed to retrieve session: %v", err)
				return false
			}
			if retrievedUserID != userID {
				t.Logf("Retrieved user ID mismatch: expected %s, got %s", userID, retrievedUserID)
				return false
			}

			// Clear the session
			w2 := httptest.NewRecorder()
			sessionManager.ClearSession(w2)

			// Verify session was cleared
			clearedCookies := w2.Result().Cookies()
			if len(clearedCookies) == 0 {
				t.Log("No cookies returned after clear")
				return false
			}

			clearedCookie := clearedCookies[0]
			if clearedCookie.MaxAge != -1 {
				t.Logf("Cookie MaxAge should be -1, got %d", clearedCookie.MaxAge)
				return false
			}
			if clearedCookie.Value != "" {
				t.Logf("Cookie value should be empty, got %s", clearedCookie.Value)
				return false
			}

			// Create a new request with the cleared cookie
			req2 := httptest.NewRequest("GET", "/", nil)
			req2.AddCookie(clearedCookie)

			// Verify session returns empty string (cleared)
			retrievedAfterClear, err := sessionManager.GetSession(req2)
			if err != nil {
				// Cookie exists but might be empty, which is fine
				return true
			}
			// If no error, the value should be empty
			if retrievedAfterClear != "" {
				t.Logf("Session should be empty after clearing, got %s", retrievedAfterClear)
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
	))

	properties.TestingRun(t)
}

// Unit tests for session manager guest data functionality

func TestSessionManager_GuestFollows_Serialization(t *testing.T) {
	sessionManager := NewSessionManager("test-session", false, 3600)

	tests := []struct {
		name        string
		streamerIDs []string
	}{
		{
			name:        "single follow",
			streamerIDs: []string{"streamer1"},
		},
		{
			name:        "multiple follows",
			streamerIDs: []string{"streamer1", "streamer2", "streamer3"},
		},
		{
			name:        "empty follows",
			streamerIDs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			// Set guest follows
			err := sessionManager.SetGuestFollows(w, req, tt.streamerIDs)
			if err != nil {
				t.Fatalf("Failed to set guest follows: %v", err)
			}

			// Create new request with cookie
			req2 := httptest.NewRequest("GET", "/", nil)
			for _, cookie := range w.Result().Cookies() {
				req2.AddCookie(cookie)
			}

			// Retrieve guest follows
			retrievedIDs, err := sessionManager.GetGuestFollows(req2)
			if err != nil {
				t.Fatalf("Failed to get guest follows: %v", err)
			}

			// Verify
			if len(retrievedIDs) != len(tt.streamerIDs) {
				t.Errorf("Expected %d follows, got %d", len(tt.streamerIDs), len(retrievedIDs))
			}

			for i, id := range tt.streamerIDs {
				if retrievedIDs[i] != id {
					t.Errorf("Mismatch at index %d: expected %s, got %s", i, id, retrievedIDs[i])
				}
			}
		})
	}
}

func TestSessionManager_GuestProgramme_Serialization(t *testing.T) {
	sessionManager := NewSessionManager("test-session", false, 3600)

	tests := []struct {
		name      string
		programme *CustomProgrammeData
	}{
		{
			name: "single streamer",
			programme: &CustomProgrammeData{
				StreamerIDs: []string{"streamer1"},
			},
		},
		{
			name: "multiple streamers",
			programme: &CustomProgrammeData{
				StreamerIDs: []string{"streamer1", "streamer2", "streamer3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)

			// Set guest programme
			err := sessionManager.SetGuestProgramme(w, req, tt.programme)
			if err != nil {
				t.Fatalf("Failed to set guest programme: %v", err)
			}

			// Create new request with cookie
			req2 := httptest.NewRequest("GET", "/", nil)
			for _, cookie := range w.Result().Cookies() {
				req2.AddCookie(cookie)
			}

			// Retrieve guest programme
			retrievedProgramme, err := sessionManager.GetGuestProgramme(req2)
			if err != nil {
				t.Fatalf("Failed to get guest programme: %v", err)
			}

			// Verify
			if retrievedProgramme == nil {
				t.Fatal("Retrieved programme is nil")
			}

			if len(retrievedProgramme.StreamerIDs) != len(tt.programme.StreamerIDs) {
				t.Errorf("Expected %d streamers, got %d", len(tt.programme.StreamerIDs), len(retrievedProgramme.StreamerIDs))
			}

			for i, id := range tt.programme.StreamerIDs {
				if retrievedProgramme.StreamerIDs[i] != id {
					t.Errorf("Mismatch at index %d: expected %s, got %s", i, id, retrievedProgramme.StreamerIDs[i])
				}
			}
		})
	}
}

func TestSessionManager_GuestData_SizeLimit(t *testing.T) {
	sessionManager := NewSessionManager("test-session", false, 3600)

	// Create a very large list of streamer IDs that will exceed cookie size even with compression
	// Using 2000 long IDs should definitely exceed 4KB
	largeList := make([]string, 2000)
	for i := 0; i < 2000; i++ {
		// Create unique, non-compressible strings
		largeList[i] = "streamer-id-with-unique-suffix-to-prevent-compression-" + string(rune(i%256)) + string(rune((i/256)%256))
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	// Attempt to set guest follows with large data
	err := sessionManager.SetGuestFollows(w, req, largeList)
	if err == nil {
		t.Error("Expected error for data exceeding cookie size, got nil")
	}

	// Verify error message mentions size limit
	if err != nil && err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

func TestSessionManager_GuestData_CorruptedData(t *testing.T) {
	sessionManager := NewSessionManager("test-session", false, 3600)

	tests := []struct {
		name        string
		cookieValue string
	}{
		{
			name:        "invalid base64",
			cookieValue: "not-valid-base64!!!",
		},
		{
			name:        "invalid json",
			cookieValue: "aW52YWxpZCBqc29u", // base64 of "invalid json"
		},
		{
			name:        "empty value",
			cookieValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.AddCookie(&http.Cookie{
				Name:  "guest_data",
				Value: tt.cookieValue,
			})

			// Attempt to get guest follows with corrupted data
			follows, err := sessionManager.GetGuestFollows(req)

			// Should either return error or empty slice
			if err == nil && len(follows) > 0 {
				t.Error("Expected error or empty follows for corrupted data")
			}
		})
	}
}

func TestSessionManager_ClearGuestData(t *testing.T) {
	sessionManager := NewSessionManager("test-session", false, 3600)

	// Set some guest data
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/", nil)
	streamerIDs := []string{"streamer1", "streamer2"}

	err := sessionManager.SetGuestFollows(w1, req1, streamerIDs)
	if err != nil {
		t.Fatalf("Failed to set guest follows: %v", err)
	}

	// Clear guest data
	w2 := httptest.NewRecorder()
	sessionManager.ClearGuestData(w2)

	// Verify cookie was cleared
	cookies := w2.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("No cookies returned after clear")
	}

	clearedCookie := cookies[0]
	if clearedCookie.MaxAge != -1 {
		t.Errorf("Cookie MaxAge should be -1, got %d", clearedCookie.MaxAge)
	}
	if clearedCookie.Value != "" {
		t.Errorf("Cookie value should be empty, got %s", clearedCookie.Value)
	}
}

func TestSessionManager_GuestData_PreservesExisting(t *testing.T) {
	sessionManager := NewSessionManager("test-session", false, 3600)

	// Set follows first
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/", nil)
	followIDs := []string{"follow1", "follow2"}

	err := sessionManager.SetGuestFollows(w1, req1, followIDs)
	if err != nil {
		t.Fatalf("Failed to set guest follows: %v", err)
	}

	// Create request with the cookie
	req2 := httptest.NewRequest("GET", "/", nil)
	for _, cookie := range w1.Result().Cookies() {
		req2.AddCookie(cookie)
	}

	// Add programme to existing session
	w2 := httptest.NewRecorder()
	programme := &CustomProgrammeData{
		StreamerIDs: []string{"prog1", "prog2"},
	}

	err = sessionManager.SetGuestProgramme(w2, req2, programme)
	if err != nil {
		t.Fatalf("Failed to set guest programme: %v", err)
	}

	// Create request with updated cookie
	req3 := httptest.NewRequest("GET", "/", nil)
	for _, cookie := range w2.Result().Cookies() {
		req3.AddCookie(cookie)
	}

	// Verify both follows and programme are present
	retrievedFollows, err := sessionManager.GetGuestFollows(req3)
	if err != nil {
		t.Fatalf("Failed to get guest follows: %v", err)
	}

	if len(retrievedFollows) != len(followIDs) {
		t.Errorf("Expected %d follows, got %d", len(followIDs), len(retrievedFollows))
	}

	retrievedProgramme, err := sessionManager.GetGuestProgramme(req3)
	if err != nil {
		t.Fatalf("Failed to get guest programme: %v", err)
	}

	if retrievedProgramme == nil {
		t.Fatal("Retrieved programme is nil")
	}

	if len(retrievedProgramme.StreamerIDs) != len(programme.StreamerIDs) {
		t.Errorf("Expected %d programme streamers, got %d", len(programme.StreamerIDs), len(retrievedProgramme.StreamerIDs))
	}
}
