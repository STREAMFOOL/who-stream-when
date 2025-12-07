package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/user/who-live-when/internal/domain"
)

// YouTubeAdapter implements PlatformAdapter for YouTube
type YouTubeAdapter struct {
	apiKey     string
	httpClient *http.Client
}

// NewYouTubeAdapter creates a new YouTube adapter
func NewYouTubeAdapter(apiKey string) *YouTubeAdapter {
	return &YouTubeAdapter{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetLiveStatus retrieves the live status for a YouTube channel
func (y *YouTubeAdapter) GetLiveStatus(ctx context.Context, handle string) (*domain.PlatformLiveStatus, error) {
	// YouTube API endpoint for search
	baseURL := "https://www.googleapis.com/youtube/v3/search"
	params := url.Values{}
	params.Add("part", "snippet")
	params.Add("channelId", handle)
	params.Add("eventType", "live")
	params.Add("type", "video")
	params.Add("key", y.apiKey)

	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := y.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("youtube api returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
			Snippet struct {
				Title      string `json:"title"`
				Thumbnails map[string]struct {
					URL string `json:"url"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If no live streams found, return offline status
	if len(result.Items) == 0 {
		return &domain.PlatformLiveStatus{
			IsLive: false,
		}, nil
	}

	// Get the first live stream
	item := result.Items[0]
	thumbnail := ""
	if thumb, ok := item.Snippet.Thumbnails["medium"]; ok {
		thumbnail = thumb.URL
	}

	// Get viewer count from video details
	viewerCount, _ := y.getViewerCount(ctx, item.ID.VideoID)

	return &domain.PlatformLiveStatus{
		IsLive:      true,
		StreamURL:   fmt.Sprintf("https://www.youtube.com/watch?v=%s", item.ID.VideoID),
		Title:       item.Snippet.Title,
		Thumbnail:   thumbnail,
		ViewerCount: viewerCount,
	}, nil
}

// getViewerCount retrieves the current viewer count for a live video
func (y *YouTubeAdapter) getViewerCount(ctx context.Context, videoID string) (int, error) {
	baseURL := "https://www.googleapis.com/youtube/v3/videos"
	params := url.Values{}
	params.Add("part", "liveStreamingDetails")
	params.Add("id", videoID)
	params.Add("key", y.apiKey)

	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return 0, err
	}

	resp, err := y.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}

	var result struct {
		Items []struct {
			LiveStreamingDetails struct {
				ConcurrentViewers string `json:"concurrentViewers"`
			} `json:"liveStreamingDetails"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Items) == 0 {
		return 0, nil
	}

	var count int
	fmt.Sscanf(result.Items[0].LiveStreamingDetails.ConcurrentViewers, "%d", &count)
	return count, nil
}

// SearchStreamer searches for streamers on YouTube
func (y *YouTubeAdapter) SearchStreamer(ctx context.Context, query string) ([]*domain.PlatformStreamer, error) {
	baseURL := "https://www.googleapis.com/youtube/v3/search"
	params := url.Values{}
	params.Add("part", "snippet")
	params.Add("q", query)
	params.Add("type", "channel")
	params.Add("maxResults", "10")
	params.Add("key", y.apiKey)

	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := y.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("youtube api returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			ID struct {
				ChannelID string `json:"channelId"`
			} `json:"id"`
			Snippet struct {
				Title      string `json:"title"`
				Thumbnails map[string]struct {
					URL string `json:"url"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	streamers := make([]*domain.PlatformStreamer, 0, len(result.Items))
	for _, item := range result.Items {
		thumbnail := ""
		if thumb, ok := item.Snippet.Thumbnails["default"]; ok {
			thumbnail = thumb.URL
		}

		streamers = append(streamers, &domain.PlatformStreamer{
			Handle:    item.ID.ChannelID,
			Name:      item.Snippet.Title,
			Platform:  "youtube",
			Thumbnail: thumbnail,
		})
	}

	return streamers, nil
}

// GetChannelInfo retrieves detailed information about a YouTube channel
func (y *YouTubeAdapter) GetChannelInfo(ctx context.Context, handle string) (*domain.PlatformChannelInfo, error) {
	baseURL := "https://www.googleapis.com/youtube/v3/channels"
	params := url.Values{}
	params.Add("part", "snippet")
	params.Add("id", handle)
	params.Add("key", y.apiKey)

	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := y.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("youtube api returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				Thumbnails  map[string]struct {
					URL string `json:"url"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("channel not found")
	}

	item := result.Items[0]
	thumbnail := ""
	if thumb, ok := item.Snippet.Thumbnails["medium"]; ok {
		thumbnail = thumb.URL
	}

	return &domain.PlatformChannelInfo{
		Handle:      item.ID,
		Name:        item.Snippet.Title,
		Description: item.Snippet.Description,
		Thumbnail:   thumbnail,
		Platform:    "youtube",
	}, nil
}
