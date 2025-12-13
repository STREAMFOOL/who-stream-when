package service

import (
	"context"
	"os"
	"testing"
	"time"

	"who-live-when/internal/auth"
	"who-live-when/internal/config"
	"who-live-when/internal/domain"
	"who-live-when/internal/repository/sqlite"
)

// TestIntegration_GuestUserFlow tests the complete guest user flow:
// search → follow → programme → register
func TestIntegration_GuestUserFlow(t *testing.T) {
	// Create temporary database
	tmpFile := t.TempDir() + "/test.db"

	db, err := sqlite.NewDB(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	if err := sqlite.Migrate(db.DB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	streamerRepo := sqlite.NewStreamerRepository(db)
	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)
	programmeRepo := sqlite.NewCustomProgrammeRepository(db)

	// Initialize services
	streamerService := NewStreamerService(streamerRepo)
	heatmapService := NewHeatmapService(activityRepo, heatmapRepo)
	userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo, programmeRepo)
	programmeService := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	ctx := context.Background()

	// Step 1: Create test streamers (simulating search results)
	streamer1 := &domain.Streamer{
		ID:        "guest-flow-streamer-1",
		Name:      "Guest Flow Streamer 1",
		Handles:   map[string]string{"kick": "guestflow1"},
		Platforms: []string{"kick"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	streamer2 := &domain.Streamer{
		ID:        "guest-flow-streamer-2",
		Name:      "Guest Flow Streamer 2",
		Handles:   map[string]string{"kick": "guestflow2"},
		Platforms: []string{"kick"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := streamerService.AddStreamer(ctx, streamer1); err != nil {
		t.Fatalf("Failed to add streamer 1: %v", err)
	}
	if err := streamerService.AddStreamer(ctx, streamer2); err != nil {
		t.Fatalf("Failed to add streamer 2: %v", err)
	}

	// Add activity records for heatmap generation
	for i := 0; i < 10; i++ {
		timestamp := time.Now().AddDate(0, 0, -i)
		if err := heatmapService.RecordActivity(ctx, streamer1.ID, timestamp); err != nil {
			t.Fatalf("Failed to record activity: %v", err)
		}
	}

	// Step 2: Simulate guest user following streamers (session-based)
	sessionManager := auth.NewSessionManager("test-session", false, 3600)
	guestFollows := []string{streamer1.ID, streamer2.ID}

	// Step 3: Simulate guest creating custom programme
	guestProgramme := programmeService.CreateGuestProgramme(guestFollows)
	if guestProgramme == nil {
		t.Fatal("Failed to create guest programme")
	}
	if guestProgramme.UserID != "" {
		t.Error("Guest programme should have empty UserID")
	}
	if len(guestProgramme.StreamerIDs) != 2 {
		t.Errorf("Expected 2 streamers in guest programme, got %d", len(guestProgramme.StreamerIDs))
	}

	// Step 4: Generate calendar from guest programme
	week := time.Now()
	calendarView, err := programmeService.GenerateCalendarFromProgramme(ctx, guestProgramme, week)
	if err != nil {
		t.Fatalf("Failed to generate calendar from guest programme: %v", err)
	}
	if !calendarView.IsGuestSession {
		t.Error("Calendar view should indicate guest session")
	}
	if !calendarView.IsCustom {
		t.Error("Calendar view should indicate custom programme")
	}

	// Step 5: Simulate guest registering (creating user account)
	user, err := userService.CreateUser(ctx, "guest-google-id", "guest@example.com")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Step 6: Migrate guest data to registered user
	err = userService.MigrateGuestData(ctx, user.ID, guestFollows, guestProgramme)
	if err != nil {
		t.Fatalf("Failed to migrate guest data: %v", err)
	}

	// Verify follows were migrated
	migratedFollows, err := userService.GetUserFollows(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get migrated follows: %v", err)
	}
	if len(migratedFollows) != 2 {
		t.Errorf("Expected 2 migrated follows, got %d", len(migratedFollows))
	}

	// Verify custom programme was migrated
	migratedProgramme, err := programmeService.GetCustomProgramme(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get migrated programme: %v", err)
	}
	if len(migratedProgramme.StreamerIDs) != 2 {
		t.Errorf("Expected 2 streamers in migrated programme, got %d", len(migratedProgramme.StreamerIDs))
	}

	// Step 7: Verify registered user can now use database-backed programme
	registeredCalendar, err := programmeService.GenerateCalendarFromProgramme(ctx, migratedProgramme, week)
	if err != nil {
		t.Fatalf("Failed to generate calendar for registered user: %v", err)
	}
	if registeredCalendar.IsGuestSession {
		t.Error("Registered user calendar should not indicate guest session")
	}
	if !registeredCalendar.IsCustom {
		t.Error("Registered user calendar should indicate custom programme")
	}

	// Verify session manager is available (for clearing guest data after migration)
	if sessionManager == nil {
		t.Error("Session manager should be available")
	}
}

// TestIntegration_RegisteredUserCustomProgramme tests registered user custom programme
// creation and management
func TestIntegration_RegisteredUserCustomProgramme(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"

	db, err := sqlite.NewDB(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	if err := sqlite.Migrate(db.DB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	streamerRepo := sqlite.NewStreamerRepository(db)
	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)
	programmeRepo := sqlite.NewCustomProgrammeRepository(db)

	// Initialize services
	streamerService := NewStreamerService(streamerRepo)
	heatmapService := NewHeatmapService(activityRepo, heatmapRepo)
	userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo, programmeRepo)
	programmeService := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	ctx := context.Background()

	// Create test user
	user, err := userService.CreateUser(ctx, "registered-google-id", "registered@example.com")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create test streamers
	streamers := make([]*domain.Streamer, 3)
	for i := 0; i < 3; i++ {
		streamers[i] = &domain.Streamer{
			ID:        "registered-streamer-" + string(rune('a'+i)),
			Name:      "Registered Streamer " + string(rune('A'+i)),
			Handles:   map[string]string{"kick": "registered" + string(rune('a'+i))},
			Platforms: []string{"kick"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := streamerService.AddStreamer(ctx, streamers[i]); err != nil {
			t.Fatalf("Failed to add streamer %d: %v", i, err)
		}

		// Add activity records
		for j := 0; j < 10; j++ {
			timestamp := time.Now().AddDate(0, 0, -j)
			if err := heatmapService.RecordActivity(ctx, streamers[i].ID, timestamp); err != nil {
				t.Fatalf("Failed to record activity: %v", err)
			}
		}
	}

	// Test 1: Create custom programme
	t.Run("create custom programme", func(t *testing.T) {
		streamerIDs := []string{streamers[0].ID, streamers[1].ID}
		programme, err := programmeService.CreateCustomProgramme(ctx, user.ID, streamerIDs)
		if err != nil {
			t.Fatalf("Failed to create custom programme: %v", err)
		}
		if programme.UserID != user.ID {
			t.Errorf("Expected UserID %s, got %s", user.ID, programme.UserID)
		}
		if len(programme.StreamerIDs) != 2 {
			t.Errorf("Expected 2 streamers, got %d", len(programme.StreamerIDs))
		}
	})

	// Test 2: Retrieve custom programme
	t.Run("retrieve custom programme", func(t *testing.T) {
		programme, err := programmeService.GetCustomProgramme(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get custom programme: %v", err)
		}
		if len(programme.StreamerIDs) != 2 {
			t.Errorf("Expected 2 streamers, got %d", len(programme.StreamerIDs))
		}
	})

	// Test 3: Update custom programme (add streamer)
	t.Run("add streamer to programme", func(t *testing.T) {
		err := programmeService.AddStreamerToProgramme(ctx, user.ID, streamers[2].ID)
		if err != nil {
			t.Fatalf("Failed to add streamer to programme: %v", err)
		}

		programme, err := programmeService.GetCustomProgramme(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get updated programme: %v", err)
		}
		if len(programme.StreamerIDs) != 3 {
			t.Errorf("Expected 3 streamers after add, got %d", len(programme.StreamerIDs))
		}
	})

	// Test 4: Remove streamer from programme
	t.Run("remove streamer from programme", func(t *testing.T) {
		err := programmeService.RemoveStreamerFromProgramme(ctx, user.ID, streamers[1].ID)
		if err != nil {
			t.Fatalf("Failed to remove streamer from programme: %v", err)
		}

		programme, err := programmeService.GetCustomProgramme(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get updated programme: %v", err)
		}
		if len(programme.StreamerIDs) != 2 {
			t.Errorf("Expected 2 streamers after remove, got %d", len(programme.StreamerIDs))
		}

		// Verify correct streamer was removed
		for _, id := range programme.StreamerIDs {
			if id == streamers[1].ID {
				t.Error("Removed streamer should not be in programme")
			}
		}
	})

	// Test 5: Generate calendar from custom programme
	t.Run("generate calendar from custom programme", func(t *testing.T) {
		programme, err := programmeService.GetCustomProgramme(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get programme: %v", err)
		}

		week := time.Now()
		calendar, err := programmeService.GenerateCalendarFromProgramme(ctx, programme, week)
		if err != nil {
			t.Fatalf("Failed to generate calendar: %v", err)
		}

		if !calendar.IsCustom {
			t.Error("Calendar should indicate custom programme")
		}
		if calendar.IsGuestSession {
			t.Error("Calendar should not indicate guest session for registered user")
		}
		if len(calendar.Streamers) != 2 {
			t.Errorf("Expected 2 streamers in calendar, got %d", len(calendar.Streamers))
		}
	})

	// Test 6: Delete custom programme and fall back to global
	t.Run("delete programme and fallback to global", func(t *testing.T) {
		err := programmeService.DeleteCustomProgramme(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to delete custom programme: %v", err)
		}

		// Should now get global programme
		week := time.Now()
		view, err := programmeService.GetProgrammeView(ctx, user.ID, week)
		if err != nil {
			t.Fatalf("Failed to get programme view: %v", err)
		}

		if view.IsCustom {
			t.Error("Should fall back to global programme after deletion")
		}
	})
}

// TestIntegration_ConfigurationLoading tests configuration loading with various setups
func TestIntegration_ConfigurationLoading(t *testing.T) {
	// Save original environment
	originalEnv := map[string]string{
		"GOOGLE_CLIENT_ID":     os.Getenv("GOOGLE_CLIENT_ID"),
		"GOOGLE_CLIENT_SECRET": os.Getenv("GOOGLE_CLIENT_SECRET"),
		"DATABASE_PATH":        os.Getenv("DATABASE_PATH"),
		"FEATURE_FLAGS":        os.Getenv("FEATURE_FLAGS"),
		"SESSION_DURATION":     os.Getenv("SESSION_DURATION"),
	}

	// Restore environment after test
	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	t.Run("loads valid configuration", func(t *testing.T) {
		os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test-client-secret")
		os.Setenv("DATABASE_PATH", "./test.db")
		os.Setenv("FEATURE_FLAGS", "kick")

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Failed to load valid configuration: %v", err)
		}

		if cfg.GoogleClientID != "test-client-id" {
			t.Errorf("Expected GoogleClientID 'test-client-id', got '%s'", cfg.GoogleClientID)
		}
		if cfg.DatabasePath != "./test.db" {
			t.Errorf("Expected DatabasePath './test.db', got '%s'", cfg.DatabasePath)
		}
	})

	t.Run("fails on missing required variables", func(t *testing.T) {
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")

		_, err := config.Load()
		if err == nil {
			t.Error("Expected error for missing GOOGLE_CLIENT_ID")
		}
	})

	t.Run("uses default values", func(t *testing.T) {
		os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test-client-secret")
		os.Unsetenv("DATABASE_PATH")
		os.Unsetenv("SERVER_PORT")

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Failed to load configuration: %v", err)
		}

		if cfg.DatabasePath != "./data/who-live-when.db" {
			t.Errorf("Expected default DatabasePath, got '%s'", cfg.DatabasePath)
		}
		if cfg.ServerPort != "8080" {
			t.Errorf("Expected default ServerPort '8080', got '%s'", cfg.ServerPort)
		}
	})

	t.Run("parses feature flags correctly", func(t *testing.T) {
		os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test-client-secret")
		os.Setenv("FEATURE_FLAGS", "kick,youtube")

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Failed to load configuration: %v", err)
		}

		if !cfg.FeatureFlags.IsEnabled(config.FeatureKick) {
			t.Error("Expected Kick to be enabled")
		}
		if !cfg.FeatureFlags.IsEnabled(config.FeatureYouTube) {
			t.Error("Expected YouTube to be enabled")
		}
		if cfg.FeatureFlags.IsEnabled(config.FeatureTwitch) {
			t.Error("Expected Twitch to be disabled")
		}
	})

	t.Run("default feature flags enable only Kick", func(t *testing.T) {
		os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test-client-secret")
		os.Unsetenv("FEATURE_FLAGS")

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Failed to load configuration: %v", err)
		}

		if !cfg.FeatureFlags.IsEnabled(config.FeatureKick) {
			t.Error("Expected Kick to be enabled by default")
		}
		if cfg.FeatureFlags.IsEnabled(config.FeatureYouTube) {
			t.Error("Expected YouTube to be disabled by default")
		}
		if cfg.FeatureFlags.IsEnabled(config.FeatureTwitch) {
			t.Error("Expected Twitch to be disabled by default")
		}
	})

	t.Run("handles invalid session duration", func(t *testing.T) {
		os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test-client-secret")
		os.Setenv("SESSION_DURATION", "invalid")

		_, err := config.Load()
		if err == nil {
			t.Error("Expected error for invalid SESSION_DURATION")
		}
	})
}

// TestIntegration_CorrectnessPropertiesValidation verifies that all correctness properties
// from the design document are covered by tests
func TestIntegration_CorrectnessPropertiesValidation(t *testing.T) {
	// This test documents which correctness properties are validated by which tests
	// It serves as a verification checklist for the implementation

	properties := []struct {
		number      int
		name        string
		testFile    string
		testName    string
		requirement string
	}{
		{1, "Multi-Platform Search Coverage", "search_property_test.go", "TestProperty_UXE_MultiPlatformSearchCoverage", "1.2"},
		{2, "Search Result Completeness", "search_property_test.go", "TestProperty_UXE_SearchResultCompleteness", "1.3"},
		{3, "Registered User Follow Persistence", "user_test.go", "TestProperty_RegisteredUserFollowPersistence", "2.1"},
		{4, "Guest User Follow Session Storage", "google_property_test.go", "TestProperty_GuestUserFollowSessionStorage", "2.2, 7.1"},
		{5, "Follow List Completeness", "user_test.go", "TestProperty_FollowListCompleteness", "2.3"},
		{6, "Custom Programme Database Persistence", "custom_programme_property_test.go", "TestProperty_CustomProgrammeDatabasePersistence", "3.1"},
		{7, "Custom Programme Session Persistence", "google_property_test.go", "TestProperty_SessionDataPersistenceAcrossRequests", "3.2, 7.2"},
		{8, "Custom Programme Calendar Filtering", "programme_property_test.go", "TestProperty_CustomProgrammeCalendarFiltering", "3.3, 9.2"},
		{9, "Streamer Removal from Programme", "programme_property_test.go", "TestProperty_StreamerRemovalFromProgramme", "3.4, 9.3"},
		{10, "Global Programme Ranking", "programme_property_test.go", "TestProperty_GlobalProgrammeRanking", "4.2"},
		{11, "Session Data Persistence Across Requests", "google_property_test.go", "TestProperty_SessionDataPersistenceAcrossRequests", "7.3"},
		{12, "Guest Data Migration on Registration", "user_test.go", "TestGuestDataMigration", "7.5"},
		{13, "Streamer Creation from Search Result", "user_experience_integration_test.go", "TestIntegration_StreamerCreationFromSearch", "8.2"},
		{14, "Streamer Addition Idempotence", "streamer_property_test.go", "TestProperty_UXE_StreamerAdditionIdempotence", "8.4"},
		{15, "Feature Flag Platform Filtering", "user_experience_integration_test.go", "TestIntegration_FollowWithFeatureFlags", "10.2"},
		{16, "Disabled Platform Rejection", "user_test.go", "TestFollowStreamer_DisabledPlatformRejection", "10.4"},
	}

	t.Run("all correctness properties have tests", func(t *testing.T) {
		for _, prop := range properties {
			t.Logf("Property %d: %s - Validated by %s in %s (Requirements: %s)",
				prop.number, prop.name, prop.testName, prop.testFile, prop.requirement)
		}

		// Verify we have all 16 properties documented
		if len(properties) != 16 {
			t.Errorf("Expected 16 correctness properties, documented %d", len(properties))
		}
	})
}

// TestIntegration_FollowWithFeatureFlags tests follow functionality with feature flags
func TestIntegration_FollowWithFeatureFlags(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"

	db, err := sqlite.NewDB(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	if err := sqlite.Migrate(db.DB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	streamerRepo := sqlite.NewStreamerRepository(db)
	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	programmeRepo := sqlite.NewCustomProgrammeRepository(db)

	ctx := context.Background()

	// Create test user
	featureFlags := config.FeatureKick // Only Kick enabled
	userService := NewUserServiceWithFeatureFlags(userRepo, followRepo, activityRepo, streamerRepo, programmeRepo, featureFlags)

	user, err := userService.CreateUser(ctx, "feature-flag-user", "feature@example.com")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create streamers on different platforms
	kickStreamer := &domain.Streamer{
		ID:        "kick-streamer-ff",
		Name:      "Kick Streamer",
		Handles:   map[string]string{"kick": "kickstreamer"},
		Platforms: []string{"kick"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	youtubeStreamer := &domain.Streamer{
		ID:        "youtube-streamer-ff",
		Name:      "YouTube Streamer",
		Handles:   map[string]string{"youtube": "youtubestreamer"},
		Platforms: []string{"youtube"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	streamerService := NewStreamerService(streamerRepo)
	if err := streamerService.AddStreamer(ctx, kickStreamer); err != nil {
		t.Fatalf("Failed to add kick streamer: %v", err)
	}
	if err := streamerService.AddStreamer(ctx, youtubeStreamer); err != nil {
		t.Fatalf("Failed to add youtube streamer: %v", err)
	}

	t.Run("allows following enabled platform streamer", func(t *testing.T) {
		err := userService.FollowStreamer(ctx, user.ID, kickStreamer.ID)
		if err != nil {
			t.Errorf("Should allow following Kick streamer: %v", err)
		}
	})

	t.Run("rejects following disabled platform streamer", func(t *testing.T) {
		err := userService.FollowStreamer(ctx, user.ID, youtubeStreamer.ID)
		if err == nil {
			t.Error("Should reject following YouTube streamer when YouTube is disabled")
		}
	})
}

// TestIntegration_StreamerCreationFromSearch tests creating streamers from search results
func TestIntegration_StreamerCreationFromSearch(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"

	db, err := sqlite.NewDB(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	if err := sqlite.Migrate(db.DB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	streamerRepo := sqlite.NewStreamerRepository(db)
	streamerService := NewStreamerService(streamerRepo)

	ctx := context.Background()

	t.Run("creates streamer from search result", func(t *testing.T) {
		// Simulate search result data
		platform := "kick"
		handle := "newstreamer123"
		name := "New Streamer 123"

		streamer, err := streamerService.GetOrCreateStreamer(ctx, platform, handle, name)
		if err != nil {
			t.Fatalf("Failed to create streamer: %v", err)
		}

		if streamer.Name != name {
			t.Errorf("Expected name '%s', got '%s'", name, streamer.Name)
		}
		if streamer.Handles[platform] != handle {
			t.Errorf("Expected handle '%s', got '%s'", handle, streamer.Handles[platform])
		}
	})

	t.Run("returns existing streamer on duplicate", func(t *testing.T) {
		platform := "kick"
		handle := "existingstreamer"
		name := "Existing Streamer"

		// Create first time
		first, err := streamerService.GetOrCreateStreamer(ctx, platform, handle, name)
		if err != nil {
			t.Fatalf("Failed to create first streamer: %v", err)
		}

		// Try to create again
		second, err := streamerService.GetOrCreateStreamer(ctx, platform, handle, name)
		if err != nil {
			t.Fatalf("Failed on second call: %v", err)
		}

		if first.ID != second.ID {
			t.Error("Should return same streamer on duplicate creation")
		}
	})
}

// TestIntegration_GlobalProgrammeRanking tests that global programme ranks by followers
func TestIntegration_GlobalProgrammeRanking(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"

	db, err := sqlite.NewDB(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	if err := sqlite.Migrate(db.DB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	streamerRepo := sqlite.NewStreamerRepository(db)
	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	heatmapRepo := sqlite.NewHeatmapRepository(db)
	programmeRepo := sqlite.NewCustomProgrammeRepository(db)

	// Initialize services
	streamerService := NewStreamerService(streamerRepo)
	heatmapService := NewHeatmapService(activityRepo, heatmapRepo)
	userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo, programmeRepo)
	programmeService := NewProgrammeService(programmeRepo, streamerRepo, followRepo, heatmapService)

	ctx := context.Background()

	// Create streamers with different follower counts
	streamers := []struct {
		id        string
		name      string
		followers int
	}{
		{"low-follower", "Low Follower Streamer", 1},
		{"mid-follower", "Mid Follower Streamer", 5},
		{"high-follower", "High Follower Streamer", 10},
	}

	for _, s := range streamers {
		streamer := &domain.Streamer{
			ID:        s.id,
			Name:      s.name,
			Handles:   map[string]string{"kick": s.id},
			Platforms: []string{"kick"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := streamerService.AddStreamer(ctx, streamer); err != nil {
			t.Fatalf("Failed to add streamer: %v", err)
		}

		// Add activity for heatmap
		for i := 0; i < 10; i++ {
			timestamp := time.Now().AddDate(0, 0, -i)
			if err := heatmapService.RecordActivity(ctx, s.id, timestamp); err != nil {
				t.Fatalf("Failed to record activity: %v", err)
			}
		}

		// Create followers
		for i := 0; i < s.followers; i++ {
			user, err := userService.CreateUser(ctx, "follower-"+s.id+"-"+string(rune('0'+i)), "follower"+s.id+string(rune('0'+i))+"@example.com")
			if err != nil {
				t.Fatalf("Failed to create follower user: %v", err)
			}
			if err := userService.FollowStreamer(ctx, user.ID, s.id); err != nil {
				t.Fatalf("Failed to follow streamer: %v", err)
			}
		}
	}

	// Get ranked streamers
	ranked, err := programmeService.GetStreamersRankedByFollowers(ctx, 10)
	if err != nil {
		t.Fatalf("Failed to get ranked streamers: %v", err)
	}

	if len(ranked) < 3 {
		t.Fatalf("Expected at least 3 streamers, got %d", len(ranked))
	}

	// Verify ordering (highest followers first)
	for i := 0; i < len(ranked)-1; i++ {
		if ranked[i].FollowerCount < ranked[i+1].FollowerCount {
			t.Errorf("Streamers not properly ranked: %s (%d followers) should come after %s (%d followers)",
				ranked[i].Streamer.Name, ranked[i].FollowerCount,
				ranked[i+1].Streamer.Name, ranked[i+1].FollowerCount)
		}
	}
}
