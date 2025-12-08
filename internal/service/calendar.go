package service

import (
	"context"
	"time"

	"who-live-when/internal/domain"
)

// CalendarService handles calendar view rendering and navigation
type CalendarService struct {
	tvProgrammeService domain.TVProgrammeService
	userService        domain.UserService
}

// NewCalendarService creates a new CalendarService instance
func NewCalendarService(
	tvProgrammeService domain.TVProgrammeService,
	userService domain.UserService,
) *CalendarService {
	return &CalendarService{
		tvProgrammeService: tvProgrammeService,
		userService:        userService,
	}
}

// CalendarView represents a calendar view with all necessary data
type CalendarView struct {
	Week        time.Time
	PrevWeek    time.Time
	NextWeek    time.Time
	Programme   *domain.TVProgramme
	StreamerMap map[string]*domain.Streamer
	TimeSlots   [][7][]*CalendarEntry // [hour][day]entries
}

// CalendarEntry represents a single entry in the calendar grid
type CalendarEntry struct {
	StreamerID   string
	StreamerName string
	Probability  float64
	Hour         int
	DayOfWeek    int
}

// GetCalendarView generates a complete calendar view for a user and week
func (s *CalendarService) GetCalendarView(ctx context.Context, userID string, week time.Time) (*CalendarView, error) {
	// Generate TV programme for the user
	programme, err := s.tvProgrammeService.GenerateProgramme(ctx, userID, week)
	if err != nil {
		return nil, err
	}

	// Get followed streamers for display
	followedStreamers, err := s.userService.GetUserFollows(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Create streamer map for quick lookup
	streamerMap := make(map[string]*domain.Streamer)
	for _, streamer := range followedStreamers {
		streamerMap[streamer.ID] = streamer
	}

	// Normalize week to start of week
	weekStart := normalizeWeekStart(week)
	prevWeek := weekStart.AddDate(0, 0, -7)
	nextWeek := weekStart.AddDate(0, 0, 7)

	// Create time slot grid
	timeSlots := s.buildTimeSlotGrid(programme, streamerMap)

	return &CalendarView{
		Week:        weekStart,
		PrevWeek:    prevWeek,
		NextWeek:    nextWeek,
		Programme:   programme,
		StreamerMap: streamerMap,
		TimeSlots:   timeSlots,
	}, nil
}

// buildTimeSlotGrid creates a 2D grid of calendar entries organized by hour and day
func (s *CalendarService) buildTimeSlotGrid(programme *domain.TVProgramme, streamerMap map[string]*domain.Streamer) [][7][]*CalendarEntry {
	// Initialize 24-hour x 7-day grid
	timeSlots := make([][7][]*CalendarEntry, 24)
	for hour := range timeSlots {
		timeSlots[hour] = [7][]*CalendarEntry{}
		for day := range timeSlots[hour] {
			timeSlots[hour][day] = []*CalendarEntry{}
		}
	}

	// Populate grid with programme entries
	for _, entry := range programme.Entries {
		if entry.Hour < 0 || entry.Hour >= 24 || entry.DayOfWeek < 0 || entry.DayOfWeek >= 7 {
			continue
		}

		streamer := streamerMap[entry.StreamerID]
		if streamer == nil {
			continue
		}

		calEntry := &CalendarEntry{
			StreamerID:   entry.StreamerID,
			StreamerName: streamer.Name,
			Probability:  entry.Probability,
			Hour:         entry.Hour,
			DayOfWeek:    entry.DayOfWeek,
		}

		timeSlots[entry.Hour][entry.DayOfWeek] = append(timeSlots[entry.Hour][entry.DayOfWeek], calEntry)
	}

	return timeSlots
}

// NavigateWeek returns the week date for navigation (previous or next)
func (s *CalendarService) NavigateWeek(currentWeek time.Time, direction string) time.Time {
	weekStart := normalizeWeekStart(currentWeek)

	switch direction {
	case "prev", "previous":
		return weekStart.AddDate(0, 0, -7)
	case "next":
		return weekStart.AddDate(0, 0, 7)
	default:
		return weekStart
	}
}

// normalizeWeekStart returns the start of the week (Sunday at 00:00:00)
func normalizeWeekStart(t time.Time) time.Time {
	weekday := int(t.Weekday())
	daysToSubtract := weekday
	sunday := t.AddDate(0, 0, -daysToSubtract)
	return time.Date(sunday.Year(), sunday.Month(), sunday.Day(), 0, 0, 0, 0, sunday.Location())
}
