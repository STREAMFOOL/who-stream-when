package service

import (
	"context"
	"testing"
	"time"

	"who-live-when/internal/domain"
	"who-live-when/internal/repository/sqlite"
)

// TestCalendarView_Integration tests the calendar view with real database
func TestCalendarView_Integration(t *testing.T) {
	// Create temporary database
	tmpFile := t.TempDir() + "/test.db"

	db, err := sqlite.NewDB(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := sqlite.Migrate(db.DB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	streamerRepo := sqlite.NewStreamerRepository(db)
	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)

	// Initialize services
	streamerService := NewStreamerService(streamerRepo)
	heatmapService := NewHeatmapService(activityRepo, heatmapRepo)
	userService := NewUserService(userRepo, followRepo, activityRepo)
	tvProgrammeService := NewTVProgrammeService(heatmapService, userRepo, followRepo, streamerRepo, activityRepo)
	calendarService := NewCalendarService(tvProgrammeService, userService)

	ctx := context.Background()

	// Create test user
	user, err := userService.CreateUser(ctx, "google-123", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create test streamers
	streamer1 := &domain.Streamer{
		ID:        "streamer-1",
		Name:      "Test Streamer 1",
		Handles:   map[string]string{"youtube": "handle1"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	streamer2 := &domain.Streamer{
		ID:        "streamer-2",
		Name:      "Test Streamer 2",
		Handles:   map[string]string{"twitch": "handle2"},
		Platforms: []string{"twitch"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := streamerService.AddStreamer(ctx, streamer1); err != nil {
		t.Fatalf("Failed to add streamer 1: %v", err)
	}

	if err := streamerService.AddStreamer(ctx, streamer2); err != nil {
		t.Fatalf("Failed to add streamer 2: %v", err)
	}

	// Follow streamers
	if err := userService.FollowStreamer(ctx, user.ID, streamer1.ID); err != nil {
		t.Fatalf("Failed to follow streamer 1: %v", err)
	}

	if err := userService.FollowStreamer(ctx, user.ID, streamer2.ID); err != nil {
		t.Fatalf("Failed to follow streamer 2: %v", err)
	}

	// Create activity records for heatmap generation
	now := time.Now()
	for i := 0; i < 10; i++ {
		activityTime := now.AddDate(0, 0, -i)
		if err := heatmapService.RecordActivity(ctx, streamer1.ID, activityTime); err != nil {
			t.Fatalf("Failed to record activity: %v", err)
		}
	}

	// Test: Get calendar view for current week
	t.Run("calendar displays correct week", func(t *testing.T) {
		week := time.Now()
		view, err := calendarService.GetCalendarView(ctx, user.ID, week)
		if err != nil {
			t.Fatalf("Failed to get calendar view: %v", err)
		}

		if view == nil {
			t.Fatal("Expected non-nil calendar view")
		}

		// Check week is normalized to Sunday
		if view.Week.Weekday() != time.Sunday {
			t.Errorf("Expected week to start on Sunday, got %v", view.Week.Weekday())
		}

		// Check streamer map contains followed streamers
		if len(view.StreamerMap) != 2 {
			t.Errorf("Expected 2 streamers in map, got %d", len(view.StreamerMap))
		}

		if view.StreamerMap[streamer1.ID] == nil {
			t.Error("Expected streamer 1 in map")
		}

		if view.StreamerMap[streamer2.ID] == nil {
			t.Error("Expected streamer 2 in map")
		}
	})

	// Test: Navigate between weeks
	t.Run("navigation between weeks", func(t *testing.T) {
		currentWeek := time.Now()
		currentView, err := calendarService.GetCalendarView(ctx, user.ID, currentWeek)
		if err != nil {
			t.Fatalf("Failed to get current calendar view: %v", err)
		}

		// Navigate to next week
		nextWeek := calendarService.NavigateWeek(currentView.Week, "next")
		nextView, err := calendarService.GetCalendarView(ctx, user.ID, nextWeek)
		if err != nil {
			t.Fatalf("Failed to get next week calendar view: %v", err)
		}

		// Check next week is 7 days after current week
		expectedNextWeek := currentView.Week.AddDate(0, 0, 7)
		if !nextView.Week.Equal(expectedNextWeek) {
			t.Errorf("Expected next week to be %v, got %v", expectedNextWeek, nextView.Week)
		}

		// Navigate to previous week
		prevWeek := calendarService.NavigateWeek(currentView.Week, "prev")
		prevView, err := calendarService.GetCalendarView(ctx, user.ID, prevWeek)
		if err != nil {
			t.Fatalf("Failed to get previous week calendar view: %v", err)
		}

		// Check previous week is 7 days before current week
		expectedPrevWeek := currentView.Week.AddDate(0, 0, -7)
		if !prevView.Week.Equal(expectedPrevWeek) {
			t.Errorf("Expected previous week to be %v, got %v", expectedPrevWeek, prevView.Week)
		}
	})

	// Test: Predicted times appear in correct slots
	t.Run("predicted times appear in correct slots", func(t *testing.T) {
		week := time.Now()
		view, err := calendarService.GetCalendarView(ctx, user.ID, week)
		if err != nil {
			t.Fatalf("Failed to get calendar view: %v", err)
		}

		// Check that time slots grid is properly initialized
		if len(view.TimeSlots) != 24 {
			t.Errorf("Expected 24 hours in time slots, got %d", len(view.TimeSlots))
		}

		// Check that each hour has 7 days
		for hour := 0; hour < 24; hour++ {
			if len(view.TimeSlots[hour]) != 7 {
				t.Errorf("Expected 7 days for hour %d, got %d", hour, len(view.TimeSlots[hour]))
			}
		}

		// If there are programme entries, verify they appear in the grid
		if len(view.Programme.Entries) > 0 {
			for _, entry := range view.Programme.Entries {
				if entry.Hour < 0 || entry.Hour >= 24 || entry.DayOfWeek < 0 || entry.DayOfWeek >= 7 {
					continue
				}

				// Check if entry exists in the grid
				found := false
				for _, gridEntry := range view.TimeSlots[entry.Hour][entry.DayOfWeek] {
					if gridEntry.StreamerID == entry.StreamerID {
						found = true

						// Verify entry details
						if gridEntry.Hour != entry.Hour {
							t.Errorf("Expected hour %d, got %d", entry.Hour, gridEntry.Hour)
						}

						if gridEntry.DayOfWeek != entry.DayOfWeek {
							t.Errorf("Expected day %d, got %d", entry.DayOfWeek, gridEntry.DayOfWeek)
						}

						if gridEntry.Probability != entry.Probability {
							t.Errorf("Expected probability %f, got %f", entry.Probability, gridEntry.Probability)
						}

						break
					}
				}

				if !found {
					t.Errorf("Programme entry for streamer %s at day %d, hour %d not found in grid",
						entry.StreamerID, entry.DayOfWeek, entry.Hour)
				}
			}
		}
	})
}
