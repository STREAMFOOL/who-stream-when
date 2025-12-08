package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"who-live-when/internal/auth"
)

// setupTestMiddleware creates a test middleware with session manager
func setupTestMiddleware() (*AuthMiddleware, *auth.SessionManager) {
	sessionManager := auth.NewSessionManager("test-session", false, 3600)
	middleware := NewAuthMiddleware(sessionManager)
	return middleware, sessionManager
}

// createRequestWithSession creates a request with a valid session cookie
func createRequestWithSession(sessionManager *auth.SessionManager, userID string, method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	sessionManager.SetSession(w, userID)

	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}
	return req
}

func TestRequireAuth_BlocksUnauthenticatedRequests(t *testing.T) {
	middleware, _ := setupTestMiddleware()

	handlerCalled := false
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()

	middleware.RequireAuth(testHandler)(w, req)

	if handlerCalled {
		t.Error("Expected handler NOT to be called for unauthenticated request")
	}

	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303 (redirect), got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/login" {
		t.Errorf("Expected redirect to '/login', got: %s", location)
	}
}

func TestRequireAuth_AllowsAuthenticatedRequests(t *testing.T) {
	middleware, sessionManager := setupTestMiddleware()

	handlerCalled := false
	var capturedUserID string
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		capturedUserID = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}

	userID := "test-user-123"
	req := createRequestWithSession(sessionManager, userID, http.MethodGet, "/protected")
	w := httptest.NewRecorder()

	middleware.RequireAuth(testHandler)(w, req)

	if !handlerCalled {
		t.Error("Expected handler to be called for authenticated request")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if capturedUserID != userID {
		t.Errorf("Expected userID '%s' in context, got '%s'", userID, capturedUserID)
	}
}

func TestOptionalAuth_AllowsUnauthenticatedRequests(t *testing.T) {
	middleware, _ := setupTestMiddleware()

	handlerCalled := false
	var isAuth bool
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		isAuth = IsAuthenticated(r.Context())
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	w := httptest.NewRecorder()

	middleware.OptionalAuth(testHandler)(w, req)

	if !handlerCalled {
		t.Error("Expected handler to be called for unauthenticated request")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if isAuth {
		t.Error("Expected IsAuthenticated to be false for unauthenticated request")
	}
}

func TestOptionalAuth_AllowsAuthenticatedRequests(t *testing.T) {
	middleware, sessionManager := setupTestMiddleware()

	handlerCalled := false
	var capturedUserID string
	var isAuth bool
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		capturedUserID = GetUserID(r.Context())
		isAuth = IsAuthenticated(r.Context())
		w.WriteHeader(http.StatusOK)
	}

	userID := "test-user-456"
	req := createRequestWithSession(sessionManager, userID, http.MethodGet, "/public")
	w := httptest.NewRecorder()

	middleware.OptionalAuth(testHandler)(w, req)

	if !handlerCalled {
		t.Error("Expected handler to be called for authenticated request")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if capturedUserID != userID {
		t.Errorf("Expected userID '%s' in context, got '%s'", userID, capturedUserID)
	}

	if !isAuth {
		t.Error("Expected IsAuthenticated to be true for authenticated request")
	}
}

func TestRestrictUnregistered_BlocksUnauthenticatedRequests(t *testing.T) {
	middleware, _ := setupTestMiddleware()

	handlerCalled := false
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest(http.MethodPost, "/follow/123", nil)
	w := httptest.NewRecorder()

	middleware.RestrictUnregistered(testHandler)(w, req)

	if handlerCalled {
		t.Error("Expected handler NOT to be called for unauthenticated request")
	}

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestRestrictUnregistered_AllowsAuthenticatedRequests(t *testing.T) {
	middleware, sessionManager := setupTestMiddleware()

	handlerCalled := false
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}

	userID := "test-user-789"
	req := createRequestWithSession(sessionManager, userID, http.MethodPost, "/follow/123")
	w := httptest.NewRecorder()

	middleware.RestrictUnregistered(testHandler)(w, req)

	if !handlerCalled {
		t.Error("Expected handler to be called for authenticated request")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetUserID_ReturnsEmptyForMissingContext(t *testing.T) {
	ctx := context.Background()
	userID := GetUserID(ctx)

	if userID != "" {
		t.Errorf("Expected empty userID, got '%s'", userID)
	}
}

func TestIsAuthenticated_ReturnsFalseForMissingContext(t *testing.T) {
	ctx := context.Background()
	isAuth := IsAuthenticated(ctx)

	if isAuth {
		t.Error("Expected IsAuthenticated to be false for missing context")
	}
}

func TestRequireAuth_SetsIsAuthenticatedInContext(t *testing.T) {
	middleware, sessionManager := setupTestMiddleware()

	var isAuth bool
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		isAuth = IsAuthenticated(r.Context())
		w.WriteHeader(http.StatusOK)
	}

	userID := "test-user"
	req := createRequestWithSession(sessionManager, userID, http.MethodGet, "/protected")
	w := httptest.NewRecorder()

	middleware.RequireAuth(testHandler)(w, req)

	if !isAuth {
		t.Error("Expected IsAuthenticated to be true after RequireAuth")
	}
}
