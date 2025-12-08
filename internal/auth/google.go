package auth

import (
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

// SessionManager manages user sessions with secure cookies
type SessionManager struct {
	cookieName   string
	cookiePath   string
	cookieDomain string
	secure       bool
	httpOnly     bool
	maxAge       int // in seconds
}

// NewSessionManager creates a new session manager
func NewSessionManager(cookieName string, secure bool, maxAge int) *SessionManager {
	return &SessionManager{
		cookieName: cookieName,
		cookiePath: "/",
		secure:     secure,
		httpOnly:   true,
		maxAge:     maxAge,
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
