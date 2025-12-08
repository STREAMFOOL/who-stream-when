package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"who-live-when/internal/domain"
	"who-live-when/internal/logger"
)

// YouTubeAdapter implements PlatformAdapter for YouTube
type YouTubeAdapter struct {
	apiKey     string
	httpClient *http.Client
	logger     *logger.Logger
}

// NewYouTubeAdapter creates a new YouTube adapter
func NewYouTubeAdapter(apiKey string) *YouTubeAdapter {
	return &YouTubeAdapter{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger.Default(),
	}
}

// GetLiveStatus retrieves the live status for a YouTube channel.
// It uses the YouTube Data API v3 search endpoint with eventType=live to find
// currently streaming videos. The handle parameter should be a YouTube channel ID.
func (y *YouTubeAdapter) GetLiveStatus(ctx context.Context, handle string) (*domain.PlatformLiveStatus, error) {
	baseURL := "https://www.googleapis.com/youtube/v3/search"
	params := url.Values{}
	params.Add("part", "snippet")
	params.Add("channelId", handle)
	params.Add("eventType", "live") // Only return live streams
	params.Add("type", "video")
	params.Add("key", y.apiKey)

	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := y.httpClient.Do(req)
	if err != nil {
		y.logger.Error("YouTube GetLiveStatus API request failed", map[string]interface{}{
			"handle": handle,
			"error":  err.Error(),
		})
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		y.logger.Warn("YouTube GetLiveStatus API returned non-OK status", map[string]interface{}{
			"handle":      handle,
			"status_code": resp.StatusCode,
			"response":    string(body),
		})
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
		y.logger.Error("YouTube GetLiveStatus failed to decode response", map[string]interface{}{
			"handle": handle,
			"error":  err.Error(),
		})
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Empty results means the channel is not currently streaming
	if len(result.Items) == 0 {
		return &domain.PlatformLiveStatus{
			IsLive: false,
		}, nil
	}

	item := result.Items[0]
	thumbnail := ""
	if thumb, ok := item.Snippet.Thumbnails["medium"]; ok {
		thumbnail = thumb.URL
	}

	// Viewer count requires a separate API call to the videos endpoint
	viewerCount, _ := y.getViewerCount(ctx, item.ID.VideoID)

	return &domain.PlatformLiveStatus{
		IsLive:      true,
		StreamURL:   fmt.Sprintf("https://www.youtube.com/watch?v=%s", item.ID.VideoID),
		Title:       item.Snippet.Title,
		Thumbnail:   thumbnail,
		ViewerCount: viewerCount,
	}, nil
}

// getViewerCount retrieves the current viewer count for a live video.
// This uses the videos endpoint with liveStreamingDetails part to get concurrent viewers.
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
		y.logger.Error("YouTube SearchStreamer API request failed", map[string]interface{}{
			"query": query,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		y.logger.Warn("YouTube SearchStreamer API returned non-OK status", map[string]interface{}{
			"query":       query,
			"status_code": resp.StatusCode,
			"response":    string(body),
		})
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
		y.logger.Error("YouTube SearchStreamer failed to decode response", map[string]interface{}{
			"query": query,
			"error": err.Error(),
		})
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
		y.logger.Error("YouTube GetChannelInfo API request failed", map[string]interface{}{
			"handle": handle,
			"error":  err.Error(),
		})
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		y.logger.Warn("YouTube GetChannelInfo API returned non-OK status", map[string]interface{}{
			"handle":      handle,
			"status_code": resp.StatusCode,
			"response":    string(body),
		})
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
		y.logger.Error("YouTube GetChannelInfo failed to decode response", map[string]interface{}{
			"handle": handle,
			"error":  err.Error(),
		})
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Items) == 0 {
		y.logger.Warn("YouTube channel not found", map[string]interface{}{
			"handle": handle,
		})
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
