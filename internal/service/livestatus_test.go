package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"who-live-when/internal/domain"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// mockLiveStatusRepository is a mock implementation of LiveStatusRepository for testing
type mockLiveStatusRepository struct {
	statuses  map[string]*domain.LiveStatus
	createErr error
	getErr    error
	updateErr error
	mu        sync.RWMutex
}

func newMockLiveStatusRepository() *mockLiveStatusRepository {
	return &mockLiveStatusRepository{
		statuses: make(map[string]*domain.LiveStatus),
	}
}

func (m *mockLiveStatusRepository) Create(ctx context.Context, status *domain.LiveStatus) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statuses[status.StreamerID] = status
	return nil
}

func (m *mockLiveStatusRepository) GetByStreamerID(ctx context.Context, streamerID string) (*domain.LiveStatus, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	status, exists := m.statuses[streamerID]
	if !exists {
		return nil, nil
	}
	return status, nil
}

func (m *mockLiveStatusRepository) Update(ctx context.Context, status *domain.LiveStatus) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statuses[status.StreamerID] = status
	return nil
}

func (m *mockLiveStatusRepository) GetAll(ctx context.Context) ([]*domain.LiveStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.LiveStatus, 0, len(m.statuses))
	for _, status := range m.statuses {
		result = append(result, status)
	}
	return result, nil
}

func (m *mockLiveStatusRepository) DeleteOlderThan(ctx context.Context, timestamp time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, status := range m.statuses {
		if status.UpdatedAt.Before(timestamp) {
			delete(m.statuses, id)
		}
	}
	return nil
}

// mockPlatformAdapter is a mock implementation of PlatformAdapter for testing
type mockPlatformAdapter struct {
	liveStatus *domain.PlatformLiveStatus
	err        error
}

func (m *mockPlatformAdapter) GetLiveStatus(ctx context.Context, handle string) (*domain.PlatformLiveStatus, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.liveStatus, nil
}

func (m *mockPlatformAdapter) SearchStreamer(ctx context.Context, query string) ([]*domain.PlatformStreamer, error) {
	return nil, nil
}

func (m *mockPlatformAdapter) GetChannelInfo(ctx context.Context, handle string) (*domain.PlatformChannelInfo, error) {
	return nil, nil
}

// **Feature: streamer-tracking-mvp, Property 4: Live Status Completeness**
// **Validates: Requirements 2.1, 2.2, 2.3**
// Property: For any streamer in the system, retrieving the streamer list should include live status data for all streamers
func TestProperty_LiveStatusCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all streamers have live status in GetAllLiveStatus", prop.ForAll(
		func(numStreamers int, liveStatuses []bool) bool {
			// Ensure we have at least 1 streamer
			if numStreamers < 1 {
				numStreamers = 1
			}
			if numStreamers > 20 {
				numStreamers = 20
			}

			// Ensure liveStatuses matches numStreamers
			if len(liveStatuses) < numStreamers {
				for len(liveStatuses) < numStreamers {
					liveStatuses = append(liveStatuses, false)
				}
			}
			liveStatuses = liveStatuses[:numStreamers]

			ctx := context.Background()
			streamerRepo := newMockStreamerRepository()
			liveStatusRepo := newMockLiveStatusRepository()

			// Create platform adapters
			platformAdapters := make(map[string]domain.PlatformAdapter)

			// Create streamers with varying live statuses
			platforms := []string{"youtube", "kick", "twitch"}
			for i := 0; i < numStreamers; i++ {
				streamerID := "streamer-" + string(rune('a'+i))
				platform := platforms[i%3]

				streamer := &domain.Streamer{
					ID:        streamerID,
					Name:      "Streamer" + streamerID,
					Platforms: []string{platform},
					Handles:   map[string]string{platform: "handle_" + streamerID},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				streamerRepo.streamers[streamerID] = streamer

				// Create mock adapter for this platform if not exists
				if _, exists := platformAdapters[platform]; !exists {
					isLive := liveStatuses[i]
					adapter := &mockPlatformAdapter{
						liveStatus: &domain.PlatformLiveStatus{
							IsLive:      isLive,
							StreamURL:   "https://example.com/" + streamerID,
							Title:       "Stream Title",
							Thumbnail:   "https://example.com/thumb.jpg",
							ViewerCount: 100,
						},
					}
					platformAdapters[platform] = adapter
				}
			}

			// Create service
			service := NewLiveStatusService(streamerRepo, liveStatusRepo, platformAdapters)

			// Get all live statuses
			allStatuses, err := service.GetAllLiveStatus(ctx)
			if err != nil {
				return false
			}

			// Verify all streamers have status
			if len(allStatuses) != numStreamers {
				return false
			}

			// Verify each streamer has a status entry
			for streamerID := range streamerRepo.streamers {
				if _, exists := allStatuses[streamerID]; !exists {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 20),
		gen.SliceOf(gen.Bool()),
	))

	properties.TestingRun(t)
}

// Test GetLiveStatus with live streamer
func TestGetLiveStatus_LiveStreamer(t *testing.T) {
	ctx := context.Background()
	streamerRepo := newMockStreamerRepository()
	liveStatusRepo := newMockLiveStatusRepository()

	// Create a streamer
	streamer := &domain.Streamer{
		ID:        "live-streamer",
		Name:      "LiveStreamer",
		Platforms: []string{"youtube"},
		Handles:   map[string]string{"youtube": "live_handle"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	streamerRepo.streamers["live-streamer"] = streamer

	// Create platform adapter that returns live status
	platformAdapters := map[string]domain.PlatformAdapter{
		"youtube": &mockPlatformAdapter{
			liveStatus: &domain.PlatformLiveStatus{
				IsLive:      true,
				StreamURL:   "https://youtube.com/watch?v=123",
				Title:       "Live Stream",
				Thumbnail:   "https://example.com/thumb.jpg",
				ViewerCount: 500,
			},
		},
	}

	service := NewLiveStatusService(streamerRepo, liveStatusRepo, platformAdapters)

	// Get live status
	status, err := service.GetLiveStatus(ctx, "live-streamer")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !status.IsLive {
		t.Error("expected streamer to be live")
	}
	if status.Platform != "youtube" {
		t.Errorf("expected platform youtube, got %s", status.Platform)
	}
	if status.ViewerCount != 500 {
		t.Errorf("expected viewer count 500, got %d", status.ViewerCount)
	}
}

// Test GetLiveStatus with offline streamer
func TestGetLiveStatus_OfflineStreamer(t *testing.T) {
	ctx := context.Background()
	streamerRepo := newMockStreamerRepository()
	liveStatusRepo := newMockLiveStatusRepository()

	// Create a streamer
	streamer := &domain.Streamer{
		ID:        "offline-streamer",
		Name:      "OfflineStreamer",
		Platforms: []string{"twitch"},
		Handles:   map[string]string{"twitch": "offline_handle"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	streamerRepo.streamers["offline-streamer"] = streamer

	// Create platform adapter that returns offline status
	platformAdapters := map[string]domain.PlatformAdapter{
		"twitch": &mockPlatformAdapter{
			liveStatus: &domain.PlatformLiveStatus{
				IsLive: false,
			},
		},
	}

	service := NewLiveStatusService(streamerRepo, liveStatusRepo, platformAdapters)

	// Get live status
	status, err := service.GetLiveStatus(ctx, "offline-streamer")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if status.IsLive {
		t.Error("expected streamer to be offline")
	}
}

// Test GetLiveStatus caching behavior
func TestGetLiveStatus_CachingBehavior(t *testing.T) {
	ctx := context.Background()
	streamerRepo := newMockStreamerRepository()
	liveStatusRepo := newMockLiveStatusRepository()

	// Create a streamer
	streamer := &domain.Streamer{
		ID:        "cached-streamer",
		Name:      "CachedStreamer",
		Platforms: []string{"kick"},
		Handles:   map[string]string{"kick": "cached_handle"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	streamerRepo.streamers["cached-streamer"] = streamer

	// Create cached status (within TTL)
	cachedStatus := &domain.LiveStatus{
		StreamerID:  "cached-streamer",
		IsLive:      true,
		Platform:    "kick",
		StreamURL:   "https://kick.com/cached",
		Title:       "Cached Stream",
		ViewerCount: 100,
		UpdatedAt:   time.Now().Add(-30 * time.Minute), // 30 minutes ago, within 1 hour TTL
	}
	liveStatusRepo.statuses["cached-streamer"] = cachedStatus

	// Create platform adapter (should not be called due to cache)
	platformAdapters := map[string]domain.PlatformAdapter{
		"kick": &mockPlatformAdapter{
			liveStatus: &domain.PlatformLiveStatus{
				IsLive:      false, // Different from cache
				StreamURL:   "",
				Title:       "",
				ViewerCount: 0,
			},
		},
	}

	service := NewLiveStatusService(streamerRepo, liveStatusRepo, platformAdapters)

	// Get live status - should return cached value
	status, err := service.GetLiveStatus(ctx, "cached-streamer")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should return cached status
	if !status.IsLive {
		t.Error("expected cached status to be live")
	}
	if status.ViewerCount != 100 {
		t.Errorf("expected cached viewer count 100, got %d", status.ViewerCount)
	}
}

// Test GetLiveStatus with expired cache
func TestGetLiveStatus_ExpiredCache(t *testing.T) {
	ctx := context.Background()
	streamerRepo := newMockStreamerRepository()
	liveStatusRepo := newMockLiveStatusRepository()

	// Create a streamer
	streamer := &domain.Streamer{
		ID:        "expired-cache-streamer",
		Name:      "ExpiredCacheStreamer",
		Platforms: []string{"youtube"},
		Handles:   map[string]string{"youtube": "expired_handle"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	streamerRepo.streamers["expired-cache-streamer"] = streamer

	// Create expired cached status (older than TTL)
	expiredStatus := &domain.LiveStatus{
		StreamerID:  "expired-cache-streamer",
		IsLive:      true,
		Platform:    "youtube",
		StreamURL:   "https://youtube.com/old",
		Title:       "Old Stream",
		ViewerCount: 50,
		UpdatedAt:   time.Now().Add(-2 * time.Hour), // 2 hours ago, expired
	}
	liveStatusRepo.statuses["expired-cache-streamer"] = expiredStatus

	// Create platform adapter with fresh data
	platformAdapters := map[string]domain.PlatformAdapter{
		"youtube": &mockPlatformAdapter{
			liveStatus: &domain.PlatformLiveStatus{
				IsLive:      false, // Now offline
				StreamURL:   "",
				Title:       "",
				ViewerCount: 0,
			},
		},
	}

	service := NewLiveStatusService(streamerRepo, liveStatusRepo, platformAdapters)

	// Get live status - should refresh from platform
	status, err := service.GetLiveStatus(ctx, "expired-cache-streamer")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should return fresh status (offline)
	if status.IsLive {
		t.Error("expected fresh status to be offline")
	}
}

// Test GetLiveStatus error handling when platform is unavailable
func TestGetLiveStatus_PlatformUnavailable(t *testing.T) {
	ctx := context.Background()
	streamerRepo := newMockStreamerRepository()
	liveStatusRepo := newMockLiveStatusRepository()

	// Create a streamer
	streamer := &domain.Streamer{
		ID:        "error-streamer",
		Name:      "ErrorStreamer",
		Platforms: []string{"twitch"},
		Handles:   map[string]string{"twitch": "error_handle"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	streamerRepo.streamers["error-streamer"] = streamer

	// Create platform adapter that returns error
	platformAdapters := map[string]domain.PlatformAdapter{
		"twitch": &mockPlatformAdapter{
			err: errors.New("platform unavailable"),
		},
	}

	service := NewLiveStatusService(streamerRepo, liveStatusRepo, platformAdapters)

	// Get live status - should return error when no cached data exists
	_, err := service.GetLiveStatus(ctx, "error-streamer")
	if err == nil {
		t.Fatal("expected error when platform is unavailable and no cache exists")
	}
}

// Test GetLiveStatus with cached data fallback when platform fails
func TestGetLiveStatus_FallbackToCachedData(t *testing.T) {
	ctx := context.Background()
	streamerRepo := newMockStreamerRepository()
	liveStatusRepo := newMockLiveStatusRepository()

	// Create a streamer
	streamer := &domain.Streamer{
		ID:        "fallback-streamer",
		Name:      "FallbackStreamer",
		Platforms: []string{"kick"},
		Handles:   map[string]string{"kick": "fallback_handle"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	streamerRepo.streamers["fallback-streamer"] = streamer

	// Create expired cached status
	cachedStatus := &domain.LiveStatus{
		StreamerID:  "fallback-streamer",
		IsLive:      true,
		Platform:    "kick",
		StreamURL:   "https://kick.com/fallback",
		Title:       "Cached Stream",
		ViewerCount: 200,
		UpdatedAt:   time.Now().Add(-2 * time.Hour), // Expired
	}
	liveStatusRepo.statuses["fallback-streamer"] = cachedStatus

	// Create platform adapter that returns error
	platformAdapters := map[string]domain.PlatformAdapter{
		"kick": &mockPlatformAdapter{
			err: errors.New("platform unavailable"),
		},
	}

	service := NewLiveStatusService(streamerRepo, liveStatusRepo, platformAdapters)

	// Get live status - should fall back to cached data
	status, err := service.GetLiveStatus(ctx, "fallback-streamer")
	if err != nil {
		t.Fatalf("expected no error with fallback, got %v", err)
	}

	// Should return cached status as fallback
	if !status.IsLive {
		t.Error("expected fallback to cached live status")
	}
	if status.ViewerCount != 200 {
		t.Errorf("expected cached viewer count 200, got %d", status.ViewerCount)
	}
}

// Test RefreshLiveStatus forces refresh
func TestRefreshLiveStatus_ForcesRefresh(t *testing.T) {
	ctx := context.Background()
	streamerRepo := newMockStreamerRepository()
	liveStatusRepo := newMockLiveStatusRepository()

	// Create a streamer
	streamer := &domain.Streamer{
		ID:        "refresh-streamer",
		Name:      "RefreshStreamer",
		Platforms: []string{"youtube"},
		Handles:   map[string]string{"youtube": "refresh_handle"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	streamerRepo.streamers["refresh-streamer"] = streamer

	// Create cached status
	cachedStatus := &domain.LiveStatus{
		StreamerID: "refresh-streamer",
		IsLive:     false,
		Platform:   "youtube",
		UpdatedAt:  time.Now().Add(-10 * time.Minute), // Recent cache
	}
	liveStatusRepo.statuses["refresh-streamer"] = cachedStatus

	// Create platform adapter with new live status
	platformAdapters := map[string]domain.PlatformAdapter{
		"youtube": &mockPlatformAdapter{
			liveStatus: &domain.PlatformLiveStatus{
				IsLive:      true,
				StreamURL:   "https://youtube.com/live",
				Title:       "Now Live!",
				ViewerCount: 1000,
			},
		},
	}

	service := NewLiveStatusService(streamerRepo, liveStatusRepo, platformAdapters)

	// Refresh live status - should query platform even with valid cache
	status, err := service.RefreshLiveStatus(ctx, "refresh-streamer")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should return fresh status from platform
	if !status.IsLive {
		t.Error("expected refreshed status to be live")
	}
	if status.ViewerCount != 1000 {
		t.Errorf("expected viewer count 1000, got %d", status.ViewerCount)
	}
}
