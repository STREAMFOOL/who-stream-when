package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestSessionManager_GetGuestFollows_ErrorHandling tests error handling for guest follows retrieval
func TestSessionManager_GetGuestFollows_ErrorHandling(t *testing.T) {
	sm := NewSessionManager("session", false, 3600)

	tests := []struct {
		name          string
		setupRequest  func() *http.Request
		expectedCount int
	}{
		{
			name: "returns empty slice when no guest data cookie exists",
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectedCount: 0,
		},
		{
			name: "returns empty slice when guest data cookie is empty",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(&http.Cookie{
					Name:  "guest_data",
					Value: "",
				})
				return req
			},
			expectedCount: 0,
		},
		{
			name: "handles corrupted base64 data gracefully",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(&http.Cookie{
					Name:  "guest_data",
					Value: "not-valid-base64!!!",
				})
				return req
			},
			expectedCount: 0,
		},
		{
			name: "handles invalid JSON data gracefully",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				invalidJSON := base64.URLEncoding.EncodeToString([]byte("not json"))
				req.AddCookie(&http.Cookie{
					Name:  "guest_data",
					Value: invalidJSON,
				})
				return req
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			follows, _ := sm.GetGuestFollows(req)

			if len(follows) != tt.expectedCount {
				t.Errorf("Expected %d follows, got %d", tt.expectedCount, len(follows))
			}
		})
	}
}

// TestSessionManager_SetGuestFollows_ErrorHandling tests error handling for guest follows storage
func TestSessionManager_SetGuestFollows_ErrorHandling(t *testing.T) {
	sm := NewSessionManager("session", false, 3600)

	tests := []struct {
		name          string
		streamerIDs   []string
		shouldSucceed bool
	}{
		{
			name:          "stores empty follows list",
			streamerIDs:   []string{},
			shouldSucceed: true,
		},
		{
			name:          "stores single follow",
			streamerIDs:   []string{"streamer-1"},
			shouldSucceed: true,
		},
		{
			name:          "stores multiple follows",
			streamerIDs:   []string{"streamer-1", "streamer-2", "streamer-3"},
			shouldSucceed: true,
		},
		{
			name:          "stores large number of follows",
			streamerIDs:   generateStreamerIDs(100),
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			err := sm.SetGuestFollows(w, req, tt.streamerIDs)

			if tt.shouldSucceed && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.shouldSucceed {
				// Verify cookie was set
				cookies := w.Result().Cookies()
				found := false
				for _, cookie := range cookies {
					if cookie.Name == "guest_data" && cookie.Value != "" {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected guest_data cookie to be set")
				}
			}
		})
	}
}

// TestSessionManager_GetGuestProgramme_ErrorHandling tests error handling for guest programme retrieval
func TestSessionManager_GetGuestProgramme_ErrorHandling(t *testing.T) {
	sm := NewSessionManager("session", false, 3600)

	tests := []struct {
		name         string
		setupRequest func() *http.Request
		expectedNil  bool
	}{
		{
			name: "returns nil when no guest data cookie exists",
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectedNil: true,
		},
		{
			name: "returns nil when guest data has no programme",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				guestData := &GuestData{
					FollowedStreamerIDs: []string{"streamer-1"},
					CustomProgramme:     nil,
					CreatedAt:           time.Now(),
				}
				jsonData, _ := json.Marshal(guestData)
				encoded := base64.URLEncoding.EncodeToString(jsonData)
				req.AddCookie(&http.Cookie{
					Name:  "guest_data",
					Value: encoded,
				})
				return req
			},
			expectedNil: true,
		},
		{
			name: "handles corrupted programme data gracefully",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(&http.Cookie{
					Name:  "guest_data",
					Value: "corrupted-data",
				})
				return req
			},
			expectedNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			programme, _ := sm.GetGuestProgramme(req)

			if tt.expectedNil && programme != nil {
				t.Error("Expected nil programme, got non-nil")
			}
		})
	}
}

// TestSessionManager_SetGuestProgramme_ErrorHandling tests error handling for guest programme storage
func TestSessionManager_SetGuestProgramme_ErrorHandling(t *testing.T) {
	sm := NewSessionManager("session", false, 3600)

	tests := []struct {
		name          string
		programme     *CustomProgrammeData
		shouldSucceed bool
	}{
		{
			name: "stores programme with empty streamer list",
			programme: &CustomProgrammeData{
				StreamerIDs: []string{},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			shouldSucceed: true,
		},
		{
			name: "stores programme with single streamer",
			programme: &CustomProgrammeData{
				StreamerIDs: []string{"streamer-1"},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			shouldSucceed: true,
		},
		{
			name: "stores programme with multiple streamers",
			programme: &CustomProgrammeData{
				StreamerIDs: []string{"streamer-1", "streamer-2", "streamer-3"},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			err := sm.SetGuestProgramme(w, req, tt.programme)

			if tt.shouldSucceed && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestSessionManager_ClearGuestDataCookie tests clearing guest data
func TestSessionManager_ClearGuestDataCookie(t *testing.T) {
	sm := NewSessionManager("session", false, 3600)

	t.Run("clears guest data cookie", func(t *testing.T) {
		w := httptest.NewRecorder()

		sm.ClearGuestData(w)

		cookies := w.Result().Cookies()
		found := false
		for _, cookie := range cookies {
			if cookie.Name == "guest_data" {
				found = true
				if cookie.MaxAge != -1 {
					t.Errorf("Expected MaxAge -1, got %d", cookie.MaxAge)
				}
				break
			}
		}

		if !found {
			t.Error("Expected guest_data cookie to be set for clearing")
		}
	})
}

// TestSessionManager_RoundTripDataIntegrity tests that data survives round-trip storage and retrieval
func TestSessionManager_RoundTripDataIntegrity(t *testing.T) {
	sm := NewSessionManager("session", false, 3600)

	tests := []struct {
		name        string
		streamerIDs []string
	}{
		{
			name:        "single streamer",
			streamerIDs: []string{"streamer-1"},
		},
		{
			name:        "multiple streamers",
			streamerIDs: []string{"streamer-1", "streamer-2", "streamer-3"},
		},
		{
			name:        "many streamers",
			streamerIDs: generateStreamerIDs(50),
		},
		{
			name:        "empty list",
			streamerIDs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store data
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			err := sm.SetGuestFollows(w, req, tt.streamerIDs)
			if err != nil {
				t.Fatalf("Failed to set guest follows: %v", err)
			}

			// Retrieve data
			newReq := httptest.NewRequest(http.MethodGet, "/", nil)
			for _, cookie := range w.Result().Cookies() {
				newReq.AddCookie(cookie)
			}

			follows, err := sm.GetGuestFollows(newReq)
			if err != nil {
				t.Fatalf("Failed to get guest follows: %v", err)
			}

			// Verify integrity
			if len(follows) != len(tt.streamerIDs) {
				t.Errorf("Expected %d follows, got %d", len(tt.streamerIDs), len(follows))
			}

			for i, follow := range follows {
				if follow != tt.streamerIDs[i] {
					t.Errorf("Follow mismatch at index %d: expected %s, got %s", i, tt.streamerIDs[i], follow)
				}
			}
		})
	}
}

// TestSessionManager_NilRequestHandling tests handling of nil requests
func TestSessionManager_NilRequestHandling(t *testing.T) {
	sm := NewSessionManager("session", false, 3600)

	t.Run("GetGuestProgramme handles nil request gracefully", func(t *testing.T) {
		programme, err := sm.GetGuestProgramme(nil)
		if err == nil {
			t.Error("Expected error for nil request")
		}
		if programme != nil {
			t.Error("Expected nil programme for nil request")
		}
	})
}

// TestSessionManager_GuestDataWithExistingFollows tests updating guest data when follows already exist
func TestSessionManager_GuestDataWithExistingFollows(t *testing.T) {
	sm := NewSessionManager("session", false, 3600)

	t.Run("preserves existing follows when setting programme", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		// Set initial follows
		initialFollows := []string{"streamer-1", "streamer-2"}
		err := sm.SetGuestFollows(w, req, initialFollows)
		if err != nil {
			t.Fatalf("Failed to set initial follows: %v", err)
		}

		// Copy cookie to new request
		newReq := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, cookie := range w.Result().Cookies() {
			newReq.AddCookie(cookie)
		}

		// Set programme
		programme := &CustomProgrammeData{
			StreamerIDs: []string{"streamer-1", "streamer-3"},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		w2 := httptest.NewRecorder()
		err = sm.SetGuestProgramme(w2, newReq, programme)
		if err != nil {
			t.Fatalf("Failed to set programme: %v", err)
		}

		// Copy new cookie to another request
		finalReq := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, cookie := range w2.Result().Cookies() {
			finalReq.AddCookie(cookie)
		}

		// Verify follows are still there
		follows, err := sm.GetGuestFollows(finalReq)
		if err != nil {
			t.Fatalf("Failed to get follows: %v", err)
		}

		if len(follows) != len(initialFollows) {
			t.Errorf("Expected %d follows, got %d", len(initialFollows), len(follows))
		}

		// Verify programme is there
		prog, err := sm.GetGuestProgramme(finalReq)
		if err != nil {
			t.Fatalf("Failed to get programme: %v", err)
		}

		if prog == nil {
			t.Error("Expected programme to be set")
		}
	})
}

// Helper functions

func generateStreamerIDs(count int) []string {
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = fmt.Sprintf("streamer-%d", i)
	}
	return ids
}
