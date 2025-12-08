package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: user-experience-enhancements, Property 4: Guest User Follow Session Storage**
// **Validates: Requirements 2.2, 7.1**
// For any guest user and streamer, following the streamer should store the relationship in session data
// that persists across requests within the same session.
func TestProperty_GuestUserFollowSessionStorage(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("guest follows persist in session across requests", prop.ForAll(
		func(streamerIDs []string) bool {
			sessionManager := NewSessionManager("test-session", false, 3600)

			// Step 1: Store guest follows
			w1 := httptest.NewRecorder()
			req1 := httptest.NewRequest(http.MethodGet, "/", nil)
			if err := sessionManager.SetGuestFollows(w1, req1, streamerIDs); err != nil {
				t.Logf("Failed to set guest follows: %v", err)
				return false
			}

			// Step 2: Simulate new request with the session cookie
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for _, cookie := range w1.Result().Cookies() {
				req.AddCookie(cookie)
			}

			// Step 3: Retrieve guest follows
			retrievedIDs, err := sessionManager.GetGuestFollows(req)
			if err != nil {
				t.Logf("Failed to get guest follows: %v", err)
				return false
			}

			// Step 4: Verify the follows match
			if len(retrievedIDs) != len(streamerIDs) {
				t.Logf("Expected %d follows, got %d", len(streamerIDs), len(retrievedIDs))
				return false
			}

			for i, id := range streamerIDs {
				if retrievedIDs[i] != id {
					t.Logf("Mismatch at index %d: expected %s, got %s", i, id, retrievedIDs[i])
					return false
				}
			}

			return true
		},
		gen.SliceOf(gen.Identifier()).SuchThat(func(v []string) bool {
			return len(v) > 0 && len(v) <= 50 // Reasonable limit
		}),
	))

	properties.Property("guest programme persists in session across requests", prop.ForAll(
		func(streamerIDs []string) bool {
			sessionManager := NewSessionManager("test-session", false, 3600)

			programme := &CustomProgrammeData{
				StreamerIDs: streamerIDs,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			// Step 1: Store guest programme
			w1 := httptest.NewRecorder()
			req1 := httptest.NewRequest(http.MethodGet, "/", nil)
			if err := sessionManager.SetGuestProgramme(w1, req1, programme); err != nil {
				t.Logf("Failed to set guest programme: %v", err)
				return false
			}

			// Step 2: Simulate new request with the session cookie
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for _, cookie := range w1.Result().Cookies() {
				req.AddCookie(cookie)
			}

			// Step 3: Retrieve guest programme
			retrievedProgramme, err := sessionManager.GetGuestProgramme(req)
			if err != nil {
				t.Logf("Failed to get guest programme: %v", err)
				return false
			}

			// Step 4: Verify the programme matches
			if retrievedProgramme == nil {
				t.Log("Retrieved programme is nil")
				return false
			}

			if len(retrievedProgramme.StreamerIDs) != len(streamerIDs) {
				t.Logf("Expected %d streamers, got %d", len(streamerIDs), len(retrievedProgramme.StreamerIDs))
				return false
			}

			for i, id := range streamerIDs {
				if retrievedProgramme.StreamerIDs[i] != id {
					t.Logf("Mismatch at index %d: expected %s, got %s", i, id, retrievedProgramme.StreamerIDs[i])
					return false
				}
			}

			return true
		},
		gen.SliceOf(gen.Identifier()).SuchThat(func(v []string) bool {
			return len(v) > 0 && len(v) <= 50 // Reasonable limit
		}),
	))

	properties.TestingRun(t)
}

// **Feature: user-experience-enhancements, Property 11: Session Data Persistence Across Requests**
// **Validates: Requirements 7.3**
// For any guest user session data (follows and programme), making multiple requests within the same
// session should maintain that data without loss.
func TestProperty_SessionDataPersistenceAcrossRequests(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("session data persists across multiple requests", prop.ForAll(
		func(numFollows int, numProgramme int, numRequests int) bool {
			sessionManager := NewSessionManager("test-session", false, 3600)

			// Generate shorter IDs to fit in cookie
			followIDs := make([]string, numFollows)
			for i := 0; i < numFollows; i++ {
				followIDs[i] = fmt.Sprintf("f%d", i)
			}

			programmeIDs := make([]string, numProgramme)
			for i := 0; i < numProgramme; i++ {
				programmeIDs[i] = fmt.Sprintf("p%d", i)
			}

			// Step 1: Store both follows and programme
			w1 := httptest.NewRecorder()
			req0 := httptest.NewRequest(http.MethodGet, "/", nil)
			if err := sessionManager.SetGuestFollows(w1, req0, followIDs); err != nil {
				t.Logf("Failed to set guest follows: %v", err)
				return false
			}

			// Get the cookie from first write
			cookies := w1.Result().Cookies()

			// Step 2: Add programme to existing session
			req1 := httptest.NewRequest(http.MethodGet, "/", nil)
			for _, cookie := range cookies {
				req1.AddCookie(cookie)
			}

			programme := &CustomProgrammeData{
				StreamerIDs: programmeIDs,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			w2 := httptest.NewRecorder()
			if err := sessionManager.SetGuestProgramme(w2, req1, programme); err != nil {
				t.Logf("Failed to set guest programme: %v", err)
				return false
			}

			// Update cookies
			cookies = w2.Result().Cookies()

			// Step 3: Make multiple requests and verify data persists
			for i := 0; i < numRequests; i++ {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				for _, cookie := range cookies {
					req.AddCookie(cookie)
				}

				// Verify follows
				retrievedFollows, err := sessionManager.GetGuestFollows(req)
				if err != nil {
					t.Logf("Request %d: Failed to get guest follows: %v", i, err)
					return false
				}

				if len(retrievedFollows) != len(followIDs) {
					t.Logf("Request %d: Expected %d follows, got %d", i, len(followIDs), len(retrievedFollows))
					return false
				}

				for j, id := range followIDs {
					if retrievedFollows[j] != id {
						t.Logf("Request %d: Follow mismatch at index %d", i, j)
						return false
					}
				}

				// Verify programme
				retrievedProgramme, err := sessionManager.GetGuestProgramme(req)
				if err != nil {
					t.Logf("Request %d: Failed to get guest programme: %v", i, err)
					return false
				}

				if retrievedProgramme == nil {
					t.Logf("Request %d: Retrieved programme is nil", i)
					return false
				}

				if len(retrievedProgramme.StreamerIDs) != len(programmeIDs) {
					t.Logf("Request %d: Expected %d programme streamers, got %d", i, len(programmeIDs), len(retrievedProgramme.StreamerIDs))
					return false
				}

				for j, id := range programmeIDs {
					if retrievedProgramme.StreamerIDs[j] != id {
						t.Logf("Request %d: Programme mismatch at index %d", i, j)
						return false
					}
				}
			}

			return true
		},
		gen.IntRange(1, 20), // Number of follows
		gen.IntRange(1, 20), // Number of programme streamers
		gen.IntRange(2, 10), // Number of requests to test
	))

	properties.TestingRun(t)
}
