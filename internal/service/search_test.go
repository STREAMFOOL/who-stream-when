package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"who-live-when/internal/domain"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// mockSearchPlatformAdapter is a mock implementation for search testing
type mockSearchPlatformAdapter struct {
	results []*domain.PlatformStreamer
	err     error
}

func (m *mockSearchPlatformAdapter) GetLiveStatus(ctx context.Context, handle string) (*domain.PlatformLiveStatus, error) {
	return nil, nil
}

func (m *mockSearchPlatformAdapter) SearchStreamer(ctx context.Context, query string) ([]*domain.PlatformStreamer, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func (m *mockSearchPlatformAdapter) GetChannelInfo(ctx context.Context, handle string) (*domain.PlatformChannelInfo, error) {
	return nil, nil
}

// **Feature: streamer-tracking-mvp, Property 14: Multi-Platform Search Coverage**
// **Validates: Requirements 7.1, 7.4**
// Property: For any registered user search query, the system should query all three platforms (YouTube, Kick, Twitch) and return results from all available matches
func TestProperty_MultiPlatformSearchCoverage(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("search queries all three platforms and aggregates results", prop.ForAll(
		func(query string, youtubeCount, kickCount, twitchCount int) bool {
			// Normalize counts to reasonable range
			if youtubeCount < 0 {
				youtubeCount = 0
			}
			if youtubeCount > 10 {
				youtubeCount = 10
			}
			if kickCount < 0 {
				kickCount = 0
			}
			if kickCount > 10 {
				kickCount = 10
			}
			if twitchCount < 0 {
				twitchCount = 0
			}
			if twitchCount > 10 {
				twitchCount = 10
			}

			// Skip empty queries
			if strings.TrimSpace(query) == "" {
				return true
			}

			ctx := context.Background()

			// Create mock adapters with different result counts
			youtubeResults := make([]*domain.PlatformStreamer, youtubeCount)
			for i := 0; i < youtubeCount; i++ {
				youtubeResults[i] = &domain.PlatformStreamer{
					Handle:    fmt.Sprintf("yt_handle_%d", i),
					Name:      fmt.Sprintf("YouTube Streamer %d", i),
					Platform:  "youtube",
					Thumbnail: "https://example.com/yt.jpg",
				}
			}

			kickResults := make([]*domain.PlatformStreamer, kickCount)
			for i := 0; i < kickCount; i++ {
				kickResults[i] = &domain.PlatformStreamer{
					Handle:    fmt.Sprintf("kick_handle_%d", i),
					Name:      fmt.Sprintf("Kick Streamer %d", i),
					Platform:  "kick",
					Thumbnail: "https://example.com/kick.jpg",
				}
			}

			twitchResults := make([]*domain.PlatformStreamer, twitchCount)
			for i := 0; i < twitchCount; i++ {
				twitchResults[i] = &domain.PlatformStreamer{
					Handle:    fmt.Sprintf("twitch_handle_%d", i),
					Name:      fmt.Sprintf("Twitch Streamer %d", i),
					Platform:  "twitch",
					Thumbnail: "https://example.com/twitch.jpg",
				}
			}

			youtubeAdapter := &mockSearchPlatformAdapter{results: youtubeResults}
			kickAdapter := &mockSearchPlatformAdapter{results: kickResults}
			twitchAdapter := &mockSearchPlatformAdapter{results: twitchResults}

			service := NewSearchService(youtubeAdapter, kickAdapter, twitchAdapter)

			// Execute search
			results, err := service.SearchStreamers(ctx, query)
			if err != nil {
				return false
			}

			// Verify results contain entries from all platforms that had results
			expectedTotal := youtubeCount + kickCount + twitchCount
			if len(results) != expectedTotal {
				return false
			}

			// Count results by platform
			platformCounts := make(map[string]int)
			for _, result := range results {
				for _, platform := range result.Platforms {
					platformCounts[platform]++
				}
			}

			// Verify each platform's results are included
			if youtubeCount > 0 && platformCounts["youtube"] != youtubeCount {
				return false
			}
			if kickCount > 0 && platformCounts["kick"] != kickCount {
				return false
			}
			if twitchCount > 0 && platformCounts["twitch"] != twitchCount {
				return false
			}

			return true
		},
		gen.AnyString(),
		gen.IntRange(0, 10),
		gen.IntRange(0, 10),
		gen.IntRange(0, 10),
	))

	properties.TestingRun(t)
}

// **Feature: streamer-tracking-mvp, Property 15: Search Result Completeness**
// **Validates: Requirements 7.2**
// Property: For any search result, the returned data should include streamer name, handle, and all available platforms
func TestProperty_SearchResultCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all search results contain name, handle, and platforms", prop.ForAll(
		func(query string, numResults int) bool {
			// Normalize to reasonable range
			if numResults < 0 {
				numResults = 0
			}
			if numResults > 20 {
				numResults = 20
			}

			// Skip empty queries
			if strings.TrimSpace(query) == "" {
				return true
			}

			ctx := context.Background()

			// Create mock results with complete data
			platforms := []string{"youtube", "kick", "twitch"}
			allResults := make(map[string][]*domain.PlatformStreamer)

			for _, platform := range platforms {
				platformResults := make([]*domain.PlatformStreamer, numResults)
				for i := 0; i < numResults; i++ {
					platformResults[i] = &domain.PlatformStreamer{
						Handle:    fmt.Sprintf("%s_handle_%d", platform, i),
						Name:      fmt.Sprintf("Streamer %d", i),
						Platform:  platform,
						Thumbnail: fmt.Sprintf("https://example.com/%s_%d.jpg", platform, i),
					}
				}
				allResults[platform] = platformResults
			}

			youtubeAdapter := &mockSearchPlatformAdapter{results: allResults["youtube"]}
			kickAdapter := &mockSearchPlatformAdapter{results: allResults["kick"]}
			twitchAdapter := &mockSearchPlatformAdapter{results: allResults["twitch"]}

			service := NewSearchService(youtubeAdapter, kickAdapter, twitchAdapter)

			// Execute search
			results, err := service.SearchStreamers(ctx, query)
			if err != nil {
				return false
			}

			// Verify each result has complete data
			for _, result := range results {
				// Must have a name
				if result.Name == "" {
					return false
				}

				// Must have at least one platform
				if len(result.Platforms) == 0 {
					return false
				}

				// Must have handles for all listed platforms
				if len(result.Handles) == 0 {
					return false
				}

				// Each platform in Platforms must have a corresponding handle
				for _, platform := range result.Platforms {
					if handle, exists := result.Handles[platform]; !exists || handle == "" {
						return false
					}
				}

				// Each handle must correspond to a platform in Platforms
				for platform := range result.Handles {
					found := false
					for _, p := range result.Platforms {
						if p == platform {
							found = true
							break
						}
					}
					if !found {
						return false
					}
				}
			}

			return true
		},
		gen.AnyString(),
		gen.IntRange(0, 20),
	))

	properties.TestingRun(t)
}

// Test SearchStreamers with various queries
func TestSearchStreamers_VariousQueries(t *testing.T) {
	ctx := context.Background()

	// Create mock adapters with sample data
	youtubeAdapter := &mockSearchPlatformAdapter{
		results: []*domain.PlatformStreamer{
			{Handle: "yt1", Name: "Gaming Channel", Platform: "youtube", Thumbnail: "yt1.jpg"},
			{Handle: "yt2", Name: "Music Channel", Platform: "youtube", Thumbnail: "yt2.jpg"},
		},
	}

	kickAdapter := &mockSearchPlatformAdapter{
		results: []*domain.PlatformStreamer{
			{Handle: "kick1", Name: "Gaming Stream", Platform: "kick", Thumbnail: "kick1.jpg"},
		},
	}

	twitchAdapter := &mockSearchPlatformAdapter{
		results: []*domain.PlatformStreamer{
			{Handle: "twitch1", Name: "Pro Gamer", Platform: "twitch", Thumbnail: "twitch1.jpg"},
			{Handle: "twitch2", Name: "Music Stream", Platform: "twitch", Thumbnail: "twitch2.jpg"},
		},
	}

	service := NewSearchService(youtubeAdapter, kickAdapter, twitchAdapter)

	// Test with valid query
	results, err := service.SearchStreamers(ctx, "gaming")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should have results from all platforms
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}

	// Verify each result has required fields
	for _, result := range results {
		if result.Name == "" {
			t.Error("result missing name")
		}
		if len(result.Platforms) == 0 {
			t.Error("result missing platforms")
		}
		if len(result.Handles) == 0 {
			t.Error("result missing handles")
		}
	}
}

// Test SearchStreamers with empty query
func TestSearchStreamers_EmptyQuery(t *testing.T) {
	ctx := context.Background()

	youtubeAdapter := &mockSearchPlatformAdapter{results: []*domain.PlatformStreamer{}}
	kickAdapter := &mockSearchPlatformAdapter{results: []*domain.PlatformStreamer{}}
	twitchAdapter := &mockSearchPlatformAdapter{results: []*domain.PlatformStreamer{}}

	service := NewSearchService(youtubeAdapter, kickAdapter, twitchAdapter)

	// Test with empty query
	results, err := service.SearchStreamers(ctx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should return empty results
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(results))
	}
}

// Test deduplication across platforms
func TestSearchStreamers_Deduplication(t *testing.T) {
	ctx := context.Background()

	// Create mock adapters with duplicate streamers (same name, different platforms)
	youtubeAdapter := &mockSearchPlatformAdapter{
		results: []*domain.PlatformStreamer{
			{Handle: "yt_ninja", Name: "Ninja", Platform: "youtube", Thumbnail: "ninja_yt.jpg"},
		},
	}

	kickAdapter := &mockSearchPlatformAdapter{
		results: []*domain.PlatformStreamer{
			{Handle: "kick_ninja", Name: "Ninja", Platform: "kick", Thumbnail: "ninja_kick.jpg"},
		},
	}

	twitchAdapter := &mockSearchPlatformAdapter{
		results: []*domain.PlatformStreamer{
			{Handle: "twitch_ninja", Name: "Ninja", Platform: "twitch", Thumbnail: "ninja_twitch.jpg"},
		},
	}

	service := NewSearchService(youtubeAdapter, kickAdapter, twitchAdapter)

	// Execute search
	results, err := service.SearchStreamers(ctx, "ninja")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should deduplicate to single result with multiple platforms
	if len(results) != 1 {
		t.Fatalf("expected 1 deduplicated result, got %d", len(results))
	}

	result := results[0]
	if result.Name != "Ninja" {
		t.Errorf("expected name 'Ninja', got '%s'", result.Name)
	}

	// Should have all three platforms
	if len(result.Platforms) != 3 {
		t.Errorf("expected 3 platforms, got %d", len(result.Platforms))
	}

	// Should have handles for all platforms
	if len(result.Handles) != 3 {
		t.Errorf("expected 3 handles, got %d", len(result.Handles))
	}

	// Verify handles are correct
	if result.Handles["youtube"] != "yt_ninja" {
		t.Errorf("expected youtube handle 'yt_ninja', got '%s'", result.Handles["youtube"])
	}
	if result.Handles["kick"] != "kick_ninja" {
		t.Errorf("expected kick handle 'kick_ninja', got '%s'", result.Handles["kick"])
	}
	if result.Handles["twitch"] != "twitch_ninja" {
		t.Errorf("expected twitch handle 'twitch_ninja', got '%s'", result.Handles["twitch"])
	}
}

// Test edge case: no results found
func TestSearchStreamers_NoResults(t *testing.T) {
	ctx := context.Background()

	// Create mock adapters with no results
	youtubeAdapter := &mockSearchPlatformAdapter{results: []*domain.PlatformStreamer{}}
	kickAdapter := &mockSearchPlatformAdapter{results: []*domain.PlatformStreamer{}}
	twitchAdapter := &mockSearchPlatformAdapter{results: []*domain.PlatformStreamer{}}

	service := NewSearchService(youtubeAdapter, kickAdapter, twitchAdapter)

	// Execute search
	results, err := service.SearchStreamers(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should return empty results
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// Test partial platform failures
func TestSearchStreamers_PartialPlatformFailures(t *testing.T) {
	ctx := context.Background()

	// YouTube succeeds
	youtubeAdapter := &mockSearchPlatformAdapter{
		results: []*domain.PlatformStreamer{
			{Handle: "yt1", Name: "YouTube Streamer", Platform: "youtube", Thumbnail: "yt.jpg"},
		},
	}

	// Kick fails
	kickAdapter := &mockSearchPlatformAdapter{
		err: fmt.Errorf("kick api unavailable"),
	}

	// Twitch succeeds
	twitchAdapter := &mockSearchPlatformAdapter{
		results: []*domain.PlatformStreamer{
			{Handle: "twitch1", Name: "Twitch Streamer", Platform: "twitch", Thumbnail: "twitch.jpg"},
		},
	}

	service := NewSearchService(youtubeAdapter, kickAdapter, twitchAdapter)

	// Execute search - should succeed with partial results
	results, err := service.SearchStreamers(ctx, "test")
	if err != nil {
		t.Fatalf("expected no error with partial results, got %v", err)
	}

	// Should have results from successful platforms
	if len(results) != 2 {
		t.Errorf("expected 2 results from successful platforms, got %d", len(results))
	}
}

// Test all platforms fail
func TestSearchStreamers_AllPlatformsFail(t *testing.T) {
	ctx := context.Background()

	// All platforms fail
	youtubeAdapter := &mockSearchPlatformAdapter{err: fmt.Errorf("youtube unavailable")}
	kickAdapter := &mockSearchPlatformAdapter{err: fmt.Errorf("kick unavailable")}
	twitchAdapter := &mockSearchPlatformAdapter{err: fmt.Errorf("twitch unavailable")}

	service := NewSearchService(youtubeAdapter, kickAdapter, twitchAdapter)

	// Execute search - should return error
	_, err := service.SearchStreamers(ctx, "test")
	if err == nil {
		t.Fatal("expected error when all platforms fail")
	}
}

// Test case sensitivity in deduplication
func TestSearchStreamers_CaseInsensitiveDeduplication(t *testing.T) {
	ctx := context.Background()

	// Create mock adapters with same streamer but different case
	youtubeAdapter := &mockSearchPlatformAdapter{
		results: []*domain.PlatformStreamer{
			{Handle: "yt1", Name: "Streamer Name", Platform: "youtube", Thumbnail: "yt.jpg"},
		},
	}

	kickAdapter := &mockSearchPlatformAdapter{
		results: []*domain.PlatformStreamer{
			{Handle: "kick1", Name: "STREAMER NAME", Platform: "kick", Thumbnail: "kick.jpg"},
		},
	}

	twitchAdapter := &mockSearchPlatformAdapter{
		results: []*domain.PlatformStreamer{
			{Handle: "twitch1", Name: "streamer name", Platform: "twitch", Thumbnail: "twitch.jpg"},
		},
	}

	service := NewSearchService(youtubeAdapter, kickAdapter, twitchAdapter)

	// Execute search
	results, err := service.SearchStreamers(ctx, "streamer")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should deduplicate case-insensitively to single result
	if len(results) != 1 {
		t.Fatalf("expected 1 deduplicated result (case-insensitive), got %d", len(results))
	}

	// Should have all three platforms
	if len(results[0].Platforms) != 3 {
		t.Errorf("expected 3 platforms after case-insensitive deduplication, got %d", len(results[0].Platforms))
	}
}
