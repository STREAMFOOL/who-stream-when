package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"who-live-when/internal/domain"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: streamer-tracking-mvp, Property 20: Calendar Display Accuracy**
// **Validates: Requirements 9.1, 9.2**
// For any registered user viewing the calendar, all followed streamers with predictions
// should appear in their corresponding time slots
func TestProperty_CalendarDisplayAccuracy(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("all followed streamers with predictions appear in correct time slots", prop.ForAll(
		func(userID string, numStreamers int, weekTimestamp int64) bool {
			ctx := context.Background()
			week := time.Unix(weekTimestamp, 0)

			// Generate streamers
			streamers := make([]*domain.Streamer, numStreamers)

			for i := 0; i < numStreamers; i++ {
				streamers[i] = &domain.Streamer{
					ID:   fmt.Sprintf("streamer-%d", i),
					Name: fmt.Sprintf("Streamer %d", i),
				}
			}

			// Generate programme entries for each streamer
			var entries []domain.ProgrammeEntry

			for i, streamer := range streamers {
				// Generate 1-3 entries per streamer
				numEntries := 1 + (i % 3)

				for j := 0; j < numEntries; j++ {
					day := j % 7
					hour := (j * 3) % 24
					prob := 0.5 + float64(j)*0.1

					entries = append(entries, domain.ProgrammeEntry{
						StreamerID:  streamer.ID,
						DayOfWeek:   day,
						Hour:        hour,
						Probability: prob,
					})
				}
			}

			programme := &domain.TVProgramme{
				UserID:  userID,
				Week:    week,
				Entries: entries,
			}

			// Create streamer map
			streamerMap := make(map[string]*domain.Streamer)
			for _, streamer := range streamers {
				streamerMap[streamer.ID] = streamer
			}

			// Create mock services
			mockTVProgramme := &mockTVProgrammeService{
				generateProgrammeFunc: func(ctx context.Context, userID string, week time.Time) (*domain.TVProgramme, error) {
					return programme, nil
				},
			}

			mockUser := &mockUserService{
				getUserFollowsFunc: func(ctx context.Context, userID string) ([]*domain.Streamer, error) {
					return streamers, nil
				},
			}

			service := NewCalendarService(mockTVProgramme, mockUser)

			// Get calendar view
			view, err := service.GetCalendarView(ctx, userID, week)
			if err != nil {
				return false
			}

			// Property: All entries in the programme should appear in the time slot grid
			for _, entry := range programme.Entries {
				if entry.Hour < 0 || entry.Hour >= 24 || entry.DayOfWeek < 0 || entry.DayOfWeek >= 7 {
					continue
				}

				streamer := streamerMap[entry.StreamerID]
				if streamer == nil {
					continue
				}

				// Check if entry exists in the grid
				found := false
				for _, gridEntry := range view.TimeSlots[entry.Hour][entry.DayOfWeek] {
					if gridEntry.StreamerID == entry.StreamerID &&
						gridEntry.Hour == entry.Hour &&
						gridEntry.DayOfWeek == entry.DayOfWeek {
						found = true
						break
					}
				}

				if !found {
					return false
				}
			}

			// Property: All entries in the grid should correspond to programme entries
			for hour := 0; hour < 24; hour++ {
				for day := 0; day < 7; day++ {
					for _, gridEntry := range view.TimeSlots[hour][day] {
						// Check if this entry exists in the programme
						found := false
						for _, progEntry := range programme.Entries {
							if progEntry.StreamerID == gridEntry.StreamerID &&
								progEntry.Hour == hour &&
								progEntry.DayOfWeek == day {
								found = true
								break
							}
						}

						if !found {
							return false
						}
					}
				}
			}

			return true
		},
		gen.Identifier(),
		gen.IntRange(1, 10),
		gen.Int64Range(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC).Unix()),
	))

	properties.TestingRun(t)
}

// **Feature: streamer-tracking-mvp, Property 20: Calendar Display Accuracy (Streamer Names)**
// **Validates: Requirements 9.1, 9.2**
// For any calendar entry, the streamer name should match the streamer in the map
func TestProperty_CalendarDisplayStreamerNames(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("calendar entries contain correct streamer names", prop.ForAll(
		func(userID string, numStreamers int, weekTimestamp int64) bool {
			ctx := context.Background()
			week := time.Unix(weekTimestamp, 0)

			// Generate streamers with unique names
			streamers := make([]*domain.Streamer, numStreamers)
			for i := 0; i < numStreamers; i++ {
				streamers[i] = &domain.Streamer{
					ID:   fmt.Sprintf("streamer-%d", i),
					Name: fmt.Sprintf("Streamer %d", i),
				}
			}

			// Generate programme entries
			var entries []domain.ProgrammeEntry
			for i, streamer := range streamers {
				day := i % 7
				hour := (i * 2) % 24
				prob := 0.5 + float64(i)*0.05

				entries = append(entries, domain.ProgrammeEntry{
					StreamerID:  streamer.ID,
					DayOfWeek:   day,
					Hour:        hour,
					Probability: prob,
				})
			}

			programme := &domain.TVProgramme{
				UserID:  userID,
				Week:    week,
				Entries: entries,
			}

			// Create streamer map
			streamerMap := make(map[string]*domain.Streamer)
			for _, streamer := range streamers {
				streamerMap[streamer.ID] = streamer
			}

			// Create mock services
			mockTVProgramme := &mockTVProgrammeService{
				generateProgrammeFunc: func(ctx context.Context, userID string, week time.Time) (*domain.TVProgramme, error) {
					return programme, nil
				},
			}

			mockUser := &mockUserService{
				getUserFollowsFunc: func(ctx context.Context, userID string) ([]*domain.Streamer, error) {
					return streamers, nil
				},
			}

			service := NewCalendarService(mockTVProgramme, mockUser)

			// Get calendar view
			view, err := service.GetCalendarView(ctx, userID, week)
			if err != nil {
				return false
			}

			// Property: All calendar entries should have correct streamer names
			for hour := 0; hour < 24; hour++ {
				for day := 0; day < 7; day++ {
					for _, gridEntry := range view.TimeSlots[hour][day] {
						streamer := streamerMap[gridEntry.StreamerID]
						if streamer == nil {
							return false
						}

						if gridEntry.StreamerName != streamer.Name {
							return false
						}
					}
				}
			}

			return true
		},
		gen.Identifier(),
		gen.IntRange(1, 10),
		gen.Int64Range(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC).Unix()),
	))

	properties.TestingRun(t)
}

// **Feature: streamer-tracking-mvp, Property 21: Calendar Navigation Consistency**
// **Validates: Requirements 9.4**
// For any registered user navigating between weeks, the calendar should display
// the correct week's data and allow navigation to previous and next weeks
func TestProperty_CalendarNavigationConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("navigating weeks maintains consistency", prop.ForAll(
		func(userID string, weekTimestamp int64) bool {
			ctx := context.Background()
			initialWeek := time.Unix(weekTimestamp, 0)

			// Create a simple streamer and programme
			streamer := &domain.Streamer{
				ID:   "streamer-1",
				Name: "Test Streamer",
			}

			createProgramme := func(week time.Time) *domain.TVProgramme {
				return &domain.TVProgramme{
					UserID: userID,
					Week:   week,
					Entries: []domain.ProgrammeEntry{
						{StreamerID: "streamer-1", DayOfWeek: 0, Hour: 10, Probability: 0.8},
					},
				}
			}

			// Create mock services
			mockTVProgramme := &mockTVProgrammeService{
				generateProgrammeFunc: func(ctx context.Context, userID string, week time.Time) (*domain.TVProgramme, error) {
					return createProgramme(week), nil
				},
			}

			mockUser := &mockUserService{
				getUserFollowsFunc: func(ctx context.Context, userID string) ([]*domain.Streamer, error) {
					return []*domain.Streamer{streamer}, nil
				},
			}

			service := NewCalendarService(mockTVProgramme, mockUser)

			// Get initial calendar view
			initialView, err := service.GetCalendarView(ctx, userID, initialWeek)
			if err != nil {
				return false
			}

			// Property 1: Week should be normalized to Sunday
			if initialView.Week.Weekday() != time.Sunday {
				return false
			}

			// Property 2: PrevWeek should be exactly 7 days before Week
			expectedPrevWeek := initialView.Week.AddDate(0, 0, -7)
			if !initialView.PrevWeek.Equal(expectedPrevWeek) {
				return false
			}

			// Property 3: NextWeek should be exactly 7 days after Week
			expectedNextWeek := initialView.Week.AddDate(0, 0, 7)
			if !initialView.NextWeek.Equal(expectedNextWeek) {
				return false
			}

			// Property 4: Navigating to next week and back should return to original week
			nextWeek := service.NavigateWeek(initialView.Week, "next")
			backToOriginal := service.NavigateWeek(nextWeek, "prev")
			if !backToOriginal.Equal(initialView.Week) {
				return false
			}

			// Property 5: Navigating to previous week and back should return to original week
			prevWeek := service.NavigateWeek(initialView.Week, "prev")
			backToOriginalFromPrev := service.NavigateWeek(prevWeek, "next")
			if !backToOriginalFromPrev.Equal(initialView.Week) {
				return false
			}

			// Property 6: Getting calendar view for next week should have correct week value
			nextView, err := service.GetCalendarView(ctx, userID, nextWeek)
			if err != nil {
				return false
			}

			if !nextView.Week.Equal(nextWeek) {
				return false
			}

			// Property 7: Next week's previous week should be the original week
			if !nextView.PrevWeek.Equal(initialView.Week) {
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.Int64Range(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC).Unix()),
	))

	properties.TestingRun(t)
}

// **Feature: streamer-tracking-mvp, Property 21: Calendar Navigation Consistency (Week Boundaries)**
// **Validates: Requirements 9.4**
// For any week, navigation should always land on Sunday at 00:00:00
func TestProperty_CalendarNavigationWeekBoundaries(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("navigation always lands on week boundaries", prop.ForAll(
		func(weekTimestamp int64) bool {
			initialWeek := time.Unix(weekTimestamp, 0)

			service := &CalendarService{}

			// Navigate in both directions
			nextWeek := service.NavigateWeek(initialWeek, "next")
			prevWeek := service.NavigateWeek(initialWeek, "prev")

			// Property: All navigated weeks should be on Sunday at 00:00:00
			if nextWeek.Weekday() != time.Sunday {
				return false
			}

			if prevWeek.Weekday() != time.Sunday {
				return false
			}

			if nextWeek.Hour() != 0 || nextWeek.Minute() != 0 || nextWeek.Second() != 0 {
				return false
			}

			if prevWeek.Hour() != 0 || prevWeek.Minute() != 0 || prevWeek.Second() != 0 {
				return false
			}

			// Property: Next week should be exactly 7 days after normalized current week
			normalizedCurrent := normalizeWeekStart(initialWeek)
			expectedNext := normalizedCurrent.AddDate(0, 0, 7)
			if !nextWeek.Equal(expectedNext) {
				return false
			}

			// Property: Previous week should be exactly 7 days before normalized current week
			return prevWeek.Equal(normalizedCurrent.AddDate(0, 0, -7))
		},
		gen.Int64Range(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC).Unix()),
	))

	properties.TestingRun(t)
}
