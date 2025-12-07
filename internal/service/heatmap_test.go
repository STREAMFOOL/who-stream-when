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

// setupTestDB creates a test database for heatmap service tests
func setupHeatmapTestDB(t *testing.T) *sqlite.DB {
	t.Helper()

	// Create temporary database file
	tmpFile, err := os.CreateTemp("", "test-heatmap-*.db")
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

// genActivityRecords generates random activity records for property testing
func genActivityRecords(streamerID string, minRecords, maxRecords int) gopter.Gen {
	return gen.IntRange(minRecords, maxRecords).FlatMap(func(count interface{}) gopter.Gen {
		numRecords := count.(int)
		return gopter.CombineGens(
			gen.SliceOfN(numRecords, genActivityRecord(streamerID)),
		).Map(func(values []interface{}) []*domain.ActivityRecord {
			return values[0].([]*domain.ActivityRecord)
		})
	}, reflect.TypeOf([]*domain.ActivityRecord{}))
}

// genActivityRecord generates a single random activity record
func genActivityRecord(streamerID string) gopter.Gen {
	now := time.Now()
	oneYearAgo := now.AddDate(-1, 0, 0)

	return gopter.CombineGens(
		gen.TimeRange(oneYearAgo, time.Hour*24*365), // Range from 1 year ago to now
		gen.OneConstOf("youtube", "twitch", "kick"),
	).Map(func(values []interface{}) *domain.ActivityRecord {
		startTime := values[0].(time.Time)
		platform := values[1].(string)

		return &domain.ActivityRecord{
			ID:         uuid.New().String(),
			StreamerID: streamerID,
			StartTime:  startTime,
			EndTime:    startTime.Add(time.Hour * 2), // 2 hour sessions
			Platform:   platform,
			CreatedAt:  time.Now(),
		}
	})
}

// **Feature: streamer-tracking-mvp, Property 5: Heatmap Probability Validity**
// **Validates: Requirements 3.1, 3.2**
// For any streamer with historical activity data, the generated heatmap should have
// probability values between 0 and 1 for all hours and days.
func TestProperty_HeatmapProbabilityValidity(t *testing.T) {
	db := setupHeatmapTestDB(t)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)
	service := NewHeatmapService(activityRepo, heatmapRepo)
	ctx := context.Background()

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	streamerRepo := sqlite.NewStreamerRepository(db)

	properties.Property("heatmap probabilities are between 0 and 1", prop.ForAll(
		func(records []*domain.ActivityRecord) bool {
			if len(records) == 0 {
				return true // Skip empty record sets
			}

			streamerID := records[0].StreamerID

			// Create the streamer first to satisfy foreign key constraint
			streamer := &domain.Streamer{
				ID:        streamerID,
				Name:      "Test Streamer",
				Handles:   map[string]string{"youtube": "test"},
				Platforms: []string{"youtube"},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err := streamerRepo.Create(ctx, streamer); err != nil {
				t.Logf("failed to create streamer: %v", err)
				return false
			}

			// Insert activity records
			for _, record := range records {
				if err := activityRepo.Create(ctx, record); err != nil {
					t.Logf("failed to create activity record: %v", err)
					// Clean up streamer
					streamerRepo.Delete(ctx, streamerID)
					return false
				}
			}

			// Generate heatmap
			heatmap, err := service.GenerateHeatmap(ctx, streamerID)
			if err != nil {
				t.Logf("failed to generate heatmap: %v", err)
				// Clean up
				for _, record := range records {
					activityRepo.Delete(ctx, record.ID)
				}
				heatmapRepo.Delete(ctx, streamerID)
				streamerRepo.Delete(ctx, streamerID)
				return false
			}

			// Verify all hour probabilities are between 0 and 1
			for i, prob := range heatmap.Hours {
				if prob < 0 || prob > 1 {
					t.Logf("Hour %d probability out of range: %f", i, prob)
					// Clean up
					for _, record := range records {
						activityRepo.Delete(ctx, record.ID)
					}
					heatmapRepo.Delete(ctx, streamerID)
					streamerRepo.Delete(ctx, streamerID)
					return false
				}
			}

			// Verify all day probabilities are between 0 and 1
			for i, prob := range heatmap.DaysOfWeek {
				if prob < 0 || prob > 1 {
					t.Logf("Day %d probability out of range: %f", i, prob)
					// Clean up
					for _, record := range records {
						activityRepo.Delete(ctx, record.ID)
					}
					heatmapRepo.Delete(ctx, streamerID)
					streamerRepo.Delete(ctx, streamerID)
					return false
				}
			}

			// Verify data points count matches
			if heatmap.DataPoints != len(records) {
				t.Logf("Data points mismatch: expected %d, got %d", len(records), heatmap.DataPoints)
				// Clean up
				for _, record := range records {
					activityRepo.Delete(ctx, record.ID)
				}
				heatmapRepo.Delete(ctx, streamerID)
				streamerRepo.Delete(ctx, streamerID)
				return false
			}

			// Clean up
			for _, record := range records {
				if err := activityRepo.Delete(ctx, record.ID); err != nil {
					t.Logf("failed to delete activity record: %v", err)
				}
			}
			if err := heatmapRepo.Delete(ctx, streamerID); err != nil {
				t.Logf("failed to delete heatmap: %v", err)
			}
			if err := streamerRepo.Delete(ctx, streamerID); err != nil {
				t.Logf("failed to delete streamer: %v", err)
			}

			return true
		},
		genActivityRecords(uuid.New().String(), 1, 50),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: streamer-tracking-mvp, Property 6: Weighted Activity Calculation**
// **Validates: Requirements 3.3**
// For any streamer with activity records spanning more than 3 months, the heatmap should
// weight the last 3 months at 80% and older data at 20% when calculating probabilities.
func TestProperty_WeightedActivityCalculation(t *testing.T) {
	db := setupHeatmapTestDB(t)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)
	service := NewHeatmapService(activityRepo, heatmapRepo)
	ctx := context.Background()

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	streamerRepo := sqlite.NewStreamerRepository(db)

	properties.Property("heatmap weights recent data at 80% and older at 20%", prop.ForAll(
		func(hour int) bool {
			streamerID := uuid.New().String()

			// Create the streamer first
			streamer := &domain.Streamer{
				ID:        streamerID,
				Name:      "Test Streamer",
				Handles:   map[string]string{"youtube": "test"},
				Platforms: []string{"youtube"},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err := streamerRepo.Create(ctx, streamer); err != nil {
				t.Logf("failed to create streamer: %v", err)
				return false
			}

			now := time.Now().UTC()
			oneYearAgo := now.AddDate(-1, 0, 0)
			twoMonthsAgo := now.AddDate(0, -2, 0)
			fiveMonthsAgo := now.AddDate(0, -5, 0)

			// Create 10 recent records (last 3 months) all at the same hour
			// Start from 2 months ago to ensure all records are within the recent window
			for i := 0; i < 10; i++ {
				// Use a fixed date/time to avoid DST issues
				baseTime := time.Date(twoMonthsAgo.Year(), twoMonthsAgo.Month(), twoMonthsAgo.Day(), hour, 0, 0, 0, time.UTC)
				record := &domain.ActivityRecord{
					ID:         uuid.New().String(),
					StreamerID: streamerID,
					StartTime:  baseTime.AddDate(0, 0, i),
					EndTime:    baseTime.AddDate(0, 0, i).Add(2 * time.Hour),
					Platform:   "youtube",
					CreatedAt:  time.Now(),
				}
				if err := activityRepo.Create(ctx, record); err != nil {
					t.Logf("failed to create recent activity record: %v", err)
					streamerRepo.Delete(ctx, streamerID)
					return false
				}
			}

			// Create 10 older records (3-6 months ago) all at a different hour
			// Start from 5 months ago to ensure all records are in the older window
			differentHour := (hour + 12) % 24
			for i := 0; i < 10; i++ {
				// Use a fixed date/time to avoid DST issues
				baseTime := time.Date(fiveMonthsAgo.Year(), fiveMonthsAgo.Month(), fiveMonthsAgo.Day(), differentHour, 0, 0, 0, time.UTC)
				record := &domain.ActivityRecord{
					ID:         uuid.New().String(),
					StreamerID: streamerID,
					StartTime:  baseTime.AddDate(0, 0, i),
					EndTime:    baseTime.AddDate(0, 0, i).Add(2 * time.Hour),
					Platform:   "youtube",
					CreatedAt:  time.Now(),
				}
				if err := activityRepo.Create(ctx, record); err != nil {
					t.Logf("failed to create older activity record: %v", err)
					// Clean up
					activityRepo.GetByStreamerID(ctx, streamerID, fiveMonthsAgo)
					streamerRepo.Delete(ctx, streamerID)
					return false
				}
			}

			// Generate heatmap
			heatmap, err := service.GenerateHeatmap(ctx, streamerID)
			if err != nil {
				t.Logf("failed to generate heatmap: %v", err)
				streamerRepo.Delete(ctx, streamerID)
				return false
			}

			// The recent hour should have higher probability than the older hour
			// Recent: 10/10 = 1.0 * 0.8 = 0.8
			// Older: 10/10 = 1.0 * 0.2 = 0.2
			// So the recent hour should be 0.8 and the older hour should be 0.2
			recentProb := heatmap.Hours[hour]
			olderProb := heatmap.Hours[differentHour]

			// Allow for small floating point errors
			expectedRecent := 0.8
			expectedOlder := 0.2
			tolerance := 0.01

			if recentProb < expectedRecent-tolerance || recentProb > expectedRecent+tolerance {
				t.Logf("Recent hour %d probability incorrect: expected ~%f, got %f", hour, expectedRecent, recentProb)
				// Clean up
				records, _ := activityRepo.GetByStreamerID(ctx, streamerID, oneYearAgo)
				for _, record := range records {
					activityRepo.Delete(ctx, record.ID)
				}
				heatmapRepo.Delete(ctx, streamerID)
				streamerRepo.Delete(ctx, streamerID)
				return false
			}

			if olderProb < expectedOlder-tolerance || olderProb > expectedOlder+tolerance {
				t.Logf("Older hour %d probability incorrect: expected ~%f, got %f", differentHour, expectedOlder, olderProb)
				// Clean up
				records, _ := activityRepo.GetByStreamerID(ctx, streamerID, oneYearAgo)
				for _, record := range records {
					activityRepo.Delete(ctx, record.ID)
				}
				heatmapRepo.Delete(ctx, streamerID)
				streamerRepo.Delete(ctx, streamerID)
				return false
			}

			// Clean up
			records, _ := activityRepo.GetByStreamerID(ctx, streamerID, oneYearAgo)
			for _, record := range records {
				activityRepo.Delete(ctx, record.ID)
			}
			heatmapRepo.Delete(ctx, streamerID)
			streamerRepo.Delete(ctx, streamerID)

			return true
		},
		gen.IntRange(0, 23), // Generate random hours
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Unit Tests

// TestGenerateHeatmap_VariousDistributions tests heatmap generation with different data patterns
func TestGenerateHeatmap_VariousDistributions(t *testing.T) {
	db := setupHeatmapTestDB(t)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)
	streamerRepo := sqlite.NewStreamerRepository(db)
	service := NewHeatmapService(activityRepo, heatmapRepo)
	ctx := context.Background()

	streamerID := uuid.New().String()

	// Create streamer
	streamer := &domain.Streamer{
		ID:        streamerID,
		Name:      "Test Streamer",
		Handles:   map[string]string{"youtube": "test"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := streamerRepo.Create(ctx, streamer); err != nil {
		t.Fatalf("failed to create streamer: %v", err)
	}

	now := time.Now().UTC()
	twoMonthsAgo := now.AddDate(0, -2, 0)

	// Create records with a specific pattern: all at hour 14
	for i := 0; i < 5; i++ {
		baseTime := time.Date(twoMonthsAgo.Year(), twoMonthsAgo.Month(), twoMonthsAgo.Day(), 14, 0, 0, 0, time.UTC)
		record := &domain.ActivityRecord{
			ID:         uuid.New().String(),
			StreamerID: streamerID,
			StartTime:  baseTime.AddDate(0, 0, i),
			EndTime:    baseTime.AddDate(0, 0, i).Add(2 * time.Hour),
			Platform:   "youtube",
			CreatedAt:  time.Now(),
		}
		if err := activityRepo.Create(ctx, record); err != nil {
			t.Fatalf("failed to create activity record: %v", err)
		}
	}

	// Generate heatmap
	heatmap, err := service.GenerateHeatmap(ctx, streamerID)
	if err != nil {
		t.Fatalf("failed to generate heatmap: %v", err)
	}

	// Verify hour 14 has the highest probability
	if heatmap.Hours[14] <= 0 {
		t.Errorf("Expected hour 14 to have positive probability, got %f", heatmap.Hours[14])
	}

	// Verify other hours have lower or zero probability
	for i := 0; i < 24; i++ {
		if i != 14 && heatmap.Hours[i] > heatmap.Hours[14] {
			t.Errorf("Hour %d has higher probability than hour 14: %f > %f", i, heatmap.Hours[i], heatmap.Hours[14])
		}
	}
}

// TestGenerateHeatmap_WeightingAlgorithm tests the weighting algorithm with known data
func TestGenerateHeatmap_WeightingAlgorithm(t *testing.T) {
	db := setupHeatmapTestDB(t)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)
	streamerRepo := sqlite.NewStreamerRepository(db)
	service := NewHeatmapService(activityRepo, heatmapRepo)
	ctx := context.Background()

	streamerID := uuid.New().String()

	// Create streamer
	streamer := &domain.Streamer{
		ID:        streamerID,
		Name:      "Test Streamer",
		Handles:   map[string]string{"youtube": "test"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := streamerRepo.Create(ctx, streamer); err != nil {
		t.Fatalf("failed to create streamer: %v", err)
	}

	now := time.Now().UTC()
	twoMonthsAgo := now.AddDate(0, -2, 0)
	fiveMonthsAgo := now.AddDate(0, -5, 0)

	// Create 5 recent records at hour 10
	for i := 0; i < 5; i++ {
		baseTime := time.Date(twoMonthsAgo.Year(), twoMonthsAgo.Month(), twoMonthsAgo.Day(), 10, 0, 0, 0, time.UTC)
		record := &domain.ActivityRecord{
			ID:         uuid.New().String(),
			StreamerID: streamerID,
			StartTime:  baseTime.AddDate(0, 0, i),
			EndTime:    baseTime.AddDate(0, 0, i).Add(2 * time.Hour),
			Platform:   "youtube",
			CreatedAt:  time.Now(),
		}
		if err := activityRepo.Create(ctx, record); err != nil {
			t.Fatalf("failed to create recent activity record: %v", err)
		}
	}

	// Create 5 older records at hour 22
	for i := 0; i < 5; i++ {
		baseTime := time.Date(fiveMonthsAgo.Year(), fiveMonthsAgo.Month(), fiveMonthsAgo.Day(), 22, 0, 0, 0, time.UTC)
		record := &domain.ActivityRecord{
			ID:         uuid.New().String(),
			StreamerID: streamerID,
			StartTime:  baseTime.AddDate(0, 0, i),
			EndTime:    baseTime.AddDate(0, 0, i).Add(2 * time.Hour),
			Platform:   "youtube",
			CreatedAt:  time.Now(),
		}
		if err := activityRepo.Create(ctx, record); err != nil {
			t.Fatalf("failed to create older activity record: %v", err)
		}
	}

	// Generate heatmap
	heatmap, err := service.GenerateHeatmap(ctx, streamerID)
	if err != nil {
		t.Fatalf("failed to generate heatmap: %v", err)
	}

	// Expected: hour 10 = 1.0 * 0.8 = 0.8, hour 22 = 1.0 * 0.2 = 0.2
	tolerance := 0.01
	if heatmap.Hours[10] < 0.8-tolerance || heatmap.Hours[10] > 0.8+tolerance {
		t.Errorf("Hour 10 probability incorrect: expected ~0.8, got %f", heatmap.Hours[10])
	}
	if heatmap.Hours[22] < 0.2-tolerance || heatmap.Hours[22] > 0.2+tolerance {
		t.Errorf("Hour 22 probability incorrect: expected ~0.2, got %f", heatmap.Hours[22])
	}
}

// TestGenerateHeatmap_InsufficientData tests the edge case with no historical data
func TestGenerateHeatmap_InsufficientData(t *testing.T) {
	db := setupHeatmapTestDB(t)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)
	streamerRepo := sqlite.NewStreamerRepository(db)
	service := NewHeatmapService(activityRepo, heatmapRepo)
	ctx := context.Background()

	streamerID := uuid.New().String()

	// Create streamer but no activity records
	streamer := &domain.Streamer{
		ID:        streamerID,
		Name:      "Test Streamer",
		Handles:   map[string]string{"youtube": "test"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := streamerRepo.Create(ctx, streamer); err != nil {
		t.Fatalf("failed to create streamer: %v", err)
	}

	// Try to generate heatmap with no data
	_, err := service.GenerateHeatmap(ctx, streamerID)
	if err != ErrInsufficientData {
		t.Errorf("Expected ErrInsufficientData, got %v", err)
	}
}

// TestRecordActivity tests recording activity
func TestRecordActivity(t *testing.T) {
	db := setupHeatmapTestDB(t)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)
	streamerRepo := sqlite.NewStreamerRepository(db)
	service := NewHeatmapService(activityRepo, heatmapRepo)
	ctx := context.Background()

	streamerID := uuid.New().String()

	// Create streamer
	streamer := &domain.Streamer{
		ID:        streamerID,
		Name:      "Test Streamer",
		Handles:   map[string]string{"youtube": "test"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := streamerRepo.Create(ctx, streamer); err != nil {
		t.Fatalf("failed to create streamer: %v", err)
	}

	// Record activity
	timestamp := time.Now()
	if err := service.RecordActivity(ctx, streamerID, timestamp); err != nil {
		t.Fatalf("failed to record activity: %v", err)
	}

	// Verify activity was recorded
	oneYearAgo := time.Now().AddDate(-1, 0, 0)
	records, err := activityRepo.GetByStreamerID(ctx, streamerID, oneYearAgo)
	if err != nil {
		t.Fatalf("failed to get activity records: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 activity record, got %d", len(records))
	}
}

// TestGetActivityStats tests retrieving activity statistics
func TestGetActivityStats(t *testing.T) {
	db := setupHeatmapTestDB(t)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)
	streamerRepo := sqlite.NewStreamerRepository(db)
	service := NewHeatmapService(activityRepo, heatmapRepo)
	ctx := context.Background()

	streamerID := uuid.New().String()

	// Create streamer
	streamer := &domain.Streamer{
		ID:        streamerID,
		Name:      "Test Streamer",
		Handles:   map[string]string{"youtube": "test"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := streamerRepo.Create(ctx, streamer); err != nil {
		t.Fatalf("failed to create streamer: %v", err)
	}

	now := time.Now().UTC()
	twoMonthsAgo := now.AddDate(0, -2, 0)

	// Create 3 records at hour 15
	for i := 0; i < 3; i++ {
		baseTime := time.Date(twoMonthsAgo.Year(), twoMonthsAgo.Month(), twoMonthsAgo.Day(), 15, 0, 0, 0, time.UTC)
		record := &domain.ActivityRecord{
			ID:         uuid.New().String(),
			StreamerID: streamerID,
			StartTime:  baseTime.AddDate(0, 0, i),
			EndTime:    baseTime.AddDate(0, 0, i).Add(2 * time.Hour),
			Platform:   "youtube",
			CreatedAt:  time.Now(),
		}
		if err := activityRepo.Create(ctx, record); err != nil {
			t.Fatalf("failed to create activity record: %v", err)
		}
	}

	// Get stats
	stats, err := service.GetActivityStats(ctx, streamerID)
	if err != nil {
		t.Fatalf("failed to get activity stats: %v", err)
	}

	if stats.TotalSessions != 3 {
		t.Errorf("Expected 3 total sessions, got %d", stats.TotalSessions)
	}

	if stats.MostActiveHour != 15 {
		t.Errorf("Expected most active hour to be 15, got %d", stats.MostActiveHour)
	}

	expectedDuration := 2 * time.Hour
	if stats.AverageSessionDuration != expectedDuration {
		t.Errorf("Expected average session duration %v, got %v", expectedDuration, stats.AverageSessionDuration)
	}
}
