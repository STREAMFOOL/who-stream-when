package seed

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
	return m.streamers[id], nil
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

func (m *mockStreamerRepository) List(ctx context.Context, limit int) ([]*domain.Streamer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.Streamer, 0, len(m.streamers))
	for _, s := range m.streamers {
		result = append(result, s)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (m *mockStreamerRepository) Update(ctx context.Context, streamer *domain.Streamer) error {
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
	for _, s := range m.streamers {
		for _, p := range s.Platforms {
			if p == platform {
				result = append(result, s)
				break
			}
		}
	}
	return result, nil
}

func (m *mockStreamerRepository) GetByPlatformHandle(ctx context.Context, platform, handle string) (*domain.Streamer, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.streamers {
		if h, ok := s.Handles[platform]; ok && h == handle {
			return s, nil
		}
	}
	return nil, nil
}

// mockKickAdapter is a mock implementation of PlatformAdapter for testing
type mockKickAdapter struct {
	channels    map[string]*domain.PlatformChannelInfo
	getInfoErr  error
	searchErr   error
	liveStatErr error
}

func newMockKickAdapter() *mockKickAdapter {
	return &mockKickAdapter{
		channels: make(map[string]*domain.PlatformChannelInfo),
	}
}

func (m *mockKickAdapter) GetChannelInfo(ctx context.Context, handle string) (*domain.PlatformChannelInfo, error) {
	if m.getInfoErr != nil {
		return nil, m.getInfoErr
	}
	if info, ok := m.channels[handle]; ok {
		return info, nil
	}
	return nil, errors.New("channel not found")
}

func (m *mockKickAdapter) SearchStreamer(ctx context.Context, query string) ([]*domain.PlatformStreamer, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return nil, nil
}

func (m *mockKickAdapter) GetLiveStatus(ctx context.Context, handle string) (*domain.PlatformLiveStatus, error) {
	if m.liveStatErr != nil {
		return nil, m.liveStatErr
	}
	return &domain.PlatformLiveStatus{IsLive: false}, nil
}

func TestSeedPopularStreamers_CreatesExpectedStreamers(t *testing.T) {
	repo := newMockStreamerRepository()
	adapter := newMockKickAdapter()

	// Set up mock channel info for all popular streamers
	for _, handle := range PopularKickStreamers {
		adapter.channels[handle] = &domain.PlatformChannelInfo{
			Handle:   handle,
			Name:     "Test " + handle,
			Platform: "kick",
		}
	}

	seeder := NewSeeder(repo, adapter)
	result, err := seeder.SeedPopularStreamers(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Created) != len(PopularKickStreamers) {
		t.Errorf("expected %d created, got %d", len(PopularKickStreamers), len(result.Created))
	}

	if len(result.Skipped) != 0 {
		t.Errorf("expected 0 skipped, got %d", len(result.Skipped))
	}

	if len(result.Failed) != 0 {
		t.Errorf("expected 0 failed, got %d", len(result.Failed))
	}

	// Verify streamers were created in repo
	for _, handle := range PopularKickStreamers {
		streamer, _ := repo.GetByPlatformHandle(context.Background(), "kick", handle)
		if streamer == nil {
			t.Errorf("expected streamer %s to be created", handle)
		}
	}
}

func TestSeedPopularStreamers_Idempotency(t *testing.T) {
	repo := newMockStreamerRepository()
	adapter := newMockKickAdapter()

	// Set up mock channel info
	for _, handle := range PopularKickStreamers {
		adapter.channels[handle] = &domain.PlatformChannelInfo{
			Handle:   handle,
			Name:     "Test " + handle,
			Platform: "kick",
		}
	}

	// Override timeNow for consistent IDs
	originalTimeNow := timeNow
	counter := int64(0)
	timeNow = func() time.Time {
		counter++
		return time.Unix(counter, 0)
	}
	defer func() { timeNow = originalTimeNow }()

	seeder := NewSeeder(repo, adapter)

	// First run - should create all
	result1, err := seeder.SeedPopularStreamers(context.Background())
	if err != nil {
		t.Fatalf("first run: expected no error, got %v", err)
	}

	if len(result1.Created) != len(PopularKickStreamers) {
		t.Errorf("first run: expected %d created, got %d", len(PopularKickStreamers), len(result1.Created))
	}

	initialCount := len(repo.streamers)

	// Second run - should skip all (idempotent)
	result2, err := seeder.SeedPopularStreamers(context.Background())
	if err != nil {
		t.Fatalf("second run: expected no error, got %v", err)
	}

	if len(result2.Created) != 0 {
		t.Errorf("second run: expected 0 created, got %d", len(result2.Created))
	}

	if len(result2.Skipped) != len(PopularKickStreamers) {
		t.Errorf("second run: expected %d skipped, got %d", len(PopularKickStreamers), len(result2.Skipped))
	}

	// Verify no duplicates were created
	if len(repo.streamers) != initialCount {
		t.Errorf("expected %d streamers after second run, got %d", initialCount, len(repo.streamers))
	}
}

func TestSeedPopularStreamers_HandlesAPIErrors(t *testing.T) {
	repo := newMockStreamerRepository()
	adapter := newMockKickAdapter()

	// Only set up some channels, others will fail
	adapter.channels["xqc"] = &domain.PlatformChannelInfo{
		Handle:   "xqc",
		Name:     "xQc",
		Platform: "kick",
	}

	seeder := NewSeeder(repo, adapter)
	result, err := seeder.SeedPopularStreamers(context.Background())

	// Should not return error, but track failures
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Created) != 1 {
		t.Errorf("expected 1 created, got %d", len(result.Created))
	}

	if len(result.Failed) != len(PopularKickStreamers)-1 {
		t.Errorf("expected %d failed, got %d", len(PopularKickStreamers)-1, len(result.Failed))
	}

	if len(result.Errors) != len(PopularKickStreamers)-1 {
		t.Errorf("expected %d errors, got %d", len(PopularKickStreamers)-1, len(result.Errors))
	}
}

func TestSeedPopularStreamers_HandlesRepoErrors(t *testing.T) {
	repo := newMockStreamerRepository()
	repo.createErr = errors.New("database error")
	adapter := newMockKickAdapter()

	// Set up mock channel info
	for _, handle := range PopularKickStreamers {
		adapter.channels[handle] = &domain.PlatformChannelInfo{
			Handle:   handle,
			Name:     "Test " + handle,
			Platform: "kick",
		}
	}

	seeder := NewSeeder(repo, adapter)
	result, err := seeder.SeedPopularStreamers(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// All should fail due to repo error
	if len(result.Failed) != len(PopularKickStreamers) {
		t.Errorf("expected %d failed, got %d", len(PopularKickStreamers), len(result.Failed))
	}
}
