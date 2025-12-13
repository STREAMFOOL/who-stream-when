package seed

import (
	"context"
	"fmt"
	"log"
	"time"

	"who-live-when/internal/domain"
	"who-live-when/internal/repository"
)

// timeNow is a variable for testing purposes
var timeNow = time.Now

// PopularKickStreamers contains handles of popular Kick streamers to seed
var PopularKickStreamers = []string{
	"xqc",
	"adinross",
	"kaicenat",
	"amouranth",
	"trainwreckstv",
}

// Seeder handles database seeding with initial streamer data
type Seeder struct {
	streamerRepo repository.StreamerRepository
	kickAdapter  domain.PlatformAdapter
}

// NewSeeder creates a new Seeder instance
func NewSeeder(streamerRepo repository.StreamerRepository, kickAdapter domain.PlatformAdapter) *Seeder {
	return &Seeder{
		streamerRepo: streamerRepo,
		kickAdapter:  kickAdapter,
	}
}

// SeedResult contains the results of a seeding operation
type SeedResult struct {
	Created []string // Handles of newly created streamers
	Skipped []string // Handles of existing streamers (skipped)
	Failed  []string // Handles that failed to seed
	Errors  []error  // Errors encountered during seeding
}

// SeedPopularStreamers seeds the database with popular Kick streamers.
// This operation is idempotent - existing streamers are skipped.
func (s *Seeder) SeedPopularStreamers(
	ctx context.Context,
) (*SeedResult, error) {
	result := &SeedResult{
		Created: make([]string, 0),
		Skipped: make([]string, 0),
		Failed:  make([]string, 0),
		Errors:  make([]error, 0),
	}

	for _, handle := range PopularKickStreamers {
		created, err := s.seedStreamer(ctx, handle)
		if err != nil {
			result.Failed = append(result.Failed, handle)
			result.Errors = append(result.Errors, fmt.Errorf("failed to seed %s: %w", handle, err))
			log.Printf("Failed to seed streamer %s: %v", handle, err)
			continue
		}

		if created {
			result.Created = append(result.Created, handle)
		} else {
			result.Skipped = append(result.Skipped, handle)
		}
	}

	return result, nil
}

// seedStreamer seeds a single streamer by handle.
// Returns true if the streamer was created, false if it already existed.
func (s *Seeder) seedStreamer(ctx context.Context, handle string) (bool, error) {
	// Check if streamer already exists
	existing, err := s.streamerRepo.GetByPlatformHandle(ctx, "kick", handle)
	if err != nil {
		return false, fmt.Errorf("failed to check existing streamer: %w", err)
	}

	if existing != nil {
		return false, nil
	}

	// Fetch channel info from Kick API
	channelInfo, err := s.kickAdapter.GetChannelInfo(ctx, handle)
	if err != nil {
		return false, fmt.Errorf("failed to get channel info from Kick: %w", err)
	}

	// Create streamer in database
	streamer := &domain.Streamer{
		ID:        generateStreamerID(),
		Name:      channelInfo.Name,
		Platforms: []string{"kick"},
		Handles:   map[string]string{"kick": channelInfo.Handle},
	}

	if err := s.streamerRepo.Create(ctx, streamer); err != nil {
		return false, fmt.Errorf("failed to create streamer: %w", err)
	}

	log.Printf("Seeded streamer: %s (%s)", channelInfo.Name, handle)
	return true, nil
}

// generateStreamerID generates a unique ID for a new streamer
func generateStreamerID() string {
	return fmt.Sprintf("str_%d", timeNow().UnixNano())
}
