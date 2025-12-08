package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"who-live-when/internal/domain"
	"who-live-when/internal/repository"

	"github.com/google/uuid"
)

var (
	// ErrInsufficientData is returned when there's not enough historical data
	ErrInsufficientData = errors.New("insufficient historical data")
)

// heatmapService implements the HeatmapService interface
type heatmapService struct {
	activityRepo repository.ActivityRecordRepository
	heatmapRepo  repository.HeatmapRepository
}

// NewHeatmapService creates a new HeatmapService instance
func NewHeatmapService(
	activityRepo repository.ActivityRecordRepository,
	heatmapRepo repository.HeatmapRepository,
) domain.HeatmapService {
	return &heatmapService{
		activityRepo: activityRepo,
		heatmapRepo:  heatmapRepo,
	}
}

// GenerateHeatmap generates a heatmap for a streamer with weighted calculation.
// The weighting algorithm gives 80% weight to the last 3 months and 20% to older data
// (up to 1 year total). This ensures recent patterns have more influence while still
// considering historical trends.
func (s *heatmapService) GenerateHeatmap(ctx context.Context, streamerID string) (*domain.Heatmap, error) {
	if streamerID == "" {
		return nil, fmt.Errorf("streamer ID cannot be empty")
	}

	now := time.Now()
	oneYearAgo := now.AddDate(-1, 0, 0)
	threeMonthsAgo := now.AddDate(0, -3, 0)

	records, err := s.activityRepo.GetByStreamerID(ctx, streamerID, oneYearAgo)
	if err != nil {
		return nil, fmt.Errorf("failed to get activity records: %w", err)
	}

	if len(records) == 0 {
		return nil, ErrInsufficientData
	}

	// Partition records into two time windows for weighted calculation
	var recentRecords, olderRecords []*domain.ActivityRecord
	for _, record := range records {
		if record.StartTime.After(threeMonthsAgo) {
			recentRecords = append(recentRecords, record)
		} else {
			olderRecords = append(olderRecords, record)
		}
	}

	hours := s.calculateHourProbabilities(recentRecords, olderRecords)
	days := s.calculateDayProbabilities(recentRecords, olderRecords)

	heatmap := &domain.Heatmap{
		StreamerID:  streamerID,
		Hours:       hours,
		DaysOfWeek:  days,
		DataPoints:  len(records),
		GeneratedAt: now,
	}

	existing, err := s.heatmapRepo.GetByStreamerID(ctx, streamerID)
	if err != nil || existing == nil {
		if err := s.heatmapRepo.Create(ctx, heatmap); err != nil {
			return nil, fmt.Errorf("failed to create heatmap: %w", err)
		}
	} else {
		if err := s.heatmapRepo.Update(ctx, heatmap); err != nil {
			return nil, fmt.Errorf("failed to update heatmap: %w", err)
		}
	}

	return heatmap, nil
}

// calculateHourProbabilities computes the probability distribution across 24 hours
// using a weighted average of recent (80%) and older (20%) activity data.
// Each hour's probability represents the likelihood of the streamer going live during that hour.
func (s *heatmapService) calculateHourProbabilities(recent, older []*domain.ActivityRecord) [24]float64 {
	var hours [24]float64
	recentCounts := make([]int, 24)
	olderCounts := make([]int, 24)

	for _, record := range recent {
		hour := record.StartTime.Hour()
		recentCounts[hour]++
	}

	for _, record := range older {
		hour := record.StartTime.Hour()
		olderCounts[hour]++
	}

	recentTotal := len(recent)
	olderTotal := len(older)

	// Weighted probability formula: P(hour) = 0.8 * P_recent(hour) + 0.2 * P_older(hour)
	// where P_recent(hour) = count_recent(hour) / total_recent
	for i := 0; i < 24; i++ {
		var probability float64

		if recentTotal > 0 {
			recentProb := float64(recentCounts[i]) / float64(recentTotal)
			probability += recentProb * 0.8
		}

		if olderTotal > 0 {
			olderProb := float64(olderCounts[i]) / float64(olderTotal)
			probability += olderProb * 0.2
		}

		hours[i] = probability
	}

	return hours
}

// calculateDayProbabilities computes the probability distribution across 7 days of the week
// using the same weighted average approach as hour probabilities (80% recent, 20% older).
// Days are indexed 0-6 where 0=Sunday, 1=Monday, etc. (Go's time.Weekday convention).
func (s *heatmapService) calculateDayProbabilities(recent, older []*domain.ActivityRecord) [7]float64 {
	var days [7]float64
	recentCounts := make([]int, 7)
	olderCounts := make([]int, 7)

	for _, record := range recent {
		day := int(record.StartTime.Weekday())
		recentCounts[day]++
	}

	for _, record := range older {
		day := int(record.StartTime.Weekday())
		olderCounts[day]++
	}

	recentTotal := len(recent)
	olderTotal := len(older)

	for i := 0; i < 7; i++ {
		var probability float64

		if recentTotal > 0 {
			recentProb := float64(recentCounts[i]) / float64(recentTotal)
			probability += recentProb * 0.8
		}

		if olderTotal > 0 {
			olderProb := float64(olderCounts[i]) / float64(olderTotal)
			probability += olderProb * 0.2
		}

		days[i] = probability
	}

	return days
}

// RecordActivity stores an activity record for a streamer
func (s *heatmapService) RecordActivity(ctx context.Context, streamerID string, timestamp time.Time) error {
	if streamerID == "" {
		return fmt.Errorf("streamer ID cannot be empty")
	}

	record := &domain.ActivityRecord{
		ID:         uuid.New().String(),
		StreamerID: streamerID,
		StartTime:  timestamp,
		EndTime:    timestamp, // For now, just use the same timestamp
		Platform:   "",        // Platform can be set by caller if needed
		CreatedAt:  time.Now(),
	}

	if err := s.activityRepo.Create(ctx, record); err != nil {
		return fmt.Errorf("failed to record activity: %w", err)
	}

	return nil
}

// GetActivityStats retrieves statistical data about streamer activity
func (s *heatmapService) GetActivityStats(ctx context.Context, streamerID string) (*domain.ActivityStats, error) {
	if streamerID == "" {
		return nil, fmt.Errorf("streamer ID cannot be empty")
	}

	now := time.Now()
	oneYearAgo := now.AddDate(-1, 0, 0)

	records, err := s.activityRepo.GetByStreamerID(ctx, streamerID, oneYearAgo)
	if err != nil {
		return nil, fmt.Errorf("failed to get activity records: %w", err)
	}

	if len(records) == 0 {
		return &domain.ActivityStats{
			StreamerID:    streamerID,
			TotalSessions: 0,
		}, nil
	}

	// Calculate statistics
	stats := &domain.ActivityStats{
		StreamerID:    streamerID,
		TotalSessions: len(records),
	}

	// Calculate average session duration
	var totalDuration time.Duration
	hourCounts := make(map[int]int)
	dayCounts := make(map[int]int)

	for _, record := range records {
		duration := record.EndTime.Sub(record.StartTime)
		totalDuration += duration

		hour := record.StartTime.Hour()
		hourCounts[hour]++

		day := int(record.StartTime.Weekday())
		dayCounts[day]++

		// Track last active time
		if record.StartTime.After(stats.LastActive) {
			stats.LastActive = record.StartTime
		}
	}

	stats.AverageSessionDuration = totalDuration / time.Duration(len(records))

	// Find most active hour
	maxHourCount := 0
	for hour, count := range hourCounts {
		if count > maxHourCount {
			maxHourCount = count
			stats.MostActiveHour = hour
		}
	}

	// Find most active day
	maxDayCount := 0
	for day, count := range dayCounts {
		if count > maxDayCount {
			maxDayCount = count
			stats.MostActiveDay = day
		}
	}

	return stats, nil
}
