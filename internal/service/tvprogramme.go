package service

import (
	"context"
	"fmt"
	"time"

	"github.com/user/who-live-when/internal/domain"
	"github.com/user/who-live-when/internal/repository"
)

// tvProgrammeService implements the TVProgrammeService interface
type tvProgrammeService struct {
	heatmapService domain.HeatmapService
	userRepo       repository.UserRepository
	followRepo     repository.FollowRepository
	streamerRepo   repository.StreamerRepository
	activityRepo   repository.ActivityRecordRepository
}

// NewTVProgrammeService creates a new TVProgrammeService instance
func NewTVProgrammeService(
	heatmapService domain.HeatmapService,
	userRepo repository.UserRepository,
	followRepo repository.FollowRepository,
	streamerRepo repository.StreamerRepository,
	activityRepo repository.ActivityRecordRepository,
) domain.TVProgrammeService {
	return &tvProgrammeService{
		heatmapService: heatmapService,
		userRepo:       userRepo,
		followRepo:     followRepo,
		streamerRepo:   streamerRepo,
		activityRepo:   activityRepo,
	}
}

// GenerateProgramme creates a weekly schedule for a user's followed streamers
func (s *tvProgrammeService) GenerateProgramme(ctx context.Context, userID string, week time.Time) (*domain.TVProgramme, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	// Verify user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get user's followed streamers
	streamers, err := s.followRepo.GetFollowedStreamers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get followed streamers: %w", err)
	}

	// Normalize week to start of week (Sunday)
	weekStart := normalizeToWeekStart(week)

	var entries []domain.ProgrammeEntry

	// Generate predictions for each followed streamer
	for _, streamer := range streamers {
		// Get heatmap for the streamer
		heatmap, err := s.heatmapService.GenerateHeatmap(ctx, streamer.ID)
		if err != nil {
			// Skip streamers with insufficient data
			continue
		}

		// Generate entries for each day of the week
		for dayOfWeek := 0; dayOfWeek < 7; dayOfWeek++ {
			// Get the most likely hours for this day
			dayProbability := heatmap.DaysOfWeek[dayOfWeek]

			// Only include days with reasonable probability
			if dayProbability > 0.1 {
				// Find the most likely hours for this day
				for hour := 0; hour < 24; hour++ {
					hourProbability := heatmap.Hours[hour]

					// Combine day and hour probabilities
					combinedProbability := dayProbability * hourProbability

					// Only include time slots with reasonable probability
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

	programme := &domain.TVProgramme{
		UserID:      userID,
		Week:        weekStart,
		Entries:     entries,
		GeneratedAt: time.Now(),
	}

	return programme, nil
}

// GetPredictedLiveTime returns the predicted live time for a streamer on a specific day
func (s *tvProgrammeService) GetPredictedLiveTime(ctx context.Context, streamerID string, dayOfWeek int) (*domain.PredictedTime, error) {
	if streamerID == "" {
		return nil, fmt.Errorf("streamer ID cannot be empty")
	}

	if dayOfWeek < 0 || dayOfWeek > 6 {
		return nil, fmt.Errorf("day of week must be between 0 and 6")
	}

	// Get heatmap for the streamer
	heatmap, err := s.heatmapService.GenerateHeatmap(ctx, streamerID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate heatmap: %w", err)
	}

	// Get day probability
	dayProbability := heatmap.DaysOfWeek[dayOfWeek]

	// Find the most likely hour for this day
	maxProbability := 0.0
	mostLikelyHour := 0

	for hour := 0; hour < 24; hour++ {
		hourProbability := heatmap.Hours[hour]
		combinedProbability := dayProbability * hourProbability

		if combinedProbability > maxProbability {
			maxProbability = combinedProbability
			mostLikelyHour = hour
		}
	}

	return &domain.PredictedTime{
		DayOfWeek:   dayOfWeek,
		Hour:        mostLikelyHour,
		Probability: maxProbability,
	}, nil
}

// GetMostViewedStreamers returns the most viewed streamers based on follower count
func (s *tvProgrammeService) GetMostViewedStreamers(ctx context.Context, limit int) ([]*domain.Streamer, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than 0")
	}

	// Get all streamers (use a large limit to get all)
	allStreamers, err := s.streamerRepo.List(ctx, 10000)
	if err != nil {
		return nil, fmt.Errorf("failed to list streamers: %w", err)
	}

	// Create a map of streamer ID to follower count
	type streamerWithCount struct {
		streamer *domain.Streamer
		count    int
	}

	var streamersWithCounts []streamerWithCount

	for _, streamer := range allStreamers {
		count, err := s.followRepo.GetFollowerCount(ctx, streamer.ID)
		if err != nil {
			// Skip streamers with errors
			continue
		}

		streamersWithCounts = append(streamersWithCounts, streamerWithCount{
			streamer: streamer,
			count:    count,
		})
	}

	// Sort by follower count (descending)
	for i := 0; i < len(streamersWithCounts); i++ {
		for j := i + 1; j < len(streamersWithCounts); j++ {
			if streamersWithCounts[j].count > streamersWithCounts[i].count {
				streamersWithCounts[i], streamersWithCounts[j] = streamersWithCounts[j], streamersWithCounts[i]
			}
		}
	}

	// Extract top streamers
	var result []*domain.Streamer
	for i := 0; i < len(streamersWithCounts) && i < limit; i++ {
		result = append(result, streamersWithCounts[i].streamer)
	}

	return result, nil
}

// GetDefaultWeekView returns the default week view for the home page
func (s *tvProgrammeService) GetDefaultWeekView(ctx context.Context) (*domain.WeekView, error) {
	// Get current week
	now := time.Now()
	weekStart := normalizeToWeekStart(now)

	// Get most viewed streamers (limit to 10 for home page)
	streamers, err := s.GetMostViewedStreamers(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get most viewed streamers: %w", err)
	}

	var entries []domain.ProgrammeEntry
	viewCount := make(map[string]int)

	// Generate predictions for each streamer
	for _, streamer := range streamers {
		// Get follower count for view count
		count, err := s.followRepo.GetFollowerCount(ctx, streamer.ID)
		if err == nil {
			viewCount[streamer.ID] = count
		}

		// Get heatmap for the streamer
		heatmap, err := s.heatmapService.GenerateHeatmap(ctx, streamer.ID)
		if err != nil {
			// Skip streamers with insufficient data
			continue
		}

		// Generate entries for each day of the week
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

	weekView := &domain.WeekView{
		Week:      weekStart,
		Streamers: streamers,
		Entries:   entries,
		ViewCount: viewCount,
	}

	return weekView, nil
}

// normalizeToWeekStart returns the start of the week (Sunday at 00:00:00)
func normalizeToWeekStart(t time.Time) time.Time {
	// Get the weekday (0 = Sunday, 1 = Monday, etc.)
	weekday := int(t.Weekday())

	// Calculate days to subtract to get to Sunday
	daysToSubtract := weekday

	// Go back to Sunday
	sunday := t.AddDate(0, 0, -daysToSubtract)

	// Set time to 00:00:00
	return time.Date(sunday.Year(), sunday.Month(), sunday.Day(), 0, 0, 0, 0, sunday.Location())
}
