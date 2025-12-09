package service

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"who-live-when/internal/domain"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// mockStreamerRepositoryForProperty implements repository.StreamerRepository for property testing
type mockStreamerRepositoryForProperty struct {
	streamers       map[string]*domain.Streamer
	platformHandles map[string]string // "platform:handle" -> streamerID
	mu              sync.RWMutex
}

func newMockStreamerRepositoryForProperty() *mockStreamerRepositoryForProperty {
	return &mockStreamerRepositoryForProperty{
		streamers:       make(map[string]*domain.Streamer),
		platformHandles: make(map[string]string),
	}
}

func (m *mockStreamerRepositoryForProperty) Create(ctx context.Context, streamer *domain.Streamer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streamers[streamer.ID] = streamer
	for platform, handle := range streamer.Handles {
		key := platform + ":" + handle
		m.platformHandles[key] = streamer.ID
	}
	return nil
}

func (m *mockStreamerRepositoryForProperty) GetByID(ctx context.Context, id string) (*domain.Streamer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.streamers[id], nil
}

func (m *mockStreamerRepositoryForProperty) GetByIDs(ctx context.Context, ids []string) ([]*domain.Streamer, error) {
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

func (m *mockStreamerRepositoryForProperty) List(ctx context.Context, limit int) ([]*domain.Streamer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.Streamer, 0, len(m.streamers))
	count := 0
	for _, s := range m.streamers {
		if count >= limit {
			break
		}
		result = append(result, s)
		count++
	}
	return result, nil
}

func (m *mockStreamerRepositoryForProperty) Update(ctx context.Context, streamer *domain.Streamer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streamers[streamer.ID] = streamer
	return nil
}

func (m *mockStreamerRepositoryForProperty) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.streamers[id]; ok {
		for platform, handle := range s.Handles {
			delete(m.platformHandles, platform+":"+handle)
		}
	}
	delete(m.streamers, id)
	return nil
}

func (m *mockStreamerRepositoryForProperty) GetByPlatform(ctx context.Context, platform string) ([]*domain.Streamer, error) {
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

func (m *mockStreamerRepositoryForProperty) GetByPlatformHandle(ctx context.Context, platform, handle string) (*domain.Streamer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := platform + ":" + handle
	if id, ok := m.platformHandles[key]; ok {
		return m.streamers[id], nil
	}
	return nil, nil
}

func (m *mockStreamerRepositoryForProperty) countStreamers() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.streamers)
}

// **Feature: user-experience-enhancements, Property 14: Streamer Addition Idempotence**
// **Validates: Requirements 8.4**
// Property: For any streamer with specific platform and handle, adding them multiple times
// should result in a single streamer record without duplicates.
func TestProperty_UXE_StreamerAdditionIdempotence(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	platforms := []string{"youtube", "kick", "twitch"}

	properties.Property("adding same streamer multiple times results in single record", prop.ForAll(
		func(platformIdx int, handleSeed, nameSeed int, addCount int) bool {
			// Normalize inputs
			platformIdx = platformIdx % len(platforms)
			platform := platforms[platformIdx]

			// Generate non-empty handle and name from seeds
			handle := fmt.Sprintf("handle_%d", handleSeed)
			name := fmt.Sprintf("Streamer_%d", nameSeed)

			// Normalize add count to reasonable range (2-5)
			if addCount < 2 {
				addCount = 2
			}
			if addCount > 5 {
				addCount = 5
			}

			ctx := context.Background()
			repo := newMockStreamerRepositoryForProperty()
			service := NewStreamerService(repo)

			// Add the same streamer multiple times
			var firstStreamer *domain.Streamer
			for i := 0; i < addCount; i++ {
				streamer, err := service.GetOrCreateStreamer(ctx, platform, handle, name)
				if err != nil {
					return false
				}
				if i == 0 {
					firstStreamer = streamer
				} else {
					// All subsequent calls should return the same streamer ID
					if streamer.ID != firstStreamer.ID {
						return false
					}
				}
			}

			// Verify only one streamer exists in the repository
			if repo.countStreamers() != 1 {
				return false
			}

			// Verify the streamer has correct data
			result, err := repo.GetByPlatformHandle(ctx, platform, handle)
			if err != nil || result == nil {
				return false
			}

			return result.ID == firstStreamer.ID &&
				result.Name == name &&
				result.Handles[platform] == handle
		},
		gen.IntRange(0, 2),
		gen.IntRange(1, 1000),
		gen.IntRange(1, 1000),
		gen.IntRange(2, 5),
	))

	properties.TestingRun(t)
}
