package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"who-live-when/internal/domain"
)

// **Feature: user-experience-enhancements, Property 6: Custom Programme Database Persistence**
// **Validates: Requirements 3.1**
// For any registered user and set of streamer IDs, creating a custom programme then retrieving it
// should return the same streamer IDs.
func TestProperty_CustomProgrammeDatabasePersistence(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("custom programme round-trip preserves streamer IDs", prop.ForAll(
		func(streamerIDs []string) bool {
			// Setup: Create test database
			db, cleanup := setupTestDB(t)
			defer cleanup()

			repo := NewCustomProgrammeRepository(db)
			ctx := context.Background()

			// Create a test user
			userID := uuid.New().String()
			user := &domain.User{
				ID:        userID,
				GoogleID:  "test-google-" + userID,
				Email:     "test@example.com",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			userRepo := NewUserRepository(db)
			if err := userRepo.Create(ctx, user); err != nil {
				t.Logf("Failed to create test user: %v", err)
				return false
			}

			// Create test streamers
			streamerRepo := NewStreamerRepository(db)
			for _, streamerID := range streamerIDs {
				streamer := &domain.Streamer{
					ID:   streamerID,
					Name: "Test Streamer " + streamerID,
					Handles: map[string]string{
						"youtube": "handle-" + streamerID,
					},
					Platforms: []string{"youtube"},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				if err := streamerRepo.Create(ctx, streamer); err != nil {
					t.Logf("Failed to create test streamer: %v", err)
					return false
				}
			}

			// Step 1: Create custom programme
			programme := &domain.CustomProgramme{
				ID:          uuid.New().String(),
				UserID:      userID,
				StreamerIDs: streamerIDs,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			if err := repo.Create(ctx, programme); err != nil {
				t.Logf("Failed to create custom programme: %v", err)
				return false
			}

			// Step 2: Retrieve custom programme
			retrieved, err := repo.GetByUserID(ctx, userID)
			if err != nil {
				t.Logf("Failed to retrieve custom programme: %v", err)
				return false
			}

			// Step 3: Verify streamer IDs match
			if len(retrieved.StreamerIDs) != len(streamerIDs) {
				t.Logf("Expected %d streamer IDs, got %d", len(streamerIDs), len(retrieved.StreamerIDs))
				return false
			}

			for i, id := range streamerIDs {
				if retrieved.StreamerIDs[i] != id {
					t.Logf("Mismatch at index %d: expected %s, got %s", i, id, retrieved.StreamerIDs[i])
					return false
				}
			}

			// Verify other fields
			if retrieved.UserID != userID {
				t.Logf("UserID mismatch: expected %s, got %s", userID, retrieved.UserID)
				return false
			}

			return true
		},
		gen.SliceOf(gen.Identifier()).SuchThat(func(v []string) bool {
			// Ensure unique streamer IDs and reasonable size
			if len(v) == 0 || len(v) > 50 {
				return false
			}
			seen := make(map[string]bool)
			for _, id := range v {
				if seen[id] {
					return false
				}
				seen[id] = true
			}
			return true
		}),
	))

	properties.TestingRun(t)
}
