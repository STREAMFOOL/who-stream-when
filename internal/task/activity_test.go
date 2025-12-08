package task

import (
	"context"
	"os"
	"testing"
	"time"

	"who-live-when/internal/domain"
	"who-live-when/internal/repository/sqlite"
	"who-live-when/internal/service"

	"github.com/google/uuid"
)

// setupTestDB creates a test database for activity tracker tests
func setupTestDB(t *testing.T) *sqlite.DB {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "test-activity-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()

	db, err := sqlite.NewDB(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to open database: %v", err)
	}

	if err := sqlite.Migrate(db.DB); err != nil {
		db.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to run migrations: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
		os.Remove(tmpFile.Name())
	})

	return db
}

// mockLiveStatusService implements domain.LiveStatusService for testing
type mockLiveStatusService struct {
	statuses map[string]*domain.LiveStatus
}

func newMockLiveStatusService() *mockLiveStatusService {
	return &mockLiveStatusService{
		statuses: make(map[string]*domain.LiveStatus),
	}
}

func (m *mockLiveStatusService) GetLiveStatus(ctx context.Context, streamerID string) (*domain.LiveStatus, error) {
	if status, ok := m.statuses[streamerID]; ok {
		return status, nil
	}
	return &domain.LiveStatus{StreamerID: streamerID, IsLive: false}, nil
}

func (m *mockLiveStatusService) RefreshLiveStatus(ctx context.Context, streamerID string) (*domain.LiveStatus, error) {
	return m.GetLiveStatus(ctx, streamerID)
}

func (m *mockLiveStatusService) GetAllLiveStatus(ctx context.Context) (map[string]*domain.LiveStatus, error) {
	return m.statuses, nil
}

func (m *mockLiveStatusService) SetLiveStatus(streamerID string, isLive bool, platform string) {
	m.statuses[streamerID] = &domain.LiveStatus{
		StreamerID: streamerID,
		IsLive:     isLive,
		Platform:   platform,
		UpdatedAt:  time.Now(),
	}
}

// TestActivityTracker_RecordsActivityWhenStreamerGoesLive tests that activity records
// are created when a streamer transitions from offline to live
func TestActivityTracker_RecordsActivityWhenStreamerGoesLive(t *testing.T) {
	db := setupTestDB(t)
	streamerRepo := sqlite.NewStreamerRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	mockLiveStatus := newMockLiveStatusService()
	ctx := context.Background()

	streamerID := uuid.New().String()
	streamer := &domain.Streamer{
		ID:        streamerID,
		Name:      "Test Streamer",
		Handles:   map[string]string{"youtube": "testhandle"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := streamerRepo.Create(ctx, streamer); err != nil {
		t.Fatalf("failed to create streamer: %v", err)
	}

	tracker := NewActivityTracker(streamerRepo, activityRepo, mockLiveStatus, time.Hour)

	// Initially offline
	mockLiveStatus.SetLiveStatus(streamerID, false, "")
	tracker.checkAndRecordActivity(ctx)

	// Verify no activity recorded yet
	oneYearAgo := time.Now().AddDate(-1, 0, 0)
	records, err := activityRepo.GetByStreamerID(ctx, streamerID, oneYearAgo)
	if err != nil {
		t.Fatalf("failed to get activity records: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("Expected 0 activity records when offline, got %d", len(records))
	}

	// Streamer goes live
	mockLiveStatus.SetLiveStatus(streamerID, true, "youtube")
	tracker.checkAndRecordActivity(ctx)

	// Verify activity was recorded
	records, err = activityRepo.GetByStreamerID(ctx, streamerID, oneYearAgo)
	if err != nil {
		t.Fatalf("failed to get activity records: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("Expected 1 activity record when going live, got %d", len(records))
	}

	// Verify record has correct platform
	if len(records) > 0 && records[0].Platform != "youtube" {
		t.Errorf("Expected platform 'youtube', got '%s'", records[0].Platform)
	}
}

// TestActivityTracker_NoRecordWhenAlreadyLive tests that no duplicate activity records
// are created when a streamer remains live
func TestActivityTracker_NoRecordWhenAlreadyLive(t *testing.T) {
	db := setupTestDB(t)
	streamerRepo := sqlite.NewStreamerRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	mockLiveStatus := newMockLiveStatusService()
	ctx := context.Background()

	streamerID := uuid.New().String()
	streamer := &domain.Streamer{
		ID:        streamerID,
		Name:      "Test Streamer",
		Handles:   map[string]string{"twitch": "testhandle"},
		Platforms: []string{"twitch"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := streamerRepo.Create(ctx, streamer); err != nil {
		t.Fatalf("failed to create streamer: %v", err)
	}

	tracker := NewActivityTracker(streamerRepo, activityRepo, mockLiveStatus, time.Hour)

	// Streamer goes live
	mockLiveStatus.SetLiveStatus(streamerID, true, "twitch")
	tracker.checkAndRecordActivity(ctx)

	// Check again while still live
	tracker.checkAndRecordActivity(ctx)
	tracker.checkAndRecordActivity(ctx)

	// Verify only one activity record was created
	oneYearAgo := time.Now().AddDate(-1, 0, 0)
	records, err := activityRepo.GetByStreamerID(ctx, streamerID, oneYearAgo)
	if err != nil {
		t.Fatalf("failed to get activity records: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("Expected 1 activity record (no duplicates), got %d", len(records))
	}
}

// TestActivityTracker_RecordsNewActivityAfterGoingOffline tests that a new activity
// record is created when a streamer goes live again after being offline
func TestActivityTracker_RecordsNewActivityAfterGoingOffline(t *testing.T) {
	db := setupTestDB(t)
	streamerRepo := sqlite.NewStreamerRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	mockLiveStatus := newMockLiveStatusService()
	ctx := context.Background()

	streamerID := uuid.New().String()
	streamer := &domain.Streamer{
		ID:        streamerID,
		Name:      "Test Streamer",
		Handles:   map[string]string{"kick": "testhandle"},
		Platforms: []string{"kick"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := streamerRepo.Create(ctx, streamer); err != nil {
		t.Fatalf("failed to create streamer: %v", err)
	}

	tracker := NewActivityTracker(streamerRepo, activityRepo, mockLiveStatus, time.Hour)

	// First live session
	mockLiveStatus.SetLiveStatus(streamerID, true, "kick")
	tracker.checkAndRecordActivity(ctx)

	// Goes offline
	mockLiveStatus.SetLiveStatus(streamerID, false, "")
	tracker.checkAndRecordActivity(ctx)

	// Goes live again
	mockLiveStatus.SetLiveStatus(streamerID, true, "kick")
	tracker.checkAndRecordActivity(ctx)

	// Verify two activity records were created
	oneYearAgo := time.Now().AddDate(-1, 0, 0)
	records, err := activityRepo.GetByStreamerID(ctx, streamerID, oneYearAgo)
	if err != nil {
		t.Fatalf("failed to get activity records: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("Expected 2 activity records (two live sessions), got %d", len(records))
	}
}

// TestActivityTracker_ActivityDataUsedForHeatmap tests that activity data recorded
// by the tracker is correctly used for heatmap generation
func TestActivityTracker_ActivityDataUsedForHeatmap(t *testing.T) {
	db := setupTestDB(t)
	streamerRepo := sqlite.NewStreamerRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)
	mockLiveStatus := newMockLiveStatusService()
	ctx := context.Background()

	streamerID := uuid.New().String()
	streamer := &domain.Streamer{
		ID:        streamerID,
		Name:      "Test Streamer",
		Handles:   map[string]string{"youtube": "testhandle"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := streamerRepo.Create(ctx, streamer); err != nil {
		t.Fatalf("failed to create streamer: %v", err)
	}

	tracker := NewActivityTracker(streamerRepo, activityRepo, mockLiveStatus, time.Hour)
	heatmapSvc := service.NewHeatmapService(activityRepo, heatmapRepo)

	// Simulate multiple live sessions
	for i := 0; i < 5; i++ {
		mockLiveStatus.SetLiveStatus(streamerID, true, "youtube")
		tracker.checkAndRecordActivity(ctx)

		mockLiveStatus.SetLiveStatus(streamerID, false, "")
		tracker.checkAndRecordActivity(ctx)
	}

	// Verify activity records were created
	oneYearAgo := time.Now().AddDate(-1, 0, 0)
	records, err := activityRepo.GetByStreamerID(ctx, streamerID, oneYearAgo)
	if err != nil {
		t.Fatalf("failed to get activity records: %v", err)
	}
	if len(records) != 5 {
		t.Errorf("Expected 5 activity records, got %d", len(records))
	}

	// Generate heatmap using the recorded activity data
	heatmap, err := heatmapSvc.GenerateHeatmap(ctx, streamerID)
	if err != nil {
		t.Fatalf("failed to generate heatmap: %v", err)
	}

	// Verify heatmap was generated with correct data points
	if heatmap.DataPoints != 5 {
		t.Errorf("Expected heatmap to have 5 data points, got %d", heatmap.DataPoints)
	}

	// Verify the current hour has activity (all records were created at current time)
	currentHour := time.Now().Hour()
	if heatmap.Hours[currentHour] <= 0 {
		t.Errorf("Expected current hour %d to have positive probability, got %f", currentHour, heatmap.Hours[currentHour])
	}
}

// TestActivityTracker_MultipleStreamers tests tracking multiple streamers simultaneously
func TestActivityTracker_MultipleStreamers(t *testing.T) {
	db := setupTestDB(t)
	streamerRepo := sqlite.NewStreamerRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	mockLiveStatus := newMockLiveStatusService()
	ctx := context.Background()

	// Create multiple streamers
	streamerIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		streamerID := uuid.New().String()
		streamerIDs[i] = streamerID
		streamer := &domain.Streamer{
			ID:        streamerID,
			Name:      "Test Streamer " + streamerID[:8],
			Handles:   map[string]string{"youtube": "handle" + streamerID[:8]},
			Platforms: []string{"youtube"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := streamerRepo.Create(ctx, streamer); err != nil {
			t.Fatalf("failed to create streamer: %v", err)
		}
	}

	tracker := NewActivityTracker(streamerRepo, activityRepo, mockLiveStatus, time.Hour)

	// First streamer goes live
	mockLiveStatus.SetLiveStatus(streamerIDs[0], true, "youtube")
	// Second streamer stays offline
	mockLiveStatus.SetLiveStatus(streamerIDs[1], false, "")
	// Third streamer goes live
	mockLiveStatus.SetLiveStatus(streamerIDs[2], true, "youtube")

	tracker.checkAndRecordActivity(ctx)

	// Verify activity records
	oneYearAgo := time.Now().AddDate(-1, 0, 0)

	// First streamer should have 1 record
	records, _ := activityRepo.GetByStreamerID(ctx, streamerIDs[0], oneYearAgo)
	if len(records) != 1 {
		t.Errorf("Streamer 0: expected 1 record, got %d", len(records))
	}

	// Second streamer should have 0 records
	records, _ = activityRepo.GetByStreamerID(ctx, streamerIDs[1], oneYearAgo)
	if len(records) != 0 {
		t.Errorf("Streamer 1: expected 0 records, got %d", len(records))
	}

	// Third streamer should have 1 record
	records, _ = activityRepo.GetByStreamerID(ctx, streamerIDs[2], oneYearAgo)
	if len(records) != 1 {
		t.Errorf("Streamer 2: expected 1 record, got %d", len(records))
	}
}

// TestActivityTracker_GetLastLiveStatus tests the helper methods for testing
func TestActivityTracker_GetLastLiveStatus(t *testing.T) {
	db := setupTestDB(t)
	streamerRepo := sqlite.NewStreamerRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	mockLiveStatus := newMockLiveStatusService()

	tracker := NewActivityTracker(streamerRepo, activityRepo, mockLiveStatus, time.Hour)

	streamerID := "test-streamer-123"

	// Initially should be false (not tracked)
	if tracker.GetLastLiveStatus(streamerID) {
		t.Error("Expected initial status to be false")
	}

	// Set status
	tracker.SetLastLiveStatus(streamerID, true)
	if !tracker.GetLastLiveStatus(streamerID) {
		t.Error("Expected status to be true after setting")
	}

	// Set back to false
	tracker.SetLastLiveStatus(streamerID, false)
	if tracker.GetLastLiveStatus(streamerID) {
		t.Error("Expected status to be false after resetting")
	}
}
