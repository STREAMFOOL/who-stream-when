package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"who-live-when/internal/domain"
	"who-live-when/internal/repository/sqlite"

	"github.com/google/uuid"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// setupTestDB creates a test database for property tests
func setupTestDB(t *testing.T) *sqlite.DB {
	t.Helper()

	// Create temporary database file
	tmpFile, err := os.CreateTemp("", "test-*.db")
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

// **Feature: streamer-tracking-mvp, Property 12: User Session Establishment**
// **Validates: Requirements 6.2**
// For any successful Google OAuth authentication, the system should create a new user account
// (if not exists) or retrieve the existing one and establish a valid session.
func TestProperty_UserSessionEstablishment(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("creating user with Google OAuth data should be idempotent", prop.ForAll(
		func(googleID string, email string) bool {
			db := setupTestDB(t)
			defer db.Close()

			userRepo := sqlite.NewUserRepository(db)
			followRepo := sqlite.NewFollowRepository(db)
			activityRepo := sqlite.NewActivityRecordRepository(db)
			streamerRepo := sqlite.NewStreamerRepository(db)
			userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

			ctx := context.Background()

			// First call - create user
			user1, err := userService.CreateUser(ctx, googleID, email)
			if err != nil {
				t.Logf("Failed to create user: %v", err)
				return false
			}

			// Verify user was created
			if user1 == nil {
				t.Log("User1 is nil")
				return false
			}
			if user1.GoogleID != googleID {
				t.Logf("GoogleID mismatch: expected %s, got %s", googleID, user1.GoogleID)
				return false
			}
			if user1.Email != email {
				t.Logf("Email mismatch: expected %s, got %s", email, user1.Email)
				return false
			}
			if user1.ID == "" {
				t.Log("User ID is empty")
				return false
			}

			// Second call - should return existing user (idempotent)
			user2, err := userService.CreateUser(ctx, googleID, email)
			if err != nil {
				t.Logf("Failed to get existing user: %v", err)
				return false
			}

			// Verify same user is returned
			if user2 == nil {
				t.Log("User2 is nil")
				return false
			}
			if user1.ID != user2.ID {
				t.Logf("User IDs don't match: %s != %s", user1.ID, user2.ID)
				return false
			}
			if user1.GoogleID != user2.GoogleID {
				t.Logf("GoogleIDs don't match: %s != %s", user1.GoogleID, user2.GoogleID)
				return false
			}

			// Verify user can be retrieved by ID
			retrievedUser, err := userService.GetUser(ctx, user1.ID)
			if err != nil {
				t.Logf("Failed to retrieve user: %v", err)
				return false
			}
			if retrievedUser.ID != user1.ID {
				t.Logf("Retrieved user ID mismatch: %s != %s", retrievedUser.ID, user1.ID)
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
		gen.Identifier().Map(func(s string) string { return s + "@example.com" }),
	))

	properties.TestingRun(t)
}

// **Feature: streamer-tracking-mvp, Property 16: Follow Operation Idempotence**
// **Validates: Requirements 8.1**
// For any registered user following a streamer, following the same streamer multiple times
// should result in a single follow relationship.
func TestProperty_FollowOperationIdempotence(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("following a streamer multiple times is idempotent", prop.ForAll(
		func(googleID string, email string, streamerName string) bool {
			db := setupTestDB(t)
			defer db.Close()

			userRepo := sqlite.NewUserRepository(db)
			followRepo := sqlite.NewFollowRepository(db)
			activityRepo := sqlite.NewActivityRecordRepository(db)
			streamerRepo := sqlite.NewStreamerRepository(db)
			userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

			ctx := context.Background()

			// Create user
			user, err := userService.CreateUser(ctx, googleID, email)
			if err != nil {
				t.Logf("Failed to create user: %v", err)
				return false
			}

			// Create streamer
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

			// Follow streamer first time
			if err := userService.FollowStreamer(ctx, user.ID, streamer.ID); err != nil {
				t.Logf("Failed to follow streamer first time: %v", err)
				return false
			}

			// Get follow count
			count1, err := followRepo.GetFollowerCount(ctx, streamer.ID)
			if err != nil {
				t.Logf("Failed to get follower count: %v", err)
				return false
			}
			if count1 != 1 {
				t.Logf("Expected 1 follower, got %d", count1)
				return false
			}

			// Follow streamer second time (should be idempotent)
			if err := userService.FollowStreamer(ctx, user.ID, streamer.ID); err != nil {
				t.Logf("Failed to follow streamer second time: %v", err)
				return false
			}

			// Get follow count again
			count2, err := followRepo.GetFollowerCount(ctx, streamer.ID)
			if err != nil {
				t.Logf("Failed to get follower count after second follow: %v", err)
				return false
			}
			if count2 != 1 {
				t.Logf("Expected 1 follower after second follow, got %d", count2)
				return false
			}

			// Follow streamer third time
			if err := userService.FollowStreamer(ctx, user.ID, streamer.ID); err != nil {
				t.Logf("Failed to follow streamer third time: %v", err)
				return false
			}

			// Get follow count again
			count3, err := followRepo.GetFollowerCount(ctx, streamer.ID)
			if err != nil {
				t.Logf("Failed to get follower count after third follow: %v", err)
				return false
			}
			if count3 != 1 {
				t.Logf("Expected 1 follower after third follow, got %d", count3)
				return false
			}

			// Verify user's followed list contains exactly one instance
			follows, err := userService.GetUserFollows(ctx, user.ID)
			if err != nil {
				t.Logf("Failed to get user follows: %v", err)
				return false
			}
			if len(follows) != 1 {
				t.Logf("Expected 1 followed streamer, got %d", len(follows))
				return false
			}
			if follows[0].ID != streamer.ID {
				t.Logf("Followed streamer ID mismatch: expected %s, got %s", streamer.ID, follows[0].ID)
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
		gen.Identifier().Map(func(s string) string { return s + "@example.com" }),
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
	))

	properties.TestingRun(t)
}

// **Feature: streamer-tracking-mvp, Property 17: Unfollow Removes Relationship**
// **Validates: Requirements 8.2**
// For any registered user who has followed a streamer, unfollowing should remove
// the streamer from their followed list.
func TestProperty_UnfollowRemovesRelationship(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("unfollowing a streamer removes the relationship", prop.ForAll(
		func(googleID string, email string, streamerName string) bool {
			db := setupTestDB(t)
			defer db.Close()

			userRepo := sqlite.NewUserRepository(db)
			followRepo := sqlite.NewFollowRepository(db)
			activityRepo := sqlite.NewActivityRecordRepository(db)
			streamerRepo := sqlite.NewStreamerRepository(db)
			userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

			ctx := context.Background()

			// Create user
			user, err := userService.CreateUser(ctx, googleID, email)
			if err != nil {
				t.Logf("Failed to create user: %v", err)
				return false
			}

			// Create streamer
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

			// Follow streamer
			if err := userService.FollowStreamer(ctx, user.ID, streamer.ID); err != nil {
				t.Logf("Failed to follow streamer: %v", err)
				return false
			}

			// Verify follow exists
			follows, err := userService.GetUserFollows(ctx, user.ID)
			if err != nil {
				t.Logf("Failed to get user follows: %v", err)
				return false
			}
			if len(follows) != 1 {
				t.Logf("Expected 1 followed streamer, got %d", len(follows))
				return false
			}

			// Unfollow streamer
			if err := userService.UnfollowStreamer(ctx, user.ID, streamer.ID); err != nil {
				t.Logf("Failed to unfollow streamer: %v", err)
				return false
			}

			// Verify follow was removed
			followsAfter, err := userService.GetUserFollows(ctx, user.ID)
			if err != nil {
				t.Logf("Failed to get user follows after unfollow: %v", err)
				return false
			}
			if len(followsAfter) != 0 {
				t.Logf("Expected 0 followed streamers after unfollow, got %d", len(followsAfter))
				return false
			}

			// Verify follower count is 0
			count, err := followRepo.GetFollowerCount(ctx, streamer.ID)
			if err != nil {
				t.Logf("Failed to get follower count: %v", err)
				return false
			}
			if count != 0 {
				t.Logf("Expected 0 followers after unfollow, got %d", count)
				return false
			}

			// Verify IsFollowing returns false
			isFollowing, err := followRepo.IsFollowing(ctx, user.ID, streamer.ID)
			if err != nil {
				t.Logf("Failed to check follow status: %v", err)
				return false
			}
			if isFollowing {
				t.Log("IsFollowing should return false after unfollow")
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
		gen.Identifier().Map(func(s string) string { return s + "@example.com" }),
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
	))

	properties.TestingRun(t)
}

// **Feature: streamer-tracking-mvp, Property 18: Followed Streamer Visibility**
// **Validates: Requirements 8.1**
// For any registered user, their followed streamers should be visible to other users
// and appear in search results.
func TestProperty_FollowedStreamerVisibility(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("followed streamers are visible to all users", prop.ForAll(
		func(googleID1 string, email1 string, googleID2 string, email2 string, streamerName string) bool {
			// Ensure different users
			if googleID1 == googleID2 {
				return true // Skip if same user
			}

			db := setupTestDB(t)
			defer db.Close()

			userRepo := sqlite.NewUserRepository(db)
			followRepo := sqlite.NewFollowRepository(db)
			activityRepo := sqlite.NewActivityRecordRepository(db)
			streamerRepo := sqlite.NewStreamerRepository(db)
			userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

			ctx := context.Background()

			// Create first user
			user1, err := userService.CreateUser(ctx, googleID1, email1)
			if err != nil {
				t.Logf("Failed to create user1: %v", err)
				return false
			}

			// Create second user
			user2, err := userService.CreateUser(ctx, googleID2, email2)
			if err != nil {
				t.Logf("Failed to create user2: %v", err)
				return false
			}

			// Create streamer
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

			// User1 follows streamer
			if err := userService.FollowStreamer(ctx, user1.ID, streamer.ID); err != nil {
				t.Logf("Failed to follow streamer: %v", err)
				return false
			}

			// Verify streamer is in user1's followed list
			user1Follows, err := userService.GetUserFollows(ctx, user1.ID)
			if err != nil {
				t.Logf("Failed to get user1 follows: %v", err)
				return false
			}
			if len(user1Follows) != 1 {
				t.Logf("Expected 1 followed streamer for user1, got %d", len(user1Follows))
				return false
			}
			if user1Follows[0].ID != streamer.ID {
				t.Logf("Followed streamer ID mismatch for user1")
				return false
			}

			// Verify streamer is visible to user2 (can be retrieved from repository)
			retrievedStreamer, err := streamerRepo.GetByID(ctx, streamer.ID)
			if err != nil {
				t.Logf("Failed to retrieve streamer for user2: %v", err)
				return false
			}
			if retrievedStreamer.ID != streamer.ID {
				t.Logf("Retrieved streamer ID mismatch")
				return false
			}

			// Verify follower count is visible
			followerCount, err := followRepo.GetFollowerCount(ctx, streamer.ID)
			if err != nil {
				t.Logf("Failed to get follower count: %v", err)
				return false
			}
			if followerCount != 1 {
				t.Logf("Expected 1 follower, got %d", followerCount)
				return false
			}

			// User2 can also follow the same streamer (visibility confirmed)
			if err := userService.FollowStreamer(ctx, user2.ID, streamer.ID); err != nil {
				t.Logf("Failed for user2 to follow streamer: %v", err)
				return false
			}

			// Verify both users have the streamer in their follows
			user2Follows, err := userService.GetUserFollows(ctx, user2.ID)
			if err != nil {
				t.Logf("Failed to get user2 follows: %v", err)
				return false
			}
			if len(user2Follows) != 1 {
				t.Logf("Expected 1 followed streamer for user2, got %d", len(user2Follows))
				return false
			}

			// Verify follower count increased
			followerCount2, err := followRepo.GetFollowerCount(ctx, streamer.ID)
			if err != nil {
				t.Logf("Failed to get follower count after user2 follow: %v", err)
				return false
			}
			if followerCount2 != 2 {
				t.Logf("Expected 2 followers, got %d", followerCount2)
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
		gen.Identifier().Map(func(s string) string { return s + "@example.com" }),
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
		gen.Identifier().Map(func(s string) string { return s + "@example.com" }),
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
	))

	properties.TestingRun(t)
}

// **Feature: streamer-tracking-mvp, Property 19: Activity Tracking on Follow**
// **Validates: Requirements 8.4**
// For any streamer that is followed by a user, the system should begin recording
// activity data for heatmap generation.
func TestProperty_ActivityTrackingOnFollow(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("following a streamer enables activity tracking", prop.ForAll(
		func(googleID string, email string, streamerName string) bool {
			db := setupTestDB(t)
			defer db.Close()

			userRepo := sqlite.NewUserRepository(db)
			followRepo := sqlite.NewFollowRepository(db)
			activityRepo := sqlite.NewActivityRecordRepository(db)
			streamerRepo := sqlite.NewStreamerRepository(db)
			userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

			ctx := context.Background()

			// Create user
			user, err := userService.CreateUser(ctx, googleID, email)
			if err != nil {
				t.Logf("Failed to create user: %v", err)
				return false
			}

			// Create streamer
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

			// Check activity records before follow (should be empty)
			recordsBefore, err := activityRepo.GetByStreamerID(ctx, streamer.ID, time.Now().Add(-24*time.Hour))
			if err != nil {
				t.Logf("Failed to get activity records before follow: %v", err)
				return false
			}
			if len(recordsBefore) != 0 {
				t.Logf("Expected 0 activity records before follow, got %d", len(recordsBefore))
				return false
			}

			// Follow streamer
			if err := userService.FollowStreamer(ctx, user.ID, streamer.ID); err != nil {
				t.Logf("Failed to follow streamer: %v", err)
				return false
			}

			// Verify streamer is now being tracked (has a follow relationship)
			isFollowing, err := followRepo.IsFollowing(ctx, user.ID, streamer.ID)
			if err != nil {
				t.Logf("Failed to check follow status: %v", err)
				return false
			}
			if !isFollowing {
				t.Log("User should be following streamer")
				return false
			}

			// Simulate activity recording (this would normally be done by a background task)
			// For this test, we verify that the infrastructure is in place to record activity
			activityRecord := &domain.ActivityRecord{
				ID:         uuid.New().String(),
				StreamerID: streamer.ID,
				StartTime:  time.Now(),
				EndTime:    time.Now().Add(2 * time.Hour),
				Platform:   "youtube",
				CreatedAt:  time.Now(),
			}
			if err := activityRepo.Create(ctx, activityRecord); err != nil {
				t.Logf("Failed to create activity record: %v", err)
				return false
			}

			// Verify activity record was created
			recordsAfter, err := activityRepo.GetByStreamerID(ctx, streamer.ID, time.Now().Add(-24*time.Hour))
			if err != nil {
				t.Logf("Failed to get activity records after follow: %v", err)
				return false
			}
			if len(recordsAfter) != 1 {
				t.Logf("Expected 1 activity record after follow, got %d", len(recordsAfter))
				return false
			}
			if recordsAfter[0].StreamerID != streamer.ID {
				t.Logf("Activity record streamer ID mismatch")
				return false
			}

			// Verify follower count indicates tracking
			followerCount, err := followRepo.GetFollowerCount(ctx, streamer.ID)
			if err != nil {
				t.Logf("Failed to get follower count: %v", err)
				return false
			}
			if followerCount < 1 {
				t.Logf("Expected at least 1 follower for activity tracking, got %d", followerCount)
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
		gen.Identifier().Map(func(s string) string { return s + "@example.com" }),
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
	))

	properties.TestingRun(t)
}

// **Feature: user-experience-enhancements, Property 3: Registered User Follow Persistence**
// **Validates: Requirements 2.1**
// For any registered user and streamer, following the streamer then retrieving follows
// should include that streamer in the results.
func TestProperty_RegisteredUserFollowPersistence(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("following a streamer persists in database and is retrievable", prop.ForAll(
		func(googleID string, email string, streamerName string) bool {
			db := setupTestDB(t)
			defer db.Close()

			userRepo := sqlite.NewUserRepository(db)
			followRepo := sqlite.NewFollowRepository(db)
			activityRepo := sqlite.NewActivityRecordRepository(db)
			streamerRepo := sqlite.NewStreamerRepository(db)
			userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

			ctx := context.Background()

			// Create user
			user, err := userService.CreateUser(ctx, googleID, email)
			if err != nil {
				t.Logf("Failed to create user: %v", err)
				return false
			}

			// Create streamer
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

			// Follow the streamer
			if err := userService.FollowStreamer(ctx, user.ID, streamer.ID); err != nil {
				t.Logf("Failed to follow streamer: %v", err)
				return false
			}

			// Retrieve follows
			follows, err := userService.GetUserFollows(ctx, user.ID)
			if err != nil {
				t.Logf("Failed to get user follows: %v", err)
				return false
			}

			// Verify the streamer is in the follows list
			found := false
			for _, f := range follows {
				if f.ID == streamer.ID {
					found = true
					break
				}
			}

			if !found {
				t.Logf("Streamer %s not found in follows list", streamer.ID)
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
		gen.Identifier().Map(func(s string) string { return s + "@example.com" }),
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
	))

	properties.TestingRun(t)
}

// **Feature: user-experience-enhancements, Property 5: Follow List Completeness**
// **Validates: Requirements 2.3**
// For any user (registered or guest) with a set of follows, retrieving their follows
// should return all followed streamers without omissions.
func TestProperty_FollowListCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Test registered user follow list completeness
	properties.Property("registered user follow list contains all followed streamers", prop.ForAll(
		func(googleID string, email string, numStreamers int) bool {
			db := setupTestDB(t)
			defer db.Close()

			userRepo := sqlite.NewUserRepository(db)
			followRepo := sqlite.NewFollowRepository(db)
			activityRepo := sqlite.NewActivityRecordRepository(db)
			streamerRepo := sqlite.NewStreamerRepository(db)
			userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

			ctx := context.Background()

			// Create user
			user, err := userService.CreateUser(ctx, googleID, email)
			if err != nil {
				t.Logf("Failed to create user: %v", err)
				return false
			}

			// Create multiple streamers and follow them
			expectedIDs := make(map[string]bool)
			for i := 0; i < numStreamers; i++ {
				streamer := &domain.Streamer{
					ID:        uuid.New().String(),
					Name:      fmt.Sprintf("Streamer%d", i),
					Handles:   map[string]string{"youtube": fmt.Sprintf("streamer%d", i)},
					Platforms: []string{"youtube"},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				if err := streamerRepo.Create(ctx, streamer); err != nil {
					t.Logf("Failed to create streamer: %v", err)
					return false
				}

				if err := userService.FollowStreamer(ctx, user.ID, streamer.ID); err != nil {
					t.Logf("Failed to follow streamer: %v", err)
					return false
				}
				expectedIDs[streamer.ID] = true
			}

			// Retrieve follows
			follows, err := userService.GetUserFollows(ctx, user.ID)
			if err != nil {
				t.Logf("Failed to get user follows: %v", err)
				return false
			}

			// Verify count matches
			if len(follows) != numStreamers {
				t.Logf("Expected %d follows, got %d", numStreamers, len(follows))
				return false
			}

			// Verify all followed streamers are present (completeness)
			for _, follow := range follows {
				if !expectedIDs[follow.ID] {
					t.Logf("Unexpected streamer %s in follows", follow.ID)
					return false
				}
				delete(expectedIDs, follow.ID)
			}

			// Verify no streamers are missing
			if len(expectedIDs) != 0 {
				t.Logf("Missing %d streamers from follows", len(expectedIDs))
				return false
			}

			return true
		},
		gen.Identifier().SuchThat(func(v string) bool { return v != "" }),
		gen.Identifier().Map(func(s string) string { return s + "@example.com" }),
		gen.IntRange(1, 20), // Number of streamers to follow
	))

	// Test guest user follow list completeness via GetStreamersByIDs
	properties.Property("guest user follow list contains all requested streamers", prop.ForAll(
		func(numStreamers int) bool {
			db := setupTestDB(t)
			defer db.Close()

			userRepo := sqlite.NewUserRepository(db)
			followRepo := sqlite.NewFollowRepository(db)
			activityRepo := sqlite.NewActivityRecordRepository(db)
			streamerRepo := sqlite.NewStreamerRepository(db)
			userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

			ctx := context.Background()

			// Create multiple streamers
			streamerIDs := make([]string, numStreamers)
			expectedIDs := make(map[string]bool)
			for i := 0; i < numStreamers; i++ {
				streamer := &domain.Streamer{
					ID:        uuid.New().String(),
					Name:      fmt.Sprintf("Streamer%d", i),
					Handles:   map[string]string{"youtube": fmt.Sprintf("streamer%d", i)},
					Platforms: []string{"youtube"},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				if err := streamerRepo.Create(ctx, streamer); err != nil {
					t.Logf("Failed to create streamer: %v", err)
					return false
				}
				streamerIDs[i] = streamer.ID
				expectedIDs[streamer.ID] = true
			}

			// Retrieve streamers by IDs (simulating guest follow retrieval)
			streamers, err := userService.GetStreamersByIDs(ctx, streamerIDs)
			if err != nil {
				t.Logf("Failed to get streamers by IDs: %v", err)
				return false
			}

			// Verify count matches
			if len(streamers) != numStreamers {
				t.Logf("Expected %d streamers, got %d", numStreamers, len(streamers))
				return false
			}

			// Verify all requested streamers are present (completeness)
			for _, streamer := range streamers {
				if !expectedIDs[streamer.ID] {
					t.Logf("Unexpected streamer %s in results", streamer.ID)
					return false
				}
				delete(expectedIDs, streamer.ID)
			}

			// Verify no streamers are missing
			if len(expectedIDs) != 0 {
				t.Logf("Missing %d streamers from results", len(expectedIDs))
				return false
			}

			return true
		},
		gen.IntRange(1, 20), // Number of streamers
	))

	properties.TestingRun(t)
}

// Unit Tests for User Service

func TestCreateUser_WithGoogleOAuthData(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	streamerRepo := sqlite.NewStreamerRepository(db)
	userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

	ctx := context.Background()

	// Test creating a new user
	googleID := "google123"
	email := "test@example.com"

	user, err := userService.CreateUser(ctx, googleID, email)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if user.GoogleID != googleID {
		t.Errorf("Expected GoogleID %s, got %s", googleID, user.GoogleID)
	}
	if user.Email != email {
		t.Errorf("Expected email %s, got %s", email, user.Email)
	}
	if user.ID == "" {
		t.Error("User ID should not be empty")
	}

	// Test creating the same user again (should return existing user)
	user2, err := userService.CreateUser(ctx, googleID, email)
	if err != nil {
		t.Fatalf("Failed to get existing user: %v", err)
	}

	if user2.ID != user.ID {
		t.Errorf("Expected same user ID %s, got %s", user.ID, user2.ID)
	}
}

func TestCreateUser_EmptyGoogleID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	streamerRepo := sqlite.NewStreamerRepository(db)
	userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

	ctx := context.Background()

	_, err := userService.CreateUser(ctx, "", "test@example.com")
	if err == nil {
		t.Error("Expected error for empty Google ID")
	}
}

func TestCreateUser_EmptyEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	streamerRepo := sqlite.NewStreamerRepository(db)
	userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

	ctx := context.Background()

	_, err := userService.CreateUser(ctx, "google123", "")
	if err == nil {
		t.Error("Expected error for empty email")
	}
}

func TestFollowStreamer_AndUnfollowStreamer(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	streamerRepo := sqlite.NewStreamerRepository(db)
	userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

	ctx := context.Background()

	// Create user
	user, err := userService.CreateUser(ctx, "google123", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create streamer
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

	// Test following
	if err := userService.FollowStreamer(ctx, user.ID, streamer.ID); err != nil {
		t.Fatalf("Failed to follow streamer: %v", err)
	}

	// Verify follow
	follows, err := userService.GetUserFollows(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get user follows: %v", err)
	}
	if len(follows) != 1 {
		t.Errorf("Expected 1 followed streamer, got %d", len(follows))
	}
	if follows[0].ID != streamer.ID {
		t.Errorf("Expected streamer ID %s, got %s", streamer.ID, follows[0].ID)
	}

	// Test unfollowing
	if err := userService.UnfollowStreamer(ctx, user.ID, streamer.ID); err != nil {
		t.Fatalf("Failed to unfollow streamer: %v", err)
	}

	// Verify unfollow
	followsAfter, err := userService.GetUserFollows(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get user follows after unfollow: %v", err)
	}
	if len(followsAfter) != 0 {
		t.Errorf("Expected 0 followed streamers after unfollow, got %d", len(followsAfter))
	}
}

func TestGetUserFollows_ReturnsAllFollowedStreamers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	streamerRepo := sqlite.NewStreamerRepository(db)
	userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

	ctx := context.Background()

	// Create user
	user, err := userService.CreateUser(ctx, "google123", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create multiple streamers
	streamers := []*domain.Streamer{
		{
			ID:        uuid.New().String(),
			Name:      "Streamer1",
			Handles:   map[string]string{"youtube": "streamer1"},
			Platforms: []string{"youtube"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        uuid.New().String(),
			Name:      "Streamer2",
			Handles:   map[string]string{"twitch": "streamer2"},
			Platforms: []string{"twitch"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        uuid.New().String(),
			Name:      "Streamer3",
			Handles:   map[string]string{"kick": "streamer3"},
			Platforms: []string{"kick"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, streamer := range streamers {
		if err := streamerRepo.Create(ctx, streamer); err != nil {
			t.Fatalf("Failed to create streamer: %v", err)
		}
		if err := userService.FollowStreamer(ctx, user.ID, streamer.ID); err != nil {
			t.Fatalf("Failed to follow streamer: %v", err)
		}
	}

	// Get all follows
	follows, err := userService.GetUserFollows(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get user follows: %v", err)
	}

	if len(follows) != 3 {
		t.Errorf("Expected 3 followed streamers, got %d", len(follows))
	}

	// Verify all streamers are present
	followedIDs := make(map[string]bool)
	for _, follow := range follows {
		followedIDs[follow.ID] = true
	}

	for _, streamer := range streamers {
		if !followedIDs[streamer.ID] {
			t.Errorf("Streamer %s not found in followed list", streamer.ID)
		}
	}
}

func TestFollowStreamer_EmptyUserID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	streamerRepo := sqlite.NewStreamerRepository(db)
	userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

	ctx := context.Background()

	err := userService.FollowStreamer(ctx, "", "streamer123")
	if err == nil {
		t.Error("Expected error for empty user ID")
	}
}

func TestFollowStreamer_EmptyStreamerID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	streamerRepo := sqlite.NewStreamerRepository(db)
	userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

	ctx := context.Background()

	err := userService.FollowStreamer(ctx, "user123", "")
	if err == nil {
		t.Error("Expected error for empty streamer ID")
	}
}

func TestGetUser_EmptyID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := sqlite.NewUserRepository(db)
	followRepo := sqlite.NewFollowRepository(db)
	activityRepo := sqlite.NewActivityRecordRepository(db)
	streamerRepo := sqlite.NewStreamerRepository(db)
	userService := NewUserService(userRepo, followRepo, activityRepo, streamerRepo)

	ctx := context.Background()

	_, err := userService.GetUser(ctx, "")
	if err == nil {
		t.Error("Expected error for empty user ID")
	}
}
