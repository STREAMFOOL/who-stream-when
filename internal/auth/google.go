package auth

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleOAuthConfig holds the OAuth configuration
type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	config       *oauth2.Config
}

// GoogleUserInfo represents user information from Google
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// NewGoogleOAuthConfig creates a new Google OAuth configuration
func NewGoogleOAuthConfig(clientID, clientSecret, redirectURL string) *GoogleOAuthConfig {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return &GoogleOAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		config:       config,
	}
}

// GetAuthURL generates the OAuth authorization URL with state
func (g *GoogleOAuthConfig) GetAuthURL(state string) string {
	return g.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// Exchange exchanges the authorization code for a token
func (g *GoogleOAuthConfig) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}
	return token, nil
}

// GetUserInfo retrieves user information from Google using the access token
func (g *GoogleOAuthConfig) GetUserInfo(ctx context.Context, token *oauth2.Token) (*GoogleUserInfo, error) {
	client := g.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user info: status %d, body: %s", resp.StatusCode, string(body))
	}

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &userInfo, nil
}

// GenerateStateToken generates a random state token for CSRF protection
func GenerateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate state token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GuestData represents session data for unauthenticated users.
// This data is stored in HTTP-only cookies and persists during the browser session.
// When a guest user registers, this data is migrated to database storage.
type GuestData struct {
	// FollowedStreamerIDs: List of streamer IDs the guest user is following
	FollowedStreamerIDs []string `json:"follows"`
	// CustomProgramme: Optional custom programme created by the guest user
	CustomProgramme *CustomProgrammeData `json:"programme,omitempty"`
	// CreatedAt: When this guest session was created
	CreatedAt time.Time `json:"created_at"`
}

// CustomProgrammeData represents a custom programme stored in session.
// This is the session-based equivalent of the database CustomProgramme model.
// Contains a list of streamer IDs that make up the user's personalized schedule.
type CustomProgrammeData struct {
	// StreamerIDs: List of streamer IDs in the custom programme
	StreamerIDs []string `json:"streamer_ids"`
	// CreatedAt: When the programme was created
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt: When the programme was last modified
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionManager manages user sessions with secure cookies.
// Handles both authenticated user sessions and guest user data storage.
// Guest data is stored in HTTP-only cookies with optional compression for large datasets.
// All cookies use SameSite=Lax for CSRF protection.
type SessionManager struct {
	cookieName      string // Name of the authenticated session cookie
	guestCookieName string // Name of the guest data cookie
	cookiePath      string // Cookie path (always "/")
	cookieDomain    string // Cookie domain (empty for current domain)
	secure          bool   // Secure flag (true in production)
	httpOnly        bool   // HttpOnly flag (always true for security)
	maxAge          int    // Session lifetime in seconds
	maxCookieSize   int    // Maximum cookie size in bytes (4KB limit)
}

// NewSessionManager creates a new session manager with the specified configuration.
// Parameters:
//   - cookieName: Name for the authenticated session cookie
//   - secure: Whether to set the Secure flag (true in production)
//   - maxAge: Session lifetime in seconds
//
// Guest data is stored in a separate "guest_data" cookie with automatic compression
// if the data exceeds half the maximum cookie size.
func NewSessionManager(cookieName string, secure bool, maxAge int) *SessionManager {
	return &SessionManager{
		cookieName:      cookieName,
		guestCookieName: "guest_data",
		cookiePath:      "/",
		secure:          secure,
		httpOnly:        true,
		maxAge:          maxAge,
		maxCookieSize:   4000, // Leave room for cookie overhead (4KB limit)
	}
}

// SetSession sets a session cookie with the user ID
func (sm *SessionManager) SetSession(w http.ResponseWriter, userID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    userID,
		Path:     sm.cookiePath,
		Domain:   sm.cookieDomain,
		MaxAge:   sm.maxAge,
		Secure:   sm.secure,
		HttpOnly: sm.httpOnly,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSession retrieves the user ID from the session cookie
func (sm *SessionManager) GetSession(r *http.Request) (string, error) {
	cookie, err := r.Cookie(sm.cookieName)
	if err != nil {
		return "", fmt.Errorf("session not found: %w", err)
	}
	return cookie.Value, nil
}

// ClearSession removes the session cookie
func (sm *SessionManager) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    "",
		Path:     sm.cookiePath,
		Domain:   sm.cookieDomain,
		MaxAge:   -1,
		Secure:   sm.secure,
		HttpOnly: sm.httpOnly,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
	})
}

// StateStore manages OAuth state tokens for CSRF protection
type StateStore struct {
	states map[string]time.Time
}

// NewStateStore creates a new state store
func NewStateStore() *StateStore {
	return &StateStore{
		states: make(map[string]time.Time),
	}
}

// Store stores a state token with expiration
func (s *StateStore) Store(state string) {
	s.states[state] = time.Now().Add(10 * time.Minute)
}

// Verify verifies and removes a state token
func (s *StateStore) Verify(state string) bool {
	expiry, exists := s.states[state]
	if !exists {
		return false
	}

	// Check if expired
	if time.Now().After(expiry) {
		delete(s.states, state)
		return false
	}

	// Remove after verification (one-time use)
	delete(s.states, state)
	return true
}

// Cleanup removes expired state tokens
func (s *StateStore) Cleanup() {
	now := time.Now()
	for state, expiry := range s.states {
		if now.After(expiry) {
			delete(s.states, state)
		}
	}
}

// GetGuestFollows retrieves the list of followed streamer IDs from guest session.
// Returns an empty slice if no guest data exists or on error.
// This allows graceful handling of missing session data.
func (sm *SessionManager) GetGuestFollows(r *http.Request) ([]string, error) {
	guestData, err := sm.getGuestData(r)
	if err != nil {
		return []string{}, nil // Return empty slice if no guest data
	}
	return guestData.FollowedStreamerIDs, nil
}

// SetGuestFollows stores the list of followed streamer IDs in guest session.
// Preserves any existing custom programme data.
// Automatically compresses data if it exceeds half the maximum cookie size.
// Returns error if the serialized data exceeds the maximum cookie size.
func (sm *SessionManager) SetGuestFollows(w http.ResponseWriter, r *http.Request, streamerIDs []string) error {
	// Try to get existing guest data
	guestData, err := sm.getGuestData(r)
	if err != nil {
		// No existing data, create new
		guestData = &GuestData{
			FollowedStreamerIDs: []string{},
			CreatedAt:           time.Now(),
		}
	}
	guestData.FollowedStreamerIDs = streamerIDs
	return sm.setGuestData(w, guestData)
}

// GetGuestProgramme retrieves the custom programme from guest session.
// Returns nil if no custom programme exists or on error.
// This allows checking if a guest user has created a custom programme.
func (sm *SessionManager) GetGuestProgramme(r *http.Request) (*CustomProgrammeData, error) {
	guestData, err := sm.getGuestData(r)
	if err != nil {
		return nil, err
	}
	return guestData.CustomProgramme, nil
}

// SetGuestProgramme stores the custom programme in guest session.
// Preserves any existing follows data.
// Automatically compresses data if it exceeds half the maximum cookie size.
// Returns error if the serialized data exceeds the maximum cookie size.
func (sm *SessionManager) SetGuestProgramme(w http.ResponseWriter, r *http.Request, programme *CustomProgrammeData) error {
	// Try to get existing guest data
	guestData, err := sm.getGuestData(r)
	if err != nil {
		// No existing data, create new
		guestData = &GuestData{
			FollowedStreamerIDs: []string{},
			CreatedAt:           time.Now(),
		}
	}
	guestData.CustomProgramme = programme
	return sm.setGuestData(w, guestData)
}

// ClearGuestData removes all guest session data by setting MaxAge to -1.
// This causes the browser to delete the cookie.
func (sm *SessionManager) ClearGuestData(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sm.guestCookieName,
		Value:    "",
		Path:     sm.cookiePath,
		Domain:   sm.cookieDomain,
		MaxAge:   -1,
		Secure:   sm.secure,
		HttpOnly: sm.httpOnly,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
	})
}

// getGuestData retrieves and deserializes guest data from cookie.
// Handles base64 decoding and automatic decompression if data was compressed.
// Returns error if cookie doesn't exist or data is corrupted.
// Gracefully handles corrupted data by returning an error rather than panicking.
func (sm *SessionManager) getGuestData(r *http.Request) (*GuestData, error) {
	if r == nil {
		return nil, fmt.Errorf("request is nil")
	}

	cookie, err := r.Cookie(sm.guestCookieName)
	if err != nil {
		return nil, fmt.Errorf("guest data not found: %w", err)
	}

	// Decode base64
	decoded, err := base64.URLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to decode guest data: %w", err)
	}

	// Try to decompress (if data was compressed)
	// Gzip files start with magic number 0x1f 0x8b
	var jsonData []byte
	if len(decoded) > 2 && decoded[0] == 0x1f && decoded[1] == 0x8b {
		// Gzip magic number detected
		reader, err := gzip.NewReader(bytes.NewReader(decoded))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer reader.Close()

		jsonData, err = io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress guest data: %w", err)
		}
	} else {
		jsonData = decoded
	}

	// Deserialize JSON
	var guestData GuestData
	if err := json.Unmarshal(jsonData, &guestData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal guest data: %w", err)
	}

	return &guestData, nil
}

// setGuestData serializes and stores guest data in cookie.
// Automatically compresses data using gzip if it exceeds half the maximum cookie size.
// Validates that the final encoded size doesn't exceed the maximum cookie size.
// Uses base64 URL encoding for safe cookie transmission.
// Sets HttpOnly, Secure (in production), and SameSite=Lax flags for security.
func (sm *SessionManager) setGuestData(w http.ResponseWriter, guestData *GuestData) error {
	// Serialize to JSON
	jsonData, err := json.Marshal(guestData)
	if err != nil {
		return fmt.Errorf("failed to marshal guest data: %w", err)
	}

	// Check if compression is needed
	// Compress if data is larger than half the max cookie size to leave room for overhead
	var encoded string
	if len(jsonData) > sm.maxCookieSize/2 {
		// Compress data using gzip
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		if _, err := writer.Write(jsonData); err != nil {
			return fmt.Errorf("failed to compress guest data: %w", err)
		}
		if err := writer.Close(); err != nil {
			return fmt.Errorf("failed to close gzip writer: %w", err)
		}
		encoded = base64.URLEncoding.EncodeToString(buf.Bytes())
	} else {
		// No compression needed
		encoded = base64.URLEncoding.EncodeToString(jsonData)
	}

	// Validate size - ensure encoded data fits in cookie
	if len(encoded) > sm.maxCookieSize {
		return fmt.Errorf("guest data exceeds maximum cookie size (%d > %d)", len(encoded), sm.maxCookieSize)
	}

	// Set cookie with security flags
	http.SetCookie(w, &http.Cookie{
		Name:     sm.guestCookieName,
		Value:    encoded,
		Path:     sm.cookiePath,
		Domain:   sm.cookieDomain,
		MaxAge:   sm.maxAge,
		Secure:   sm.secure,
		HttpOnly: sm.httpOnly,
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}
