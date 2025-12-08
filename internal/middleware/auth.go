package middleware

import (
	"context"
	"net/http"

	"who-live-when/internal/auth"
)

// ContextKey is a custom type for context keys to avoid collisions
type ContextKey string

const (
	// UserIDKey is the context key for the user ID
	UserIDKey ContextKey = "userID"
	// IsAuthenticatedKey is the context key for authentication status
	IsAuthenticatedKey ContextKey = "isAuthenticated"
)

// AuthMiddleware provides authentication middleware functions
type AuthMiddleware struct {
	sessionManager *auth.SessionManager
}

// NewAuthMiddleware creates a new AuthMiddleware instance
func NewAuthMiddleware(sessionManager *auth.SessionManager) *AuthMiddleware {
	return &AuthMiddleware{
		sessionManager: sessionManager,
	}
}

// RequireAuth is middleware that ensures the user is authenticated.
// If not authenticated, redirects to login page.
// Validates: Requirements 5.2, 5.4 - restricts unregistered users from search and follow operations
func (m *AuthMiddleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := m.sessionManager.GetSession(r)
		if err != nil || userID == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, IsAuthenticatedKey, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// OptionalAuth is middleware that checks for authentication but allows both
// authenticated and unauthenticated requests to proceed.
// Validates: Requirements 5.1 - default week view accessible to both authenticated and unregistered users
func (m *AuthMiddleware) OptionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := m.sessionManager.GetSession(r)
		isAuthenticated := err == nil && userID != ""

		ctx := r.Context()
		if isAuthenticated {
			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, IsAuthenticatedKey, true)
		} else {
			ctx = context.WithValue(ctx, UserIDKey, "")
			ctx = context.WithValue(ctx, IsAuthenticatedKey, false)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// GetUserID retrieves the user ID from the request context
func GetUserID(ctx context.Context) string {
	userID, ok := ctx.Value(UserIDKey).(string)
	if !ok {
		return ""
	}
	return userID
}

// IsAuthenticated checks if the request is authenticated
func IsAuthenticated(ctx context.Context) bool {
	isAuth, ok := ctx.Value(IsAuthenticatedKey).(bool)
	if !ok {
		return false
	}
	return isAuth
}

// RestrictUnregistered is middleware that blocks specific operations for unregistered users.
// Returns 401 Unauthorized for unauthenticated requests instead of redirecting.
// Validates: Requirements 5.3 - unregistered users cannot add new streamers
func (m *AuthMiddleware) RestrictUnregistered(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := m.sessionManager.GetSession(r)
		if err != nil || userID == "" {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, IsAuthenticatedKey, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
