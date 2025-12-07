package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/user/who-live-when/internal/domain"
	"github.com/user/who-live-when/internal/repository"
)

var (
	// ErrStreamerNotFound is returned when a streamer cannot be found
	ErrStreamerNotFound = errors.New("streamer not found")
	// ErrInvalidStreamerData is returned when streamer data is invalid
	ErrInvalidStreamerData = errors.New("invalid streamer data")
	// ErrInvalidPlatform is returned when an unsupported platform is specified
	ErrInvalidPlatform = errors.New("invalid platform")
)

// Supported platforms
var supportedPlatforms = map[string]bool{
	"youtube": true,
	"kick":    true,
	"twitch":  true,
}

// streamerService implements the StreamerService interface
type streamerService struct {
	repo repository.StreamerRepository
}

// NewStreamerService creates a new StreamerService instance
func NewStreamerService(repo repository.StreamerRepository) domain.StreamerService {
	return &streamerService{
		repo: repo,
	}
}

// GetStreamer retrieves a streamer by ID
func (s *streamerService) GetStreamer(ctx context.Context, id string) (*domain.Streamer, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: id cannot be empty", ErrInvalidStreamerData)
	}

	streamer, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get streamer: %w", err)
	}
	if streamer == nil {
		return nil, ErrStreamerNotFound
	}

	return streamer, nil
}

// ListStreamers retrieves a list of streamers with a limit
func (s *streamerService) ListStreamers(ctx context.Context, limit int) ([]*domain.Streamer, error) {
	if limit <= 0 {
		limit = 50 // Default limit
	}

	streamers, err := s.repo.List(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list streamers: %w", err)
	}

	return streamers, nil
}

// SearchStreamers searches for streamers by query (not implemented in this task)
func (s *streamerService) SearchStreamers(ctx context.Context, query string) ([]*domain.Streamer, error) {
	// This will be implemented in a later task with platform adapters
	return nil, errors.New("not implemented")
}

// AddStreamer adds a new streamer to the system
func (s *streamerService) AddStreamer(ctx context.Context, streamer *domain.Streamer) error {
	if err := s.validateStreamer(streamer); err != nil {
		return err
	}

	if err := s.repo.Create(ctx, streamer); err != nil {
		return fmt.Errorf("failed to add streamer: %w", err)
	}

	return nil
}

// UpdateStreamer updates an existing streamer's information
func (s *streamerService) UpdateStreamer(ctx context.Context, streamer *domain.Streamer) error {
	if streamer.ID == "" {
		return fmt.Errorf("%w: id cannot be empty", ErrInvalidStreamerData)
	}

	if err := s.validateStreamer(streamer); err != nil {
		return err
	}

	// Verify streamer exists
	existing, err := s.repo.GetByID(ctx, streamer.ID)
	if err != nil {
		return fmt.Errorf("failed to get streamer: %w", err)
	}
	if existing == nil {
		return ErrStreamerNotFound
	}

	if err := s.repo.Update(ctx, streamer); err != nil {
		return fmt.Errorf("failed to update streamer: %w", err)
	}

	return nil
}

// GetStreamersByPlatform retrieves all streamers for a specific platform
func (s *streamerService) GetStreamersByPlatform(ctx context.Context, platform string) ([]*domain.Streamer, error) {
	platform = strings.ToLower(platform)

	if !supportedPlatforms[platform] {
		return nil, fmt.Errorf("%w: %s", ErrInvalidPlatform, platform)
	}

	streamers, err := s.repo.GetByPlatform(ctx, platform)
	if err != nil {
		return nil, fmt.Errorf("failed to get streamers by platform: %w", err)
	}

	return streamers, nil
}

// validateStreamer validates streamer data
func (s *streamerService) validateStreamer(streamer *domain.Streamer) error {
	if streamer == nil {
		return fmt.Errorf("%w: streamer cannot be nil", ErrInvalidStreamerData)
	}

	if streamer.Name == "" {
		return fmt.Errorf("%w: name cannot be empty", ErrInvalidStreamerData)
	}

	if len(streamer.Platforms) == 0 {
		return fmt.Errorf("%w: at least one platform is required", ErrInvalidStreamerData)
	}

	if streamer.Handles == nil || len(streamer.Handles) == 0 {
		return fmt.Errorf("%w: at least one handle is required", ErrInvalidStreamerData)
	}

	// Validate platforms
	for _, platform := range streamer.Platforms {
		normalizedPlatform := strings.ToLower(platform)
		if !supportedPlatforms[normalizedPlatform] {
			return fmt.Errorf("%w: %s", ErrInvalidPlatform, platform)
		}
	}

	// Validate handles match platforms
	for platform, handle := range streamer.Handles {
		normalizedPlatform := strings.ToLower(platform)
		if !supportedPlatforms[normalizedPlatform] {
			return fmt.Errorf("%w: %s", ErrInvalidPlatform, platform)
		}
		if handle == "" {
			return fmt.Errorf("%w: handle for platform %s cannot be empty", ErrInvalidStreamerData, platform)
		}
	}

	return nil
}
