package service

import (
	"context"
	"testing"
	"time"

	"who-live-when/internal/domain"
)

// mockTVProgrammeService is a mock implementation for testing
type mockTVProgrammeService struct {
	generateProgrammeFunc func(ctx context.Context, userID string, week time.Time) (*domain.TVProgramme, error)
}

func (m *mockTVProgrammeService) GenerateProgramme(ctx context.Context, userID string, week time.Time) (*domain.TVProgramme, error) {
	if m.generateProgrammeFunc != nil {
		return m.generateProgrammeFunc(ctx, userID, week)
	}
	return &domain.TVProgramme{}, nil
}

func (m *mockTVProgrammeService) GetPredictedLiveTime(ctx context.Context, streamerID string, dayOfWeek int) (*domain.PredictedTime, error) {
	return nil, nil
}

func (m *mockTVProgrammeService) GetMostViewedStreamers(ctx context.Context, limit int) ([]*domain.Streamer, error) {
	return nil, nil
}

func (m *mockTVProgrammeService) GetDefaultWeekView(ctx context.Context) (*domain.WeekView, error) {
	return nil, nil
}

// mockUserService is a mock implementation for testing
type mockUserService struct {
	getUserFollowsFunc func(ctx context.Context, userID string) ([]*domain.Streamer, error)
}

func (m *mockUserService) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	return nil, nil
}

func (m *mockUserService) CreateUser(ctx context.Context, googleID string, email string) (*domain.User, error) {
	return nil, nil
}

func (m *mockUserService) GetUserFollows(ctx context.Context, userID string) ([]*domain.Streamer, error) {
	if m.getUserFollowsFunc != nil {
		return m.getUserFollowsFunc(ctx, userID)
	}
	return []*domain.Streamer{}, nil
}

func (m *mockUserService) FollowStreamer(ctx context.Context, userID, streamerID string) error {
	return nil
}

func (m *mockUserService) UnfollowStreamer(ctx context.Context, userID, streamerID string) error {
	return nil
}

func (m *mockUserService) GetStreamersByIDs(ctx context.Context, streamerIDs []string) ([]*domain.Streamer, error) {
	return []*domain.Streamer{}, nil
}

func (m *mockUserService) MigrateGuestData(ctx context.Context, userID string, guestFollows []string, guestProgramme *domain.CustomProgramme) error {
	return nil
}

func TestCalendarService_GetCalendarView(t *testing.T) {
	ctx := context.Background()
	userID := "user-1"
	week := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	streamer1 := &domain.Streamer{
		ID:   "streamer-1",
		Name: "Streamer One",
	}

	streamer2 := &domain.Streamer{
		ID:   "streamer-2",
		Name: "Streamer Two",
	}

	programme := &domain.TVProgramme{
		UserID: userID,
		Week:   week,
		Entries: []domain.ProgrammeEntry{
			{StreamerID: "streamer-1", DayOfWeek: 0, Hour: 10, Probability: 0.8},
			{StreamerID: "streamer-1", DayOfWeek: 1, Hour: 14, Probability: 0.7},
			{StreamerID: "streamer-2", DayOfWeek: 2, Hour: 18, Probability: 0.6},
		},
	}

	mockTVProgramme := &mockTVProgrammeService{
		generateProgrammeFunc: func(ctx context.Context, userID string, week time.Time) (*domain.TVProgramme, error) {
			return programme, nil
		},
	}

	mockUser := &mockUserService{
		getUserFollowsFunc: func(ctx context.Context, userID string) ([]*domain.Streamer, error) {
			return []*domain.Streamer{streamer1, streamer2}, nil
		},
	}

	service := NewCalendarService(mockTVProgramme, mockUser)

	view, err := service.GetCalendarView(ctx, userID, week)
	if err != nil {
		t.Fatalf("GetCalendarView failed: %v", err)
	}

	if view == nil {
		t.Fatal("Expected non-nil calendar view")
	}

	if view.Programme == nil {
		t.Fatal("Expected non-nil programme")
	}

	if len(view.StreamerMap) != 2 {
		t.Errorf("Expected 2 streamers in map, got %d", len(view.StreamerMap))
	}

	if view.StreamerMap["streamer-1"].Name != "Streamer One" {
		t.Errorf("Expected streamer name 'Streamer One', got '%s'", view.StreamerMap["streamer-1"].Name)
	}

	// Check week normalization
	expectedWeekStart := time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC) // Sunday
	if !view.Week.Equal(expectedWeekStart) {
		t.Errorf("Expected week start %v, got %v", expectedWeekStart, view.Week)
	}

	// Check navigation dates
	expectedPrevWeek := expectedWeekStart.AddDate(0, 0, -7)
	if !view.PrevWeek.Equal(expectedPrevWeek) {
		t.Errorf("Expected prev week %v, got %v", expectedPrevWeek, view.PrevWeek)
	}

	expectedNextWeek := expectedWeekStart.AddDate(0, 0, 7)
	if !view.NextWeek.Equal(expectedNextWeek) {
		t.Errorf("Expected next week %v, got %v", expectedNextWeek, view.NextWeek)
	}
}

func TestCalendarService_BuildTimeSlotGrid(t *testing.T) {
	service := &CalendarService{}

	streamer1 := &domain.Streamer{
		ID:   "streamer-1",
		Name: "Streamer One",
	}

	streamerMap := map[string]*domain.Streamer{
		"streamer-1": streamer1,
	}

	programme := &domain.TVProgramme{
		Entries: []domain.ProgrammeEntry{
			{StreamerID: "streamer-1", DayOfWeek: 0, Hour: 10, Probability: 0.8},
			{StreamerID: "streamer-1", DayOfWeek: 1, Hour: 14, Probability: 0.7},
			{StreamerID: "streamer-1", DayOfWeek: 0, Hour: 10, Probability: 0.6}, // Same slot
		},
	}

	grid := service.buildTimeSlotGrid(programme, streamerMap)

	if len(grid) != 24 {
		t.Errorf("Expected 24 hours in grid, got %d", len(grid))
	}

	// Check that entries are in correct slots
	if len(grid[10][0]) != 2 {
		t.Errorf("Expected 2 entries at hour 10, day 0, got %d", len(grid[10][0]))
	}

	if len(grid[14][1]) != 1 {
		t.Errorf("Expected 1 entry at hour 14, day 1, got %d", len(grid[14][1]))
	}

	// Check entry details
	entry := grid[10][0][0]
	if entry.StreamerID != "streamer-1" {
		t.Errorf("Expected streamer ID 'streamer-1', got '%s'", entry.StreamerID)
	}

	if entry.StreamerName != "Streamer One" {
		t.Errorf("Expected streamer name 'Streamer One', got '%s'", entry.StreamerName)
	}

	if entry.Probability != 0.8 {
		t.Errorf("Expected probability 0.8, got %f", entry.Probability)
	}
}

func TestCalendarService_NavigateWeek(t *testing.T) {
	service := &CalendarService{}

	currentWeek := time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC)
	expectedWeekStart := time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC) // Sunday

	tests := []struct {
		name      string
		direction string
		expected  time.Time
	}{
		{
			name:      "Navigate to previous week",
			direction: "prev",
			expected:  expectedWeekStart.AddDate(0, 0, -7),
		},
		{
			name:      "Navigate to next week",
			direction: "next",
			expected:  expectedWeekStart.AddDate(0, 0, 7),
		},
		{
			name:      "Invalid direction returns current week",
			direction: "invalid",
			expected:  expectedWeekStart,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.NavigateWeek(currentWeek, tt.direction)
			if !result.Equal(tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNormalizeWeekStart(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "Monday normalizes to previous Sunday",
			input:    time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC), // Monday
			expected: time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC),    // Sunday
		},
		{
			name:     "Sunday stays Sunday",
			input:    time.Date(2024, 1, 14, 12, 30, 45, 0, time.UTC), // Sunday
			expected: time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC),    // Sunday
		},
		{
			name:     "Saturday normalizes to previous Sunday",
			input:    time.Date(2024, 1, 20, 12, 30, 45, 0, time.UTC), // Saturday
			expected: time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC),    // Sunday
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeWeekStart(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
