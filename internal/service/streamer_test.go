package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"who-live-when/internal/domain"
)

// mockStreamerRepository is a mock implementation of StreamerRepository for testing
type mockStreamerRepository struct {
	streamers map[string]*domain.Streamer
	createErr error
	getErr    error
	updateErr error
	listErr   error
	mu        sync.RWMutex
}

func newMockStreamerRepository() *mockStreamerRepository {
	return &mockStreamerRepository{
		streamers: make(map[string]*domain.Streamer),
	}
}

func (m *mockStreamerRepository) Create(ctx context.Context, streamer *domain.Streamer) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streamers[streamer.ID] = streamer
	return nil
}

func (m *mockStreamerRepository) GetByID(ctx context.Context, id string) (*domain.Streamer, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	streamer, exists := m.streamers[id]
	if !exists {
		return nil, nil
	}
	return streamer, nil
}

func (m *mockStreamerRepository) List(ctx context.Context, limit int) ([]*domain.Streamer, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.Streamer, 0, len(m.streamers))
	count := 0
	for _, streamer := range m.streamers {
		if count >= limit {
			break
		}
		result = append(result, streamer)
		count++
	}
	return result, nil
}

func (m *mockStreamerRepository) Update(ctx context.Context, streamer *domain.Streamer) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streamers[streamer.ID] = streamer
	return nil
}

func (m *mockStreamerRepository) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.streamers, id)
	return nil
}

func (m *mockStreamerRepository) GetByPlatform(ctx context.Context, platform string) ([]*domain.Streamer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.Streamer, 0)
	for _, streamer := range m.streamers {
		for _, p := range streamer.Platforms {
			if p == platform {
				result = append(result, streamer)
				break
			}
		}
	}
	return result, nil
}

func (m *mockStreamerRepository) GetByIDs(ctx context.Context, ids []string) ([]*domain.Streamer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.Streamer, 0, len(ids))
	for _, id := range ids {
		if s, ok := m.streamers[id]; ok {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockStreamerRepository) GetByPlatformHandle(ctx context.Context, platform, handle string) (*domain.Streamer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, streamer := range m.streamers {
		if h, ok := streamer.Handles[platform]; ok && h == handle {
			return streamer, nil
		}
	}
	return nil, nil
}

// Test GetStreamer with valid ID
func TestGetStreamer_ValidID(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	// Add a test streamer
	testStreamer := &domain.Streamer{
		ID:        "test-id-1",
		Name:      "TestStreamer",
		Platforms: []string{"youtube"},
		Handles:   map[string]string{"youtube": "test_handle"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo.streamers["test-id-1"] = testStreamer

	// Get the streamer
	result, err := service.GetStreamer(ctx, "test-id-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ID != "test-id-1" {
		t.Errorf("expected ID test-id-1, got %s", result.ID)
	}
	if result.Name != "TestStreamer" {
		t.Errorf("expected name TestStreamer, got %s", result.Name)
	}
}

// Test GetStreamer with invalid (empty) ID
func TestGetStreamer_EmptyID(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	_, err := service.GetStreamer(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
	if !errors.Is(err, ErrInvalidStreamerData) {
		t.Errorf("expected ErrInvalidStreamerData, got %v", err)
	}
}

// Test GetStreamer with non-existent ID
func TestGetStreamer_NotFound(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	_, err := service.GetStreamer(ctx, "non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent ID, got nil")
	}
	if !errors.Is(err, ErrStreamerNotFound) {
		t.Errorf("expected ErrStreamerNotFound, got %v", err)
	}
}

// Test AddStreamer with valid data
func TestAddStreamer_ValidData(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	streamer := &domain.Streamer{
		ID:        "new-id",
		Name:      "NewStreamer",
		Platforms: []string{"twitch"},
		Handles:   map[string]string{"twitch": "new_handle"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := service.AddStreamer(ctx, streamer)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify it was added
	result, _ := repo.GetByID(ctx, "new-id")
	if result == nil {
		t.Fatal("streamer was not added to repository")
	}
	if result.Name != "NewStreamer" {
		t.Errorf("expected name NewStreamer, got %s", result.Name)
	}
}

// Test AddStreamer with invalid data (empty name)
func TestAddStreamer_EmptyName(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	streamer := &domain.Streamer{
		ID:        "test-id",
		Name:      "",
		Platforms: []string{"youtube"},
		Handles:   map[string]string{"youtube": "handle"},
	}

	err := service.AddStreamer(ctx, streamer)
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	if !errors.Is(err, ErrInvalidStreamerData) {
		t.Errorf("expected ErrInvalidStreamerData, got %v", err)
	}
}

// Test AddStreamer with invalid data (no platforms)
func TestAddStreamer_NoPlatforms(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	streamer := &domain.Streamer{
		ID:        "test-id",
		Name:      "TestStreamer",
		Platforms: []string{},
		Handles:   map[string]string{},
	}

	err := service.AddStreamer(ctx, streamer)
	if err == nil {
		t.Fatal("expected error for no platforms, got nil")
	}
	if !errors.Is(err, ErrInvalidStreamerData) {
		t.Errorf("expected ErrInvalidStreamerData, got %v", err)
	}
}

// Test AddStreamer with invalid platform
func TestAddStreamer_InvalidPlatform(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	streamer := &domain.Streamer{
		ID:        "test-id",
		Name:      "TestStreamer",
		Platforms: []string{"invalid_platform"},
		Handles:   map[string]string{"invalid_platform": "handle"},
	}

	err := service.AddStreamer(ctx, streamer)
	if err == nil {
		t.Fatal("expected error for invalid platform, got nil")
	}
	if !errors.Is(err, ErrInvalidPlatform) {
		t.Errorf("expected ErrInvalidPlatform, got %v", err)
	}
}

// Test UpdateStreamer with platform changes
func TestUpdateStreamer_PlatformChanges(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	// Add initial streamer
	original := &domain.Streamer{
		ID:        "update-id",
		Name:      "UpdateStreamer",
		Platforms: []string{"youtube"},
		Handles:   map[string]string{"youtube": "original_handle"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo.streamers["update-id"] = original

	// Update with new platform
	updated := &domain.Streamer{
		ID:        "update-id",
		Name:      "UpdateStreamer",
		Platforms: []string{"youtube", "twitch"},
		Handles: map[string]string{
			"youtube": "original_handle",
			"twitch":  "new_handle",
		},
		CreatedAt: original.CreatedAt,
		UpdatedAt: time.Now(),
	}

	err := service.UpdateStreamer(ctx, updated)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify update
	result, _ := repo.GetByID(ctx, "update-id")
	if len(result.Platforms) != 2 {
		t.Errorf("expected 2 platforms, got %d", len(result.Platforms))
	}
	if result.Handles["twitch"] != "new_handle" {
		t.Errorf("expected twitch handle new_handle, got %s", result.Handles["twitch"])
	}
}

// Test UpdateStreamer with non-existent streamer
func TestUpdateStreamer_NotFound(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	streamer := &domain.Streamer{
		ID:        "non-existent",
		Name:      "TestStreamer",
		Platforms: []string{"youtube"},
		Handles:   map[string]string{"youtube": "handle"},
	}

	err := service.UpdateStreamer(ctx, streamer)
	if err == nil {
		t.Fatal("expected error for non-existent streamer, got nil")
	}
	if !errors.Is(err, ErrStreamerNotFound) {
		t.Errorf("expected ErrStreamerNotFound, got %v", err)
	}
}

// Test GetStreamersByPlatform filtering
func TestGetStreamersByPlatform_Filtering(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	// Add streamers with different platforms
	repo.streamers["id1"] = &domain.Streamer{
		ID:        "id1",
		Name:      "YouTubeStreamer",
		Platforms: []string{"youtube"},
		Handles:   map[string]string{"youtube": "yt_handle"},
	}
	repo.streamers["id2"] = &domain.Streamer{
		ID:        "id2",
		Name:      "TwitchStreamer",
		Platforms: []string{"twitch"},
		Handles:   map[string]string{"twitch": "tw_handle"},
	}
	repo.streamers["id3"] = &domain.Streamer{
		ID:        "id3",
		Name:      "MultiPlatformStreamer",
		Platforms: []string{"youtube", "twitch"},
		Handles: map[string]string{
			"youtube": "multi_yt",
			"twitch":  "multi_tw",
		},
	}

	// Get YouTube streamers
	youtubeStreamers, err := service.GetStreamersByPlatform(ctx, "youtube")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(youtubeStreamers) != 2 {
		t.Errorf("expected 2 YouTube streamers, got %d", len(youtubeStreamers))
	}

	// Get Twitch streamers
	twitchStreamers, err := service.GetStreamersByPlatform(ctx, "twitch")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(twitchStreamers) != 2 {
		t.Errorf("expected 2 Twitch streamers, got %d", len(twitchStreamers))
	}

	// Get Kick streamers (should be empty)
	kickStreamers, err := service.GetStreamersByPlatform(ctx, "kick")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(kickStreamers) != 0 {
		t.Errorf("expected 0 Kick streamers, got %d", len(kickStreamers))
	}
}

// Test GetStreamersByPlatform with invalid platform
func TestGetStreamersByPlatform_InvalidPlatform(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	_, err := service.GetStreamersByPlatform(ctx, "invalid_platform")
	if err == nil {
		t.Fatal("expected error for invalid platform, got nil")
	}
	if !errors.Is(err, ErrInvalidPlatform) {
		t.Errorf("expected ErrInvalidPlatform, got %v", err)
	}
}

// Test GetOrCreateStreamer creates new streamer when not exists
func TestGetOrCreateStreamer_CreatesNew(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	streamer, err := service.GetOrCreateStreamer(ctx, "youtube", "new_handle", "New Streamer")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if streamer == nil {
		t.Fatal("expected streamer, got nil")
	}
	if streamer.Name != "New Streamer" {
		t.Errorf("expected name 'New Streamer', got '%s'", streamer.Name)
	}
	if streamer.Handles["youtube"] != "new_handle" {
		t.Errorf("expected handle 'new_handle', got '%s'", streamer.Handles["youtube"])
	}
	if len(streamer.Platforms) != 1 || streamer.Platforms[0] != "youtube" {
		t.Errorf("expected platforms [youtube], got %v", streamer.Platforms)
	}

	// Verify it was persisted
	result, _ := repo.GetByPlatformHandle(ctx, "youtube", "new_handle")
	if result == nil {
		t.Fatal("streamer was not persisted to repository")
	}
}

// Test GetOrCreateStreamer returns existing streamer (duplicate detection)
func TestGetOrCreateStreamer_ReturnsExisting(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	// Create initial streamer
	existing := &domain.Streamer{
		ID:        "existing-id",
		Name:      "Existing Streamer",
		Platforms: []string{"twitch"},
		Handles:   map[string]string{"twitch": "existing_handle"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo.streamers["existing-id"] = existing

	// Try to create same streamer
	streamer, err := service.GetOrCreateStreamer(ctx, "twitch", "existing_handle", "Different Name")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should return existing streamer, not create new one
	if streamer.ID != "existing-id" {
		t.Errorf("expected ID 'existing-id', got '%s'", streamer.ID)
	}
	if streamer.Name != "Existing Streamer" {
		t.Errorf("expected name 'Existing Streamer', got '%s'", streamer.Name)
	}

	// Verify no duplicate was created
	count := len(repo.streamers)
	if count != 1 {
		t.Errorf("expected 1 streamer in repo, got %d", count)
	}
}

// Test GetOrCreateStreamer with empty handle
func TestGetOrCreateStreamer_EmptyHandle(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	_, err := service.GetOrCreateStreamer(ctx, "youtube", "", "Streamer Name")
	if err == nil {
		t.Fatal("expected error for empty handle, got nil")
	}
	if !errors.Is(err, ErrInvalidStreamerData) {
		t.Errorf("expected ErrInvalidStreamerData, got %v", err)
	}
}

// Test GetOrCreateStreamer with empty name
func TestGetOrCreateStreamer_EmptyName(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	_, err := service.GetOrCreateStreamer(ctx, "youtube", "handle", "")
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	if !errors.Is(err, ErrInvalidStreamerData) {
		t.Errorf("expected ErrInvalidStreamerData, got %v", err)
	}
}

// Test GetOrCreateStreamer with invalid platform
func TestGetOrCreateStreamer_InvalidPlatform(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	_, err := service.GetOrCreateStreamer(ctx, "invalid_platform", "handle", "Streamer Name")
	if err == nil {
		t.Fatal("expected error for invalid platform, got nil")
	}
	if !errors.Is(err, ErrInvalidPlatform) {
		t.Errorf("expected ErrInvalidPlatform, got %v", err)
	}
}

// Test GetOrCreateStreamer normalizes platform to lowercase
func TestGetOrCreateStreamer_NormalizesPlatform(t *testing.T) {
	repo := newMockStreamerRepository()
	service := NewStreamerService(repo)
	ctx := context.Background()

	streamer, err := service.GetOrCreateStreamer(ctx, "YouTube", "handle", "Streamer Name")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Platform should be normalized to lowercase
	if streamer.Platforms[0] != "youtube" {
		t.Errorf("expected platform 'youtube', got '%s'", streamer.Platforms[0])
	}
}
