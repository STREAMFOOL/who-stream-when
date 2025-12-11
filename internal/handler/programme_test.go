package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"who-live-when/internal/auth"
	"who-live-when/internal/domain"
	"who-live-when/internal/service"
)

// Mock programme service for testing
type mockProgrammeService struct {
	programmes map[string]*domain.CustomProgramme
	createErr  error
	getErr     error
	updateErr  error
	deleteErr  error
}

func newMockProgrammeService() *mockProgrammeService {
	return &mockProgrammeService{
		programmes: make(map[string]*domain.CustomProgramme),
	}
}

func (m *mockProgrammeService) CreateCustomProgramme(ctx context.Context, userID string, streamerIDs []string) (*domain.CustomProgramme, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	programme := &domain.CustomProgramme{
		ID:          "prog-1",
		UserID:      userID,
		StreamerIDs: streamerIDs,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.programmes[userID] = programme
	return programme, nil
}

func (m *mockProgrammeService) GetCustomProgramme(ctx context.Context, userID string) (*domain.CustomProgramme, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	prog, exists := m.programmes[userID]
	if !exists {
		return nil, service.ErrProgrammeNotFound
	}
	return prog, nil
}

func (m *mockProgrammeService) UpdateCustomProgramme(ctx context.Context, userID string, streamerIDs []string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	prog, exists := m.programmes[userID]
	if !exists {
		// Create if doesn't exist (for tests)
		prog = &domain.CustomProgramme{
			ID:          "prog-1",
			UserID:      userID,
			StreamerIDs: []string{},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		m.programmes[userID] = prog
	}
	prog.StreamerIDs = streamerIDs
	prog.UpdatedAt = time.Now()
	return nil
}

func (m *mockProgrammeService) DeleteCustomProgramme(ctx context.Context, userID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.programmes, userID)
	return nil
}

func (m *mockProgrammeService) AddStreamerToProgramme(ctx context.Context, userID, streamerID string) error {
	prog, exists := m.programmes[userID]
	if !exists {
		return service.ErrProgrammeNotFound
	}
	prog.StreamerIDs = append(prog.StreamerIDs, streamerID)
	return nil
}

func (m *mockProgrammeService) RemoveStreamerFromProgramme(ctx context.Context, userID, streamerID string) error {
	prog, exists := m.programmes[userID]
	if !exists {
		return service.ErrProgrammeNotFound
	}
	var newIDs []string
	for _, id := range prog.StreamerIDs {
		if id != streamerID {
			newIDs = append(newIDs, id)
		}
	}
	prog.StreamerIDs = newIDs
	return nil
}

func (m *mockProgrammeService) GetProgrammeView(ctx context.Context, userID string, week time.Time) (*service.ProgrammeCalendarView, error) {
	prog, exists := m.programmes[userID]
	isCustom := exists && prog != nil && len(prog.StreamerIDs) > 0
	return &service.ProgrammeCalendarView{
		Week:           week,
		Streamers:      []*domain.Streamer{},
		Entries:        []domain.ProgrammeEntry{},
		IsCustom:       isCustom,
		IsGuestSession: false,
	}, nil
}

// Mock streamer service for testing
type mockStreamerService struct {
	streamers map[string]*domain.Streamer
}

func newMockStreamerService() *mockStreamerService {
	return &mockStreamerService{
		streamers: make(map[string]*domain.Streamer),
	}
}

func (m *mockStreamerService) GetStreamer(ctx context.Context, id string) (*domain.Streamer, error) {
	streamer, exists := m.streamers[id]
	if !exists {
		return nil, service.ErrStreamerNotFound
	}
	return streamer, nil
}

func (m *mockStreamerService) ListStreamers(ctx context.Context, limit int) ([]*domain.Streamer, error) {
	var streamers []*domain.Streamer
	for _, s := range m.streamers {
		streamers = append(streamers, s)
		if len(streamers) >= limit {
			break
		}
	}
	return streamers, nil
}

func TestProgrammeHandler_HandleProgrammeManagement_Authenticated(t *testing.T) {
	programmeService := newMockProgrammeService()
	streamerService := newMockStreamerService()
	sessionManager := auth.NewSessionManager("test-session", false, 3600)

	// Add test streamers
	streamerService.streamers["streamer-1"] = &domain.Streamer{
		ID:   "streamer-1",
		Name: "Test Streamer 1",
	}
	streamerService.streamers["streamer-2"] = &domain.Streamer{
		ID:   "streamer-2",
		Name: "Test Streamer 2",
	}

	handler := NewProgrammeHandler(programmeService, streamerService, sessionManager)

	// Create a request with authenticated user
	req := httptest.NewRequest(http.MethodGet, "/programme", nil)
	ctx := context.WithValue(req.Context(), userIDKey, "user-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.HandleProgrammeManagement(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Programme Management") {
		t.Error("Expected page to contain 'Programme Management'")
	}
}

func TestProgrammeHandler_HandleCreateProgramme(t *testing.T) {
	programmeService := newMockProgrammeService()
	streamerService := newMockStreamerService()
	sessionManager := auth.NewSessionManager("test-session", false, 3600)

	handler := NewProgrammeHandler(programmeService, streamerService, sessionManager)

	// Create form data
	form := url.Values{}
	form.Add("streamer_ids", "streamer-1,streamer-2")

	req := httptest.NewRequest(http.MethodPost, "/programme/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()

	// Set session cookie to simulate authenticated user
	sessionManager.SetSession(w, "user-1")
	// Copy the cookie to the request
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	w = httptest.NewRecorder()
	handler.HandleCreateProgramme(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303, got %d", w.Code)
	}

	// Verify programme was created
	prog, err := programmeService.GetCustomProgramme(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("Expected programme to be created: %v", err)
	}

	if len(prog.StreamerIDs) != 2 {
		t.Errorf("Expected 2 streamers, got %d", len(prog.StreamerIDs))
	}
}

func TestProgrammeHandler_HandleUpdateProgramme(t *testing.T) {
	programmeService := newMockProgrammeService()
	streamerService := newMockStreamerService()
	sessionManager := auth.NewSessionManager("test-session", false, 3600)

	handler := NewProgrammeHandler(programmeService, streamerService, sessionManager)

	// Create initial programme
	_, err := programmeService.CreateCustomProgramme(context.Background(), "user-1", []string{"streamer-1"})
	if err != nil {
		t.Fatalf("Failed to create programme: %v", err)
	}

	// Update with new streamers
	form := url.Values{}
	form.Add("streamer_ids", "streamer-2,streamer-3")

	req := httptest.NewRequest(http.MethodPost, "/programme/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	sessionManager.SetSession(w, "user-1")
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	w = httptest.NewRecorder()
	handler.HandleUpdateProgramme(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303, got %d", w.Code)
	}

	// Verify programme was updated
	prog, err := programmeService.GetCustomProgramme(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("Expected programme to exist: %v", err)
	}

	if len(prog.StreamerIDs) != 2 {
		t.Errorf("Expected 2 streamers after update, got %d", len(prog.StreamerIDs))
	}
}

func TestProgrammeHandler_HandleDeleteProgramme(t *testing.T) {
	programmeService := newMockProgrammeService()
	streamerService := newMockStreamerService()
	sessionManager := auth.NewSessionManager("test-session", false, 3600)

	handler := NewProgrammeHandler(programmeService, streamerService, sessionManager)

	// Create initial programme
	_, err := programmeService.CreateCustomProgramme(context.Background(), "user-1", []string{"streamer-1"})
	if err != nil {
		t.Fatalf("Failed to create programme: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/programme/delete", nil)

	w := httptest.NewRecorder()
	sessionManager.SetSession(w, "user-1")
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	w = httptest.NewRecorder()
	handler.HandleDeleteProgramme(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303, got %d", w.Code)
	}

	// Verify programme was deleted
	_, err = programmeService.GetCustomProgramme(context.Background(), "user-1")
	if err == nil {
		t.Error("Expected programme to be deleted")
	}
}

func TestProgrammeHandler_HandleAddStreamer(t *testing.T) {
	programmeService := newMockProgrammeService()
	streamerService := newMockStreamerService()
	sessionManager := auth.NewSessionManager("test-session", false, 3600)

	handler := NewProgrammeHandler(programmeService, streamerService, sessionManager)

	// Create initial programme
	_, err := programmeService.CreateCustomProgramme(context.Background(), "user-1", []string{"streamer-1"})
	if err != nil {
		t.Fatalf("Failed to create programme: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/programme/add/streamer-2", nil)

	w := httptest.NewRecorder()
	sessionManager.SetSession(w, "user-1")
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	w = httptest.NewRecorder()
	handler.HandleAddStreamer(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303, got %d", w.Code)
	}

	// Verify streamer was added
	prog, err := programmeService.GetCustomProgramme(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("Expected programme to exist: %v", err)
	}

	if len(prog.StreamerIDs) != 2 {
		t.Errorf("Expected 2 streamers after add, got %d", len(prog.StreamerIDs))
	}
}

func TestProgrammeHandler_HandleRemoveStreamer(t *testing.T) {
	programmeService := newMockProgrammeService()
	streamerService := newMockStreamerService()
	sessionManager := auth.NewSessionManager("test-session", false, 3600)

	handler := NewProgrammeHandler(programmeService, streamerService, sessionManager)

	// Create initial programme with 2 streamers
	_, err := programmeService.CreateCustomProgramme(context.Background(), "user-1", []string{"streamer-1", "streamer-2"})
	if err != nil {
		t.Fatalf("Failed to create programme: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/programme/remove/streamer-1", nil)

	w := httptest.NewRecorder()
	sessionManager.SetSession(w, "user-1")
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	w = httptest.NewRecorder()
	handler.HandleRemoveStreamer(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303, got %d", w.Code)
	}

	// Verify streamer was removed
	prog, err := programmeService.GetCustomProgramme(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("Expected programme to exist: %v", err)
	}

	if len(prog.StreamerIDs) != 1 {
		t.Errorf("Expected 1 streamer after remove, got %d", len(prog.StreamerIDs))
	}

	if prog.StreamerIDs[0] != "streamer-2" {
		t.Errorf("Expected streamer-2 to remain, got %s", prog.StreamerIDs[0])
	}
}

func TestProgrammeHandler_UIStateForCustomProgramme(t *testing.T) {
	programmeService := newMockProgrammeService()
	streamerService := newMockStreamerService()
	sessionManager := auth.NewSessionManager("test-session", false, 3600)

	handler := NewProgrammeHandler(programmeService, streamerService, sessionManager)

	// Create custom programme
	_, err := programmeService.CreateCustomProgramme(context.Background(), "user-1", []string{"streamer-1"})
	if err != nil {
		t.Fatalf("Failed to create programme: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/programme", nil)

	w := httptest.NewRecorder()
	sessionManager.SetSession(w, "user-1")
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}

	w = httptest.NewRecorder()
	handler.HandleProgrammeManagement(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Custom Programme") && !strings.Contains(body, "custom programme") {
		t.Errorf("Expected UI to indicate custom programme, got: %s", body)
	}
}

func TestProgrammeHandler_UIStateForGlobalProgramme(t *testing.T) {
	programmeService := newMockProgrammeService()
	streamerService := newMockStreamerService()
	sessionManager := auth.NewSessionManager("test-session", false, 3600)

	handler := NewProgrammeHandler(programmeService, streamerService, sessionManager)

	req := httptest.NewRequest(http.MethodGet, "/programme", nil)
	ctx := context.WithValue(req.Context(), userIDKey, "user-1")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.HandleProgrammeManagement(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Global Programme") {
		t.Error("Expected UI to indicate global programme")
	}
}

func TestProgrammeHandler_GuestUserNotice(t *testing.T) {
	programmeService := newMockProgrammeService()
	streamerService := newMockStreamerService()
	sessionManager := auth.NewSessionManager("test-session", false, 3600)

	handler := NewProgrammeHandler(programmeService, streamerService, sessionManager)

	// Create request without authentication (guest user)
	req := httptest.NewRequest(http.MethodGet, "/programme", nil)

	w := httptest.NewRecorder()
	handler.HandleProgrammeManagement(w, req)

	body := w.Body.String()
	hasSessionNotice := strings.Contains(body, "session") || strings.Contains(body, "Session")
	hasGuestNotice := strings.Contains(body, "guest") || strings.Contains(body, "Guest")

	if !hasSessionNotice || !hasGuestNotice {
		t.Errorf("Expected UI to display guest user notice about session-based storage, got: %s", body)
	}
}
