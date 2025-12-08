package sqlite

import (
	"context"
	"os"
	"testing"
	"time"

	"who-live-when/internal/domain"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// setupTestDB creates a temporary test database
func setupTestDB(t *testing.T) *DB {
	t.Helper()

	// Create temporary database file
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()

	// Open database
	db, err := NewDB(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("failed to open database: %v", err)
	}

	// Run migrations
	if err := Migrate(db.DB); err != nil {
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

// genStreamer generates random streamers for property testing
func genStreamer() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),
		gen.Identifier(),
		genPlatformHandles(),
	).Map(func(values []interface{}) *domain.Streamer {
		id := values[0].(string)
		name := values[1].(string)
		handles := values[2].(map[string]string)

		platforms := make([]string, 0, len(handles))
		for platform := range handles {
			platforms = append(platforms, platform)
		}

		now := time.Now()
		return &domain.Streamer{
			ID:        id,
			Name:      name,
			Handles:   handles,
			Platforms: platforms,
			CreatedAt: now,
			UpdatedAt: now,
		}
	})
}

// genPlatformHandles generates random platform->handle mappings
func genPlatformHandles() gopter.Gen {
	platforms := []string{"youtube", "twitch", "kick"}
	return gen.MapOf(
		gen.OneConstOf(platforms[0], platforms[1], platforms[2]),
		gen.Identifier(),
	).SuchThat(func(v interface{}) bool {
		m := v.(map[string]string)
		return len(m) > 0 && len(m) <= 3
	})
}

// **Feature: streamer-tracking-mvp, Property 1: Streamer Data Persistence**
// **Validates: Requirements 1.1, 1.3**
// For any streamer with valid name, handles, and platforms, adding it to the system
// and retrieving it should return identical data.
func TestProperty_StreamerDataPersistence(t *testing.T) {
	db := setupTestDB(t)
	repo := NewStreamerRepository(db)
	ctx := context.Background()

	properties := gopter.NewProperties(nil)

	properties.Property("streamer round-trip preserves data", prop.ForAll(
		func(streamer *domain.Streamer) bool {
			// Create streamer
			if err := repo.Create(ctx, streamer); err != nil {
				t.Logf("failed to create streamer: %v", err)
				return false
			}

			// Retrieve streamer
			retrieved, err := repo.GetByID(ctx, streamer.ID)
			if err != nil {
				t.Logf("failed to retrieve streamer: %v", err)
				return false
			}

			// Verify data matches
			if retrieved.ID != streamer.ID {
				t.Logf("ID mismatch: expected %s, got %s", streamer.ID, retrieved.ID)
				return false
			}

			if retrieved.Name != streamer.Name {
				t.Logf("Name mismatch: expected %s, got %s", streamer.Name, retrieved.Name)
				return false
			}

			// Verify handles match
			if len(retrieved.Handles) != len(streamer.Handles) {
				t.Logf("Handles length mismatch: expected %d, got %d", len(streamer.Handles), len(retrieved.Handles))
				return false
			}

			for platform, handle := range streamer.Handles {
				if retrievedHandle, ok := retrieved.Handles[platform]; !ok || retrievedHandle != handle {
					t.Logf("Handle mismatch for platform %s: expected %s, got %s", platform, handle, retrievedHandle)
					return false
				}
			}

			// Verify platforms match
			if len(retrieved.Platforms) != len(streamer.Platforms) {
				t.Logf("Platforms length mismatch: expected %d, got %d", len(streamer.Platforms), len(retrieved.Platforms))
				return false
			}

			platformSet := make(map[string]bool)
			for _, p := range streamer.Platforms {
				platformSet[p] = true
			}
			for _, p := range retrieved.Platforms {
				if !platformSet[p] {
					t.Logf("Platform %s not found in original platforms", p)
					return false
				}
			}

			// Clean up for next iteration
			if err := repo.Delete(ctx, streamer.ID); err != nil {
				t.Logf("failed to delete streamer: %v", err)
				return false
			}

			return true
		},
		genStreamer(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: streamer-tracking-mvp, Property 2: Streamer Update Consistency**
// **Validates: Requirements 1.2**
// For any existing streamer, updating its platform information and retrieving it
// should reflect the updated platforms.
func TestProperty_StreamerUpdateConsistency(t *testing.T) {
	db := setupTestDB(t)
	repo := NewStreamerRepository(db)
	ctx := context.Background()

	properties := gopter.NewProperties(nil)

	properties.Property("streamer update reflects changes", prop.ForAll(
		func(original *domain.Streamer, newHandles map[string]string) bool {
			// Create original streamer
			if err := repo.Create(ctx, original); err != nil {
				t.Logf("failed to create streamer: %v", err)
				return false
			}

			// Update platforms
			updated := &domain.Streamer{
				ID:        original.ID,
				Name:      original.Name,
				Handles:   newHandles,
				Platforms: make([]string, 0, len(newHandles)),
				CreatedAt: original.CreatedAt,
				UpdatedAt: time.Now(),
			}
			for platform := range newHandles {
				updated.Platforms = append(updated.Platforms, platform)
			}

			if err := repo.Update(ctx, updated); err != nil {
				t.Logf("failed to update streamer: %v", err)
				return false
			}

			// Retrieve and verify
			retrieved, err := repo.GetByID(ctx, original.ID)
			if err != nil {
				t.Logf("failed to retrieve streamer: %v", err)
				return false
			}

			// Verify handles match updated values
			if len(retrieved.Handles) != len(newHandles) {
				t.Logf("Handles length mismatch: expected %d, got %d", len(newHandles), len(retrieved.Handles))
				return false
			}

			for platform, handle := range newHandles {
				if retrievedHandle, ok := retrieved.Handles[platform]; !ok || retrievedHandle != handle {
					t.Logf("Handle mismatch for platform %s: expected %s, got %s", platform, handle, retrievedHandle)
					return false
				}
			}

			// Clean up
			if err := repo.Delete(ctx, original.ID); err != nil {
				t.Logf("failed to delete streamer: %v", err)
				return false
			}

			return true
		},
		genStreamer(),
		genPlatformHandles(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: streamer-tracking-mvp, Property 3: Multi-Platform Handle Isolation**
// **Validates: Requirements 1.4, 10.1**
// For any streamer with handles on multiple platforms, each platform's handle
// should be stored and retrieved independently without cross-contamination.
func TestProperty_MultiPlatformHandleIsolation(t *testing.T) {
	db := setupTestDB(t)
	repo := NewStreamerRepository(db)
	ctx := context.Background()

	properties := gopter.NewProperties(nil)

	properties.Property("platform handles remain isolated", prop.ForAll(
		func(streamer *domain.Streamer) bool {
			// Only test streamers with multiple platforms
			if len(streamer.Handles) < 2 {
				return true // Skip single-platform streamers
			}

			// Create streamer
			if err := repo.Create(ctx, streamer); err != nil {
				t.Logf("failed to create streamer: %v", err)
				return false
			}

			// Retrieve and verify each platform independently
			retrieved, err := repo.GetByID(ctx, streamer.ID)
			if err != nil {
				t.Logf("failed to retrieve streamer: %v", err)
				return false
			}

			// Verify each platform's handle is correct and isolated
			for platform, expectedHandle := range streamer.Handles {
				actualHandle, exists := retrieved.Handles[platform]
				if !exists {
					t.Logf("Platform %s not found in retrieved handles", platform)
					return false
				}

				if actualHandle != expectedHandle {
					t.Logf("Handle mismatch for platform %s: expected %s, got %s", platform, expectedHandle, actualHandle)
					return false
				}

				// Verify this handle doesn't appear for other platforms
				for otherPlatform, otherHandle := range retrieved.Handles {
					if otherPlatform != platform && otherHandle == expectedHandle {
						// This is only a problem if the original didn't have the same handle
						if streamer.Handles[otherPlatform] != expectedHandle {
							t.Logf("Handle contamination: %s appears for both %s and %s", expectedHandle, platform, otherPlatform)
							return false
						}
					}
				}
			}

			// Clean up
			if err := repo.Delete(ctx, streamer.ID); err != nil {
				t.Logf("failed to delete streamer: %v", err)
				return false
			}

			return true
		},
		genStreamer(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
