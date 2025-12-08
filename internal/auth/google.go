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

// GuestData represents session data for unauthenticated users
type GuestData struct {
	FollowedStreamerIDs []string             `json:"follows"`
	CustomProgramme     *CustomProgrammeData `json:"programme,omitempty"`
	CreatedAt           time.Time            `json:"created_at"`
}

// CustomProgrammeData represents a custom programme stored in session
type CustomProgrammeData struct {
	StreamerIDs []string  `json:"streamer_ids"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SessionManager manages user sessions with secure cookies
type SessionManager struct {
	cookieName      string
	guestCookieName string
	cookiePath      string
	cookieDomain    string
	secure          bool
	httpOnly        bool
	maxAge          int // in seconds
	maxCookieSize   int // maximum cookie size in bytes
}

// NewSessionManager creates a new session manager
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

// GetGuestFollows retrieves the list of followed streamer IDs from guest session
func (sm *SessionManager) GetGuestFollows(r *http.Request) ([]string, error) {
	guestData, err := sm.getGuestData(r)
	if err != nil {
		return []string{}, nil // Return empty slice if no guest data
	}
	return guestData.FollowedStreamerIDs, nil
}

// SetGuestFollows stores the list of followed streamer IDs in guest session
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

// GetGuestProgramme retrieves the custom programme from guest session
func (sm *SessionManager) GetGuestProgramme(r *http.Request) (*CustomProgrammeData, error) {
	guestData, err := sm.getGuestData(r)
	if err != nil {
		return nil, err
	}
	return guestData.CustomProgramme, nil
}

// SetGuestProgramme stores the custom programme in guest session
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

// ClearGuestData removes all guest session data
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

// getGuestData retrieves and deserializes guest data from cookie
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

// setGuestData serializes and stores guest data in cookie
func (sm *SessionManager) setGuestData(w http.ResponseWriter, guestData *GuestData) error {
	// Serialize to JSON
	jsonData, err := json.Marshal(guestData)
	if err != nil {
		return fmt.Errorf("failed to marshal guest data: %w", err)
	}

	// Check if compression is needed
	var encoded string
	if len(jsonData) > sm.maxCookieSize/2 {
		// Compress data
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

	// Validate size
	if len(encoded) > sm.maxCookieSize {
		return fmt.Errorf("guest data exceeds maximum cookie size (%d > %d)", len(encoded), sm.maxCookieSize)
	}

	// Set cookie
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
