package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/user/who-live-when/internal/domain"
	"github.com/user/who-live-when/internal/repository"
)

var (
	// ErrLiveStatusNotFound is returned when live status cannot be found
	ErrLiveStatusNotFound = errors.New("live status not found")
	// ErrPlatformUnavailable is returned when a platform adapter fails
	ErrPlatformUnavailable = errors.New("platform unavailable")
)

const (
	// cacheTTL is the time-to-live for cached live status (1 hour)
	cacheTTL = 1 * time.Hour
)

// liveStatusService implements the LiveStatusService interface
type liveStatusService struct {
	streamerRepo     repository.StreamerRepository
	liveStatusRepo   repository.LiveStatusRepository
	platformAdapters map[string]domain.PlatformAdapter
	mu               sync.RWMutex
}

// NewLiveStatusService creates a new LiveStatusService instance
func NewLiveStatusService(
	streamerRepo repository.StreamerRepository,
	liveStatusRepo repository.LiveStatusRepository,
	platformAdapters map[string]domain.PlatformAdapter,
) domain.LiveStatusService {
	return &liveStatusService{
		streamerRepo:     streamerRepo,
		liveStatusRepo:   liveStatusRepo,
		platformAdapters: platformAdapters,
	}
}

// GetLiveStatus retrieves the live status for a streamer, using cache if available
func (l *liveStatusService) GetLiveStatus(ctx context.Context, streamerID string) (*domain.LiveStatus, error) {
	if streamerID == "" {
		return nil, fmt.Errorf("streamer ID cannot be empty")
	}

	// Try to get cached status first
	cachedStatus, err := l.liveStatusRepo.GetByStreamerID(ctx, streamerID)
	if err == nil && cachedStatus != nil {
		// Check if cache is still valid (within TTL)
		if time.Since(cachedStatus.UpdatedAt) < cacheTTL {
			return cachedStatus, nil
		}
	}

	// Cache miss or expired, refresh from platform
	status, err := l.RefreshLiveStatus(ctx, streamerID)
	if err != nil {
		// If refresh fails and we have cached data, return it with a note
		if cachedStatus != nil {
			return cachedStatus, nil
		}
		return nil, err
	}

	return status, nil
}

// RefreshLiveStatus forces a refresh of live status from platform adapters
func (l *liveStatusService) RefreshLiveStatus(ctx context.Context, streamerID string) (*domain.LiveStatus, error) {
	if streamerID == "" {
		return nil, fmt.Errorf("streamer ID cannot be empty")
	}

	// Get streamer information
	streamer, err := l.streamerRepo.GetByID(ctx, streamerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get streamer: %w", err)
	}
	if streamer == nil {
		return nil, ErrStreamerNotFound
	}

	// Query all platforms for this streamer
	var liveStatus *domain.LiveStatus
	var lastErr error

	for _, platform := range streamer.Platforms {
		adapter, ok := l.platformAdapters[platform]
		if !ok {
			lastErr = fmt.Errorf("%w: no adapter for platform %s", ErrPlatformUnavailable, platform)
			continue
		}

		handle, ok := streamer.Handles[platform]
		if !ok || handle == "" {
			continue
		}

		platformStatus, err := adapter.GetLiveStatus(ctx, handle)
		if err != nil {
			lastErr = fmt.Errorf("platform %s error: %w", platform, err)
			continue
		}

		// If streamer is live on this platform, use this status
		if platformStatus.IsLive {
			liveStatus = &domain.LiveStatus{
				StreamerID:  streamerID,
				IsLive:      true,
				Platform:    platform,
				StreamURL:   platformStatus.StreamURL,
				Title:       platformStatus.Title,
				Thumbnail:   platformStatus.Thumbnail,
				ViewerCount: platformStatus.ViewerCount,
				UpdatedAt:   time.Now(),
			}
			break
		}
	}

	// If no live status found on any platform, create offline status
	if liveStatus == nil {
		liveStatus = &domain.LiveStatus{
			StreamerID: streamerID,
			IsLive:     false,
			UpdatedAt:  time.Now(),
		}
	}

	// Save to cache
	existingStatus, err := l.liveStatusRepo.GetByStreamerID(ctx, streamerID)
	if err == nil && existingStatus != nil {
		// Update existing record
		if err := l.liveStatusRepo.Update(ctx, liveStatus); err != nil {
			// Log error but don't fail the request
			lastErr = fmt.Errorf("failed to update cache: %w", err)
		}
	} else {
		// Create new record
		if err := l.liveStatusRepo.Create(ctx, liveStatus); err != nil {
			// Log error but don't fail the request
			lastErr = fmt.Errorf("failed to create cache: %w", err)
		}
	}

	// If we got a status but had some errors, still return the status
	if liveStatus != nil && liveStatus.IsLive {
		return liveStatus, nil
	}

	// If offline and we had errors, return the error
	if lastErr != nil && !liveStatus.IsLive {
		return liveStatus, lastErr
	}

	return liveStatus, nil
}

// GetAllLiveStatus retrieves live status for all streamers
func (l *liveStatusService) GetAllLiveStatus(ctx context.Context) (map[string]*domain.LiveStatus, error) {
	// Get all streamers
	streamers, err := l.streamerRepo.List(ctx, 1000) // Large limit to get all
	if err != nil {
		return nil, fmt.Errorf("failed to list streamers: %w", err)
	}

	result := make(map[string]*domain.LiveStatus)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Query status for each streamer in parallel
	for _, streamer := range streamers {
		wg.Add(1)
		go func(s *domain.Streamer) {
			defer wg.Done()

			status, err := l.GetLiveStatus(ctx, s.ID)
			if err != nil {
				// Skip streamers with errors
				return
			}

			mu.Lock()
			result[s.ID] = status
			mu.Unlock()
		}(streamer)
	}

	wg.Wait()

	return result, nil
}
