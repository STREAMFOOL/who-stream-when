package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"who-live-when/internal/domain"
)

// SearchService handles multi-platform streamer search
type SearchService struct {
	youtubeAdapter domain.PlatformAdapter
	kickAdapter    domain.PlatformAdapter
	twitchAdapter  domain.PlatformAdapter
}

// NewSearchService creates a new SearchService instance
func NewSearchService(
	youtubeAdapter domain.PlatformAdapter,
	kickAdapter domain.PlatformAdapter,
	twitchAdapter domain.PlatformAdapter,
) *SearchService {
	return &SearchService{
		youtubeAdapter: youtubeAdapter,
		kickAdapter:    kickAdapter,
		twitchAdapter:  twitchAdapter,
	}
}

// SearchResult represents a search result with platform information
type SearchResult struct {
	Name      string
	Handles   map[string]string // Platform -> handle mapping
	Platforms []string
	Thumbnail string
}

// SearchStreamers queries all platform adapters and aggregates results
func (s *SearchService) SearchStreamers(ctx context.Context, query string) ([]*SearchResult, error) {
	if query == "" {
		return []*SearchResult{}, nil
	}

	// Query all platforms in parallel
	type platformResult struct {
		platform  string
		streamers []*domain.PlatformStreamer
		err       error
	}

	resultsChan := make(chan platformResult, 3)
	var wg sync.WaitGroup

	// Query YouTube
	wg.Add(1)
	go func() {
		defer wg.Done()
		streamers, err := s.youtubeAdapter.SearchStreamer(ctx, query)
		resultsChan <- platformResult{platform: "youtube", streamers: streamers, err: err}
	}()

	// Query Kick
	wg.Add(1)
	go func() {
		defer wg.Done()
		streamers, err := s.kickAdapter.SearchStreamer(ctx, query)
		resultsChan <- platformResult{platform: "kick", streamers: streamers, err: err}
	}()

	// Query Twitch
	wg.Add(1)
	go func() {
		defer wg.Done()
		streamers, err := s.twitchAdapter.SearchStreamer(ctx, query)
		resultsChan <- platformResult{platform: "twitch", streamers: streamers, err: err}
	}()

	// Wait for all queries to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results from all platforms
	allResults := make(map[string][]*domain.PlatformStreamer)
	var errors []error

	for result := range resultsChan {
		if result.err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", result.platform, result.err))
			continue
		}
		allResults[result.platform] = result.streamers
	}

	// If all platforms failed, return error
	if len(errors) == 3 {
		return nil, fmt.Errorf("all platforms failed: %v", errors)
	}

	// Deduplicate and aggregate results
	return s.deduplicateResults(allResults), nil
}

// deduplicateResults combines results from multiple platforms and deduplicates by name
func (s *SearchService) deduplicateResults(platformResults map[string][]*domain.PlatformStreamer) []*SearchResult {
	// Use normalized name as key for deduplication
	resultMap := make(map[string]*SearchResult)

	for platform, streamers := range platformResults {
		for _, streamer := range streamers {
			normalizedName := strings.ToLower(strings.TrimSpace(streamer.Name))

			if existing, found := resultMap[normalizedName]; found {
				// Streamer already exists, add this platform
				existing.Platforms = append(existing.Platforms, platform)
				existing.Handles[platform] = streamer.Handle
				// Keep first thumbnail if current is empty
				if existing.Thumbnail == "" && streamer.Thumbnail != "" {
					existing.Thumbnail = streamer.Thumbnail
				}
			} else {
				// New streamer
				resultMap[normalizedName] = &SearchResult{
					Name:      streamer.Name,
					Handles:   map[string]string{platform: streamer.Handle},
					Platforms: []string{platform},
					Thumbnail: streamer.Thumbnail,
				}
			}
		}
	}

	// Convert map to slice
	results := make([]*SearchResult, 0, len(resultMap))
	for _, result := range resultMap {
		results = append(results, result)
	}

	return results
}
