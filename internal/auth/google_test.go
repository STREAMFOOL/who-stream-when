package auth

import (
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
