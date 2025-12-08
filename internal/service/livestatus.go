package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"who-live-when/internal/domain"
	"who-live-when/internal/logger"
	"who-live-when/internal/repository"
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
	logger           *logger.Logger
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
		logger:           logger.Default(),
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
		l.logger.Error("Failed to get streamer for live status refresh", map[string]interface{}{
			"streamer_id": streamerID,
			"error":       err.Error(),
		})
		return nil, fmt.Errorf("failed to get streamer: %w", err)
	}
	if streamer == nil {
		l.logger.Warn("Streamer not found for live status refresh", map[string]interface{}{
			"streamer_id": streamerID,
		})
		return nil, ErrStreamerNotFound
	}

	// Query all platforms for this streamer
	var liveStatus *domain.LiveStatus
	var lastErr error

	for _, platform := range streamer.Platforms {
		adapter, ok := l.platformAdapters[platform]
		if !ok {
			l.logger.Warn("No adapter available for platform", map[string]interface{}{
				"platform":    platform,
				"streamer_id": streamerID,
			})
			lastErr = fmt.Errorf("%w: no adapter for platform %s", ErrPlatformUnavailable, platform)
			continue
		}

		handle, ok := streamer.Handles[platform]
		if !ok || handle == "" {
			continue
		}

		platformStatus, err := adapter.GetLiveStatus(ctx, handle)
		if err != nil {
			l.logger.Error("Platform adapter failed to get live status", map[string]interface{}{
				"platform":    platform,
				"handle":      handle,
				"streamer_id": streamerID,
				"error":       err.Error(),
			})
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
			l.logger.Error("Failed to update live status cache", map[string]interface{}{
				"streamer_id": streamerID,
				"error":       err.Error(),
			})
			lastErr = fmt.Errorf("failed to update cache: %w", err)
		}
	} else {
		// Create new record
		if err := l.liveStatusRepo.Create(ctx, liveStatus); err != nil {
			l.logger.Error("Failed to create live status cache", map[string]interface{}{
				"streamer_id": streamerID,
				"error":       err.Error(),
			})
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
		l.logger.Error("Failed to list streamers for GetAllLiveStatus", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to list streamers: %w", err)
	}

	l.logger.Info("Fetching live status for all streamers", map[string]interface{}{
		"count": len(streamers),
	})

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
				l.logger.Debug("Skipping streamer due to error", map[string]interface{}{
					"streamer_id": s.ID,
					"error":       err.Error(),
				})
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
