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

// **Feature: user-experience-enhancements, Property 1: Multi-Platform Search Coverage**
// **Validates: Requirements 1.2**
// Property: For any search query, the search service should query all three platforms
// (YouTube, Kick, Twitch) and aggregate results from all available matches.
func TestProperty_UXE_MultiPlatformSearchCoverage(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("search queries all three platforms and aggregates results", prop.ForAll(
		func(query string, youtubeCount, kickCount, twitchCount int) bool {
			// Normalize counts to reasonable range (0-5)
			youtubeCount = normalizeCount(youtubeCount)
			kickCount = normalizeCount(kickCount)
			twitchCount = normalizeCount(twitchCount)

			// Skip empty or whitespace-only queries
			if strings.TrimSpace(query) == "" {
				return true
			}

			ctx := context.Background()

			// Create mock adapters with unique results per platform
			youtubeAdapter := createMockAdapter("youtube", youtubeCount)
			kickAdapter := createMockAdapter("kick", kickCount)
			twitchAdapter := createMockAdapter("twitch", twitchCount)

			service := NewSearchService(youtubeAdapter, kickAdapter, twitchAdapter)

			results, err := service.SearchStreamers(ctx, query)
			if err != nil {
				return false
			}

			// Count results by platform
			platformCounts := countResultsByPlatform(results)

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

			// Total results should match sum of all platform results
			expectedTotal := youtubeCount + kickCount + twitchCount
			return len(results) == expectedTotal
		},
		gen.AnyString(),
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
	))

	properties.TestingRun(t)
}

// **Feature: user-experience-enhancements, Property 2: Search Result Completeness**
// **Validates: Requirements 1.3**
// Property: For any search result returned, it should contain streamer name, handle,
// and platform information fields (live status is fetched separately per-streamer).
func TestProperty_UXE_SearchResultCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all search results contain name, handle, and platform information", prop.ForAll(
		func(query string, numResults int) bool {
			// Normalize to reasonable range
			numResults = normalizeCount(numResults)

			// Skip empty or whitespace-only queries
			if strings.TrimSpace(query) == "" {
				return true
			}

			ctx := context.Background()

			// Create mock adapters with complete data
			youtubeAdapter := createMockAdapter("youtube", numResults)
			kickAdapter := createMockAdapter("kick", numResults)
			twitchAdapter := createMockAdapter("twitch", numResults)

			service := NewSearchService(youtubeAdapter, kickAdapter, twitchAdapter)

			results, err := service.SearchStreamers(ctx, query)
			if err != nil {
				return false
			}

			// Verify each result has complete data
			for _, result := range results {
				// Must have a non-empty name
				if result.Name == "" {
					return false
				}

				// Must have at least one platform
				if len(result.Platforms) == 0 {
					return false
				}

				// Must have handles map
				if result.Handles == nil || len(result.Handles) == 0 {
					return false
				}

				// Each platform in Platforms must have a corresponding handle
				for _, platform := range result.Platforms {
					handle, exists := result.Handles[platform]
					if !exists || handle == "" {
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
		gen.IntRange(0, 10),
	))

	properties.TestingRun(t)
}

// Helper functions

func normalizeCount(count int) int {
	if count < 0 {
		return 0
	}
	if count > 10 {
		return 10
	}
	return count
}

func createMockAdapter(platform string, count int) *mockSearchPlatformAdapter {
	results := make([]*domain.PlatformStreamer, count)
	for i := 0; i < count; i++ {
		results[i] = &domain.PlatformStreamer{
			Handle:    fmt.Sprintf("%s_handle_%d", platform, i),
			Name:      fmt.Sprintf("%s Streamer %d", platform, i),
			Platform:  platform,
			Thumbnail: fmt.Sprintf("https://example.com/%s_%d.jpg", platform, i),
		}
	}
	return &mockSearchPlatformAdapter{results: results}
}

func countResultsByPlatform(results []*SearchResult) map[string]int {
	counts := make(map[string]int)
	for _, result := range results {
		for _, platform := range result.Platforms {
			counts[platform]++
		}
	}
	return counts
}
