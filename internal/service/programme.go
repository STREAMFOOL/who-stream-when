package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"who-live-when/internal/domain"
	"who-live-when/internal/repository"

	"github.com/google/uuid"
)

var (
	// ErrProgrammeNotFound is returned when a custom programme cannot be found
	ErrProgrammeNotFound = errors.New("custom programme not found")
	// ErrInvalidProgrammeData is returned when programme data is invalid
	ErrInvalidProgrammeData = errors.New("invalid programme data")
)

// ProgrammeService manages custom programme creation and retrieval for all users
type ProgrammeService struct {
	programmeRepo  repository.CustomProgrammeRepository
	streamerRepo   repository.StreamerRepository
	followRepo     repository.FollowRepository
	heatmapService domain.HeatmapService
}

// NewProgrammeService creates a new ProgrammeService instance
func NewProgrammeService(
	programmeRepo repository.CustomProgrammeRepository,
	streamerRepo repository.StreamerRepository,
	followRepo repository.FollowRepository,
	heatmapService domain.HeatmapService,
) *ProgrammeService {
	return &ProgrammeService{
		programmeRepo:  programmeRepo,
		streamerRepo:   streamerRepo,
		followRepo:     followRepo,
		heatmapService: heatmapService,
	}
}

// CreateCustomProgramme creates a new custom programme for a registered user
func (s *ProgrammeService) CreateCustomProgramme(ctx context.Context, userID string, streamerIDs []string) (*domain.CustomProgramme, error) {
	if userID == "" {
		return nil, fmt.Errorf("%w: user ID cannot be empty", ErrInvalidProgrammeData)
	}

	now := time.Now()
	programme := &domain.CustomProgramme{
		ID:          uuid.New().String(),
		UserID:      userID,
		StreamerIDs: streamerIDs,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.programmeRepo.Create(ctx, programme); err != nil {
		return nil, fmt.Errorf("failed to create custom programme: %w", err)
	}

	return programme, nil
}

// GetCustomProgramme retrieves a custom programme for a user
func (s *ProgrammeService) GetCustomProgramme(ctx context.Context, userID string) (*domain.CustomProgramme, error) {
	if userID == "" {
		return nil, fmt.Errorf("%w: user ID cannot be empty", ErrInvalidProgrammeData)
	}

	programme, err := s.programmeRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, ErrProgrammeNotFound
	}

	return programme, nil
}

// UpdateCustomProgramme updates an existing custom programme
func (s *ProgrammeService) UpdateCustomProgramme(ctx context.Context, userID string, streamerIDs []string) error {
	if userID == "" {
		return fmt.Errorf("%w: user ID cannot be empty", ErrInvalidProgrammeData)
	}

	programme, err := s.programmeRepo.GetByUserID(ctx, userID)
	if err != nil {
		return ErrProgrammeNotFound
	}

	programme.StreamerIDs = streamerIDs
	programme.UpdatedAt = time.Now()

	if err := s.programmeRepo.Update(ctx, programme); err != nil {
		return fmt.Errorf("failed to update custom programme: %w", err)
	}

	return nil
}

// DeleteCustomProgramme removes a custom programme for a user
func (s *ProgrammeService) DeleteCustomProgramme(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("%w: user ID cannot be empty", ErrInvalidProgrammeData)
	}

	if err := s.programmeRepo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete custom programme: %w", err)
	}

	return nil
}

// CreateGuestProgramme creates a custom programme for a guest user (session-based)
func (s *ProgrammeService) CreateGuestProgramme(streamerIDs []string) *domain.CustomProgramme {
	now := time.Now()
	return &domain.CustomProgramme{
		ID:          uuid.New().String(),
		UserID:      "", // Empty for guest programmes
		StreamerIDs: streamerIDs,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// AddStreamerToProgramme adds a streamer to an existing programme
func (s *ProgrammeService) AddStreamerToProgramme(ctx context.Context, userID, streamerID string) error {
	if userID == "" {
		return fmt.Errorf("%w: user ID cannot be empty", ErrInvalidProgrammeData)
	}
	if streamerID == "" {
		return fmt.Errorf("%w: streamer ID cannot be empty", ErrInvalidProgrammeData)
	}

	programme, err := s.programmeRepo.GetByUserID(ctx, userID)
	if err != nil {
		return ErrProgrammeNotFound
	}

	// Check if streamer already in programme
	for _, id := range programme.StreamerIDs {
		if id == streamerID {
			return nil // Already in programme, idempotent
		}
	}

	programme.StreamerIDs = append(programme.StreamerIDs, streamerID)
	programme.UpdatedAt = time.Now()

	return s.programmeRepo.Update(ctx, programme)
}

// RemoveStreamerFromProgramme removes a streamer from an existing programme
func (s *ProgrammeService) RemoveStreamerFromProgramme(ctx context.Context, userID, streamerID string) error {
	if userID == "" {
		return fmt.Errorf("%w: user ID cannot be empty", ErrInvalidProgrammeData)
	}
	if streamerID == "" {
		return fmt.Errorf("%w: streamer ID cannot be empty", ErrInvalidProgrammeData)
	}

	programme, err := s.programmeRepo.GetByUserID(ctx, userID)
	if err != nil {
		return ErrProgrammeNotFound
	}

	// Filter out the streamer
	var newStreamerIDs []string
	for _, id := range programme.StreamerIDs {
		if id != streamerID {
			newStreamerIDs = append(newStreamerIDs, id)
		}
	}

	programme.StreamerIDs = newStreamerIDs
	programme.UpdatedAt = time.Now()

	return s.programmeRepo.Update(ctx, programme)
}

// CalendarView represents a calendar view with programme data
type ProgrammeCalendarView struct {
	Week           time.Time
	Streamers      []*domain.Streamer
	Entries        []domain.ProgrammeEntry
	IsCustom       bool
	IsGuestSession bool
}

// GenerateCalendarFromProgramme generates a calendar view from a custom programme
func (s *ProgrammeService) GenerateCalendarFromProgramme(ctx context.Context, programme *domain.CustomProgramme, week time.Time) (*ProgrammeCalendarView, error) {
	if programme == nil {
		return nil, fmt.Errorf("%w: programme cannot be nil", ErrInvalidProgrammeData)
	}

	weekStart := normalizeWeekStart(week)
	var streamers []*domain.Streamer
	var entries []domain.ProgrammeEntry

	// Load streamers and generate entries only for streamers in the programme
	for _, streamerID := range programme.StreamerIDs {
		streamer, err := s.streamerRepo.GetByID(ctx, streamerID)
		if err != nil {
			continue // Skip streamers that can't be found
		}
		streamers = append(streamers, streamer)

		// Generate heatmap entries for this streamer
		heatmap, err := s.heatmapService.GenerateHeatmap(ctx, streamerID)
		if err != nil {
			continue // Skip streamers without heatmap data
		}

		for dayOfWeek := 0; dayOfWeek < 7; dayOfWeek++ {
			dayProbability := heatmap.DaysOfWeek[dayOfWeek]
			if dayProbability > 0.1 {
				for hour := 0; hour < 24; hour++ {
					hourProbability := heatmap.Hours[hour]
					combinedProbability := dayProbability * hourProbability

					if combinedProbability > 0.05 {
						entries = append(entries, domain.ProgrammeEntry{
							StreamerID:  streamerID,
							DayOfWeek:   dayOfWeek,
							Hour:        hour,
							Probability: combinedProbability,
						})
					}
				}
			}
		}
	}

	return &ProgrammeCalendarView{
		Week:           weekStart,
		Streamers:      streamers,
		Entries:        entries,
		IsCustom:       true,
		IsGuestSession: programme.UserID == "",
	}, nil
}

// StreamerWithFollowers represents a streamer with their follower count for ranking
type StreamerWithFollowers struct {
	Streamer      *domain.Streamer
	FollowerCount int
}

// GenerateGlobalProgramme generates a calendar view with most followed streamers
func (s *ProgrammeService) GenerateGlobalProgramme(ctx context.Context, week time.Time, limit int) (*ProgrammeCalendarView, error) {
	if limit <= 0 {
		limit = 10 // Default limit
	}

	weekStart := normalizeWeekStart(week)

	// Get all streamers
	allStreamers, err := s.streamerRepo.List(ctx, 10000)
	if err != nil {
		return nil, fmt.Errorf("failed to list streamers: %w", err)
	}

	// Get follower counts and sort by followers
	streamersWithCounts := make([]StreamerWithFollowers, 0, len(allStreamers))
	for _, streamer := range allStreamers {
		count, err := s.followRepo.GetFollowerCount(ctx, streamer.ID)
		if err != nil {
			count = 0
		}
		streamersWithCounts = append(streamersWithCounts, StreamerWithFollowers{
			Streamer:      streamer,
			FollowerCount: count,
		})
	}

	// Sort by follower count descending
	sort.Slice(streamersWithCounts, func(i, j int) bool {
		return streamersWithCounts[i].FollowerCount > streamersWithCounts[j].FollowerCount
	})

	// Take top N streamers
	var topStreamers []*domain.Streamer
	for i := 0; i < len(streamersWithCounts) && i < limit; i++ {
		topStreamers = append(topStreamers, streamersWithCounts[i].Streamer)
	}

	// Generate entries for top streamers
	var entries []domain.ProgrammeEntry
	for _, streamer := range topStreamers {
		heatmap, err := s.heatmapService.GenerateHeatmap(ctx, streamer.ID)
		if err != nil {
			continue
		}

		for dayOfWeek := 0; dayOfWeek < 7; dayOfWeek++ {
			dayProbability := heatmap.DaysOfWeek[dayOfWeek]
			if dayProbability > 0.1 {
				for hour := 0; hour < 24; hour++ {
					hourProbability := heatmap.Hours[hour]
					combinedProbability := dayProbability * hourProbability

					if combinedProbability > 0.05 {
						entries = append(entries, domain.ProgrammeEntry{
							StreamerID:  streamer.ID,
							DayOfWeek:   dayOfWeek,
							Hour:        hour,
							Probability: combinedProbability,
						})
					}
				}
			}
		}
	}

	return &ProgrammeCalendarView{
		Week:           weekStart,
		Streamers:      topStreamers,
		Entries:        entries,
		IsCustom:       false,
		IsGuestSession: false,
	}, nil
}

// GetProgrammeView determines which programme to show (custom vs global) and returns the calendar
func (s *ProgrammeService) GetProgrammeView(ctx context.Context, userID string, week time.Time) (*ProgrammeCalendarView, error) {
	// Try to get custom programme first
	if userID != "" {
		programme, err := s.GetCustomProgramme(ctx, userID)
		if err == nil && programme != nil && len(programme.StreamerIDs) > 0 {
			return s.GenerateCalendarFromProgramme(ctx, programme, week)
		}
	}

	// Fall back to global programme
	return s.GenerateGlobalProgramme(ctx, week, 10)
}

// GetStreamersRankedByFollowers returns streamers sorted by follower count (descending)
func (s *ProgrammeService) GetStreamersRankedByFollowers(ctx context.Context, limit int) ([]StreamerWithFollowers, error) {
	if limit <= 0 {
		limit = 10
	}

	allStreamers, err := s.streamerRepo.List(ctx, 10000)
	if err != nil {
		return nil, fmt.Errorf("failed to list streamers: %w", err)
	}

	streamersWithCounts := make([]StreamerWithFollowers, 0, len(allStreamers))
	for _, streamer := range allStreamers {
		count, err := s.followRepo.GetFollowerCount(ctx, streamer.ID)
		if err != nil {
			count = 0
		}
		streamersWithCounts = append(streamersWithCounts, StreamerWithFollowers{
			Streamer:      streamer,
			FollowerCount: count,
		})
	}

	// Sort by follower count descending
	sort.Slice(streamersWithCounts, func(i, j int) bool {
		return streamersWithCounts[i].FollowerCount > streamersWithCounts[j].FollowerCount
	})

	// Limit results
	if len(streamersWithCounts) > limit {
		streamersWithCounts = streamersWithCounts[:limit]
	}

	return streamersWithCounts, nil
}
