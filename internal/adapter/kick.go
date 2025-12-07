package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/user/who-live-when/internal/domain"
)

// KickAdapter implements PlatformAdapter for Kick
type KickAdapter struct {
	httpClient *http.Client
}

// NewKickAdapter creates a new Kick adapter
func NewKickAdapter() *KickAdapter {
	return &KickAdapter{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetLiveStatus retrieves the live status for a Kick channel
func (k *KickAdapter) GetLiveStatus(ctx context.Context, handle string) (*domain.PlatformLiveStatus, error) {
	// Kick API endpoint for channel info
	url := fmt.Sprintf("https://kick.com/api/v2/channels/%s", handle)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("channel not found")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kick api returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Livestream *struct {
			ID           int    `json:"id"`
			SessionTitle string `json:"session_title"`
			Thumbnail    struct {
				URL string `json:"url"`
			} `json:"thumbnail"`
			ViewerCount int `json:"viewer_count"`
		} `json:"livestream"`
		Slug string `json:"slug"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If livestream is nil, the channel is offline
	if result.Livestream == nil {
		return &domain.PlatformLiveStatus{
			IsLive: false,
		}, nil
	}

	return &domain.PlatformLiveStatus{
		IsLive:      true,
		StreamURL:   fmt.Sprintf("https://kick.com/%s", result.Slug),
		Title:       result.Livestream.SessionTitle,
		Thumbnail:   result.Livestream.Thumbnail.URL,
		ViewerCount: result.Livestream.ViewerCount,
	}, nil
}

// SearchStreamer searches for streamers on Kick
func (k *KickAdapter) SearchStreamer(ctx context.Context, query string) ([]*domain.PlatformStreamer, error) {
	// Kick API endpoint for search
	url := fmt.Sprintf("https://kick.com/api/search?searched_word=%s", query)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kick api returned status %d: %s", resp.StatusCode, string(body))
	}

	var result []struct {
		Slug       string `json:"slug"`
		Username   string `json:"username"`
		ProfilePic string `json:"profile_pic"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	streamers := make([]*domain.PlatformStreamer, 0, len(result))
	for _, item := range result {
		streamers = append(streamers, &domain.PlatformStreamer{
			Handle:    item.Slug,
			Name:      item.Username,
			Platform:  "kick",
			Thumbnail: item.ProfilePic,
		})
	}

	return streamers, nil
}

// GetChannelInfo retrieves detailed information about a Kick channel
func (k *KickAdapter) GetChannelInfo(ctx context.Context, handle string) (*domain.PlatformChannelInfo, error) {
	url := fmt.Sprintf("https://kick.com/api/v2/channels/%s", handle)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("channel not found")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kick api returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Slug string `json:"slug"`
		User struct {
			Username   string `json:"username"`
			Bio        string `json:"bio"`
			ProfilePic string `json:"profile_pic"`
		} `json:"user"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &domain.PlatformChannelInfo{
		Handle:      result.Slug,
		Name:        result.User.Username,
		Description: result.User.Bio,
		Thumbnail:   result.User.ProfilePic,
		Platform:    "kick",
	}, nil
}
