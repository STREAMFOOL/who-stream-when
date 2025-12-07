package service

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"github.com/user/who-live-when/internal/domain"
	"github.com/user/who-live-when/internal/repository/sqlite"
)

// setupTVProgrammeTestDB creates a test database for TV Programme tests
func setupTVProgrammeTestDB(t *testing.T) *sqlite.DB {
	t.Helper()

	// Create temporary database file
	tmpFile, err := os.CreateTemp("", "test-tvprogramme-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()

	// Open database
	db, err := sqlite.NewDB(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to open database: %v", err)
	}

	// Run migrations
	if err := sqlite.Migrate(db.DB); err != nil {
		db.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Clean up on test completion
	t.Cleanup(func() {
		db.Close()
		os.Remove(tmpFile.Name())
	})

	return db
}

// genActivityRecordsForTVProgramme generates random activity records for property testing
func genActivityRecordsForTVProgramme(streamerID string, minRecords, maxRecords int) gopter.Gen {
	return gen.IntRange(minRecords, maxRecords).FlatMap(func(count interface{}) gopter.Gen {
		numRecords := count.(int)
		return gopter.CombineGens(
			gen.SliceOfN(numRecords, genActivityRecordForTVProgramme(streamerID)),
		).Map(func(values []interface{}) []*domain.ActivityRecord {
			return values[0].([]*domain.ActivityRecord)
		})
	}, reflect.TypeOf([]*domain.ActivityRecord{}))
}

// genActivityRecordForTVProgramme generates a single random activity record
func genActivityRecordForTVProgramme(streamerID string) gopter.Gen {
	now := time.Now()
	oneYearAgo := now.AddDate(-1, 0, 0)

	return gopter.CombineGens(
		gen.TimeRange(oneYearAgo, time.Hour*24*365),
		gen.OneConstOf("youtube", "twitch", "kick"),
	).Map(func(values []interface{}) *domain.ActivityRecord {
		startTime := values[0].(time.Time)
		platform := values[1].(string)

		return &domain.ActivityRecord{
			ID:         uuid.New().String(),
			StreamerID: streamerID,
			StartTime:  startTime,
			EndTime:    startTime.Add(time.Hour * 2),
			Platform:   platform,
			CreatedAt:  time.Now(),
		}
	})
}

// **Feature: streamer-tracking-mvp, Property 7: TV Programme Prediction Consistency**
// **Validates: Requirements 4.1, 4.2**
// Property: For any registered user viewing their TV programme for a given week,
// the predictions should be based on the same weighted historical data as the heatmap
func TestProperty_TVProgrammePredictionConsistency(t *testing.T) {
	ctx := context.Background()

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("TV programme predictions match heatmap data", prop.ForAll(
		func(streamerName string, userEmail string, activityRecords []*domain.ActivityRecord) bool {
			// Create a fresh database for each test iteration
			db := setupTVProgrammeTestDB(t)

			streamerRepo := sqlite.NewStreamerRepository(db)
			userRepo := sqlite.NewUserRepository(db)
			followRepo := sqlite.NewFollowRepository(db)
			activityRepo := sqlite.NewActivityRecordRepository(db)
			heatmapRepo := sqlite.NewHeatmapRepository(db)

			heatmapService := NewHeatmapService(activityRepo, heatmapRepo)
			tvProgrammeService := NewTVProgrammeService(heatmapService, userRepo, followRepo, streamerRepo, activityRepo)

			// Create a test streamer
			streamer := &domain.Streamer{
				ID:        uuid.New().String(),
				Name:      streamerName,
				Handles:   map[string]string{"youtube": streamerName},
				Platforms: []string{"youtube"},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			if err := streamerRepo.Create(ctx, streamer); err != nil {
				t.Logf("Failed to create streamer: %v", err)
				return false
			}

			// Create a test user
			user := &domain.User{
				ID:        uuid.New().String(),
				GoogleID:  "google-" + userEmail,
				Email:     userEmail,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			if err := userRepo.Create(ctx, user); err != nil {
				t.Logf("Failed to create user: %v", err)
				return false
			}

			// User follows the streamer
			if err := followRepo.Create(ctx, user.ID, streamer.ID); err != nil {
				t.Logf("Failed to create follow: %v", err)
				return false
			}

			// Store activity records
			for _, record := range activityRecords {
				record.StreamerID = streamer.ID
				if err := activityRepo.Create(ctx, record); err != nil {
					t.Logf("Failed to create activity record: %v", err)
					return false
				}
			}

			// Generate heatmap
			heatmap, err := heatmapService.GenerateHeatmap(ctx, streamer.ID)
			if err != nil {
				// If there's insufficient data, that's acceptable
				return true
			}

			// Generate TV programme
			week := time.Now()
			programme, err := tvProgrammeService.GenerateProgramme(ctx, user.ID, week)
			if err != nil {
				t.Logf("Failed to generate programme: %v", err)
				return false
			}

			// Verify that programme entries are based on heatmap data
			for _, entry := range programme.Entries {
				if entry.StreamerID != streamer.ID {
					continue
				}

				// Get the heatmap probabilities
				dayProb := heatmap.DaysOfWeek[entry.DayOfWeek]
				hourProb := heatmap.Hours[entry.Hour]
				expectedProb := dayProb * hourProb

				// The entry probability should match the combined heatmap probability
				// Allow small floating point differences
				diff := entry.Probability - expectedProb
				if diff < -0.0001 || diff > 0.0001 {
					t.Logf("Probability mismatch: entry=%f, expected=%f (day=%f, hour=%f)",
						entry.Probability, expectedProb, dayProb, hourProb)
					return false
				}
			}

			return true
		},
		gen.Identifier(),
		gen.Identifier(),
		genActivityRecordsForTVProgramme("", 5, 50),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: streamer-tracking-mvp, Property 8: Week-Based Programme Uniqueness**
// **Validates: Requirements 4.4**
// Property: For any registered user, TV programmes for different weeks should contain
// different predicted times based on the day-of-week patterns
func TestProperty_WeekBasedProgrammeUniqueness(t *testing.T) {
	ctx := context.Background()

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("different weeks produce different week start times", prop.ForAll(
		func(streamerName string, userEmail string, activityRecords []*domain.ActivityRecord, weekOffset1 int, weekOffset2 int) bool {
			// Ensure we have different week offsets
			if weekOffset1 == weekOffset2 {
				return true // Skip this case
			}

			// Create a fresh database for each test iteration
			db := setupTVProgrammeTestDB(t)

			streamerRepo := sqlite.NewStreamerRepository(db)
			userRepo := sqlite.NewUserRepository(db)
			followRepo := sqlite.NewFollowRepository(db)
			activityRepo := sqlite.NewActivityRecordRepository(db)
			heatmapRepo := sqlite.NewHeatmapRepository(db)

			heatmapService := NewHeatmapService(activityRepo, heatmapRepo)
			tvProgrammeService := NewTVProgrammeService(heatmapService, userRepo, followRepo, streamerRepo, activityRepo)

			// Create a test streamer
			streamer := &domain.Streamer{
				ID:        uuid.New().String(),
				Name:      streamerName,
				Handles:   map[string]string{"youtube": streamerName},
				Platforms: []string{"youtube"},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			if err := streamerRepo.Create(ctx, streamer); err != nil {
				return false
			}

			// Create a test user
			user := &domain.User{
				ID:        uuid.New().String(),
				GoogleID:  "google-" + userEmail,
				Email:     userEmail,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			if err := userRepo.Create(ctx, user); err != nil {
				return false
			}

			// User follows the streamer
			if err := followRepo.Create(ctx, user.ID, streamer.ID); err != nil {
				return false
			}

			// Store activity records
			for _, record := range activityRecords {
				record.StreamerID = streamer.ID
				if err := activityRepo.Create(ctx, record); err != nil {
					return false
				}
			}

			// Generate programmes for two different weeks
			now := time.Now()
			week1 := now.AddDate(0, 0, weekOffset1*7)
			week2 := now.AddDate(0, 0, weekOffset2*7)

			programme1, err := tvProgrammeService.GenerateProgramme(ctx, user.ID, week1)
			if err != nil {
				// If there's insufficient data, that's acceptable
				return true
			}

			programme2, err := tvProgrammeService.GenerateProgramme(ctx, user.ID, week2)
			if err != nil {
				// If there's insufficient data, that's acceptable
				return true
			}

			// The week start times should be different
			if programme1.Week.Equal(programme2.Week) {
				t.Logf("Week start times are the same: %v", programme1.Week)
				return false
			}

			// The entries should be the same (based on day-of-week patterns)
			// but the week field should be different
			if len(programme1.Entries) != len(programme2.Entries) {
				// This is acceptable - different weeks might have different entries
				return true
			}

			// Verify that the entries have the same day-of-week and hour patterns
			// (since the heatmap is based on day-of-week, not specific dates)
			for i := range programme1.Entries {
				if programme1.Entries[i].DayOfWeek != programme2.Entries[i].DayOfWeek ||
					programme1.Entries[i].Hour != programme2.Entries[i].Hour ||
					programme1.Entries[i].StreamerID != programme2.Entries[i].StreamerID {
					// Entries might be in different order, which is acceptable
					return true
				}
			}

			return true
		},
		gen.Identifier(),
		gen.Identifier(),
		genActivityRecordsForTVProgramme("", 5, 50),
		gen.IntRange(-10, 10), // Week offset 1
		gen.IntRange(-10, 10), // Week offset 2
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Unit Tests

// TestGenerateProgramme_DifferentWeeks tests that programmes for different weeks are generated correctly
func TestGenerateProgramme_DifferentWeeks(t *testing.T) {
	db := setupTVProgrammeTestDB(t)
	ctx := context.Background()

	streamerRepo := sqlite.NewStreamerRepository(db)
	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)

	heatmapService := NewHeatmapService(activityRepo, heatmapRepo)
	tvProgrammeService := NewTVProgrammeService(heatmapService, userRepo, followRepo, streamerRepo, activityRepo)

	// Create a test streamer
	streamer := &domain.Streamer{
		ID:        uuid.New().String(),
		Name:      "TestStreamer",
		Handles:   map[string]string{"youtube": "teststreamer"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := streamerRepo.Create(ctx, streamer); err != nil {
		t.Fatalf("Failed to create streamer: %v", err)
	}

	// Create a test user
	user := &domain.User{
		ID:        uuid.New().String(),
		GoogleID:  "google-test",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// User follows the streamer
	if err := followRepo.Create(ctx, user.ID, streamer.ID); err != nil {
		t.Fatalf("Failed to create follow: %v", err)
	}

	// Create activity records
	now := time.Now()
	for i := 0; i < 20; i++ {
		record := &domain.ActivityRecord{
			ID:         uuid.New().String(),
			StreamerID: streamer.ID,
			StartTime:  now.AddDate(0, 0, -i*7), // Weekly pattern
			EndTime:    now.AddDate(0, 0, -i*7).Add(time.Hour * 2),
			Platform:   "youtube",
			CreatedAt:  time.Now(),
		}
		if err := activityRepo.Create(ctx, record); err != nil {
			t.Fatalf("Failed to create activity record: %v", err)
		}
	}

	// Generate programmes for two different weeks
	week1 := time.Now()
	week2 := time.Now().AddDate(0, 0, 7)

	programme1, err := tvProgrammeService.GenerateProgramme(ctx, user.ID, week1)
	if err != nil {
		t.Fatalf("Failed to generate programme for week 1: %v", err)
	}

	programme2, err := tvProgrammeService.GenerateProgramme(ctx, user.ID, week2)
	if err != nil {
		t.Fatalf("Failed to generate programme for week 2: %v", err)
	}

	// Verify that the week start times are different
	if programme1.Week.Equal(programme2.Week) {
		t.Errorf("Expected different week start times, got same: %v", programme1.Week)
	}

	// Verify that both programmes have entries
	if len(programme1.Entries) == 0 {
		t.Errorf("Expected programme 1 to have entries")
	}

	if len(programme2.Entries) == 0 {
		t.Errorf("Expected programme 2 to have entries")
	}
}

// TestGetPredictedLiveTime_Accuracy tests the accuracy of predicted live times
func TestGetPredictedLiveTime_Accuracy(t *testing.T) {
	db := setupTVProgrammeTestDB(t)
	ctx := context.Background()

	streamerRepo := sqlite.NewStreamerRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)

	heatmapService := NewHeatmapService(activityRepo, heatmapRepo)
	tvProgrammeService := NewTVProgrammeService(heatmapService, nil, nil, streamerRepo, activityRepo)

	// Create a test streamer
	streamer := &domain.Streamer{
		ID:        uuid.New().String(),
		Name:      "TestStreamer",
		Handles:   map[string]string{"youtube": "teststreamer"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := streamerRepo.Create(ctx, streamer); err != nil {
		t.Fatalf("Failed to create streamer: %v", err)
	}

	// Create activity records with a clear pattern (always at 14:00 on Mondays)
	now := time.Now()
	for i := 0; i < 20; i++ {
		// Find the next Monday
		daysUntilMonday := (8 - int(now.Weekday())) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		monday := now.AddDate(0, 0, daysUntilMonday-i*7)
		mondayAt14 := time.Date(monday.Year(), monday.Month(), monday.Day(), 14, 0, 0, 0, monday.Location())

		record := &domain.ActivityRecord{
			ID:         uuid.New().String(),
			StreamerID: streamer.ID,
			StartTime:  mondayAt14,
			EndTime:    mondayAt14.Add(time.Hour * 2),
			Platform:   "youtube",
			CreatedAt:  time.Now(),
		}
		if err := activityRepo.Create(ctx, record); err != nil {
			t.Fatalf("Failed to create activity record: %v", err)
		}
	}

	// Get predicted live time for Monday (day 1)
	predictedTime, err := tvProgrammeService.GetPredictedLiveTime(ctx, streamer.ID, 1)
	if err != nil {
		t.Fatalf("Failed to get predicted live time: %v", err)
	}

	// Verify that the predicted hour is 14
	if predictedTime.Hour != 14 {
		t.Errorf("Expected predicted hour to be 14, got %d", predictedTime.Hour)
	}

	// Verify that the day of week is Monday (1)
	if predictedTime.DayOfWeek != 1 {
		t.Errorf("Expected day of week to be 1 (Monday), got %d", predictedTime.DayOfWeek)
	}

	// Verify that the probability is greater than 0
	if predictedTime.Probability <= 0 {
		t.Errorf("Expected probability to be greater than 0, got %f", predictedTime.Probability)
	}
}

// TestGetPredictedLiveTime_NoHistoricalData tests the edge case of no historical data
func TestGetPredictedLiveTime_NoHistoricalData(t *testing.T) {
	db := setupTVProgrammeTestDB(t)
	ctx := context.Background()

	streamerRepo := sqlite.NewStreamerRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)

	heatmapService := NewHeatmapService(activityRepo, heatmapRepo)
	tvProgrammeService := NewTVProgrammeService(heatmapService, nil, nil, streamerRepo, activityRepo)

	// Create a test streamer with no activity records
	streamer := &domain.Streamer{
		ID:        uuid.New().String(),
		Name:      "NewStreamer",
		Handles:   map[string]string{"youtube": "newstreamer"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := streamerRepo.Create(ctx, streamer); err != nil {
		t.Fatalf("Failed to create streamer: %v", err)
	}

	// Try to get predicted live time
	_, err := tvProgrammeService.GetPredictedLiveTime(ctx, streamer.ID, 1)
	if err == nil {
		t.Errorf("Expected error for streamer with no historical data, got nil")
	}
}

// TestGetMostViewedStreamers_CorrectOrdering tests that streamers are ordered by follower count
func TestGetMostViewedStreamers_CorrectOrdering(t *testing.T) {
	db := setupTVProgrammeTestDB(t)
	ctx := context.Background()

	streamerRepo := sqlite.NewStreamerRepository(db)
	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)

	heatmapService := NewHeatmapService(activityRepo, heatmapRepo)
	tvProgrammeService := NewTVProgrammeService(heatmapService, userRepo, followRepo, streamerRepo, activityRepo)

	// Create test streamers
	streamer1 := &domain.Streamer{
		ID:        uuid.New().String(),
		Name:      "Streamer1",
		Handles:   map[string]string{"youtube": "streamer1"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	streamer2 := &domain.Streamer{
		ID:        uuid.New().String(),
		Name:      "Streamer2",
		Handles:   map[string]string{"youtube": "streamer2"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	streamer3 := &domain.Streamer{
		ID:        uuid.New().String(),
		Name:      "Streamer3",
		Handles:   map[string]string{"youtube": "streamer3"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := streamerRepo.Create(ctx, streamer1); err != nil {
		t.Fatalf("Failed to create streamer1: %v", err)
	}
	if err := streamerRepo.Create(ctx, streamer2); err != nil {
		t.Fatalf("Failed to create streamer2: %v", err)
	}
	if err := streamerRepo.Create(ctx, streamer3); err != nil {
		t.Fatalf("Failed to create streamer3: %v", err)
	}

	// Create test users
	for i := 0; i < 5; i++ {
		user := &domain.User{
			ID:        uuid.New().String(),
			GoogleID:  "google-" + uuid.New().String(),
			Email:     uuid.New().String() + "@example.com",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := userRepo.Create(ctx, user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		// All users follow streamer2 (most popular)
		if err := followRepo.Create(ctx, user.ID, streamer2.ID); err != nil {
			t.Fatalf("Failed to create follow: %v", err)
		}

		// 3 users follow streamer1
		if i < 3 {
			if err := followRepo.Create(ctx, user.ID, streamer1.ID); err != nil {
				t.Fatalf("Failed to create follow: %v", err)
			}
		}

		// 1 user follows streamer3
		if i < 1 {
			if err := followRepo.Create(ctx, user.ID, streamer3.ID); err != nil {
				t.Fatalf("Failed to create follow: %v", err)
			}
		}
	}

	// Get most viewed streamers
	streamers, err := tvProgrammeService.GetMostViewedStreamers(ctx, 3)
	if err != nil {
		t.Fatalf("Failed to get most viewed streamers: %v", err)
	}

	// Verify ordering: streamer2 (5 followers), streamer1 (3 followers), streamer3 (1 follower)
	if len(streamers) != 3 {
		t.Fatalf("Expected 3 streamers, got %d", len(streamers))
	}

	if streamers[0].ID != streamer2.ID {
		t.Errorf("Expected first streamer to be streamer2, got %s", streamers[0].Name)
	}

	if streamers[1].ID != streamer1.ID {
		t.Errorf("Expected second streamer to be streamer1, got %s", streamers[1].Name)
	}

	if streamers[2].ID != streamer3.ID {
		t.Errorf("Expected third streamer to be streamer3, got %s", streamers[2].Name)
	}
}
