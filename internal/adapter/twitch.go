package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"who-live-when/internal/domain"
)

// TwitchAdapter implements PlatformAdapter for Twitch
type TwitchAdapter struct {
	clientID     string
	clientSecret string
	accessToken  string
	httpClient   *http.Client
}

// NewTwitchAdapter creates a new Twitch adapter
func NewTwitchAdapter(clientID, clientSecret string) *TwitchAdapter {
	return &TwitchAdapter{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ensureAccessToken ensures we have a valid access token for Twitch API calls.
// Twitch requires OAuth 2.0 client credentials flow for app access tokens.
// The token is cached in memory and reused for subsequent requests.
// Note: Tokens expire after ~60 days but automatic refresh is not implemented.
func (t *TwitchAdapter) ensureAccessToken(ctx context.Context) error {
	if t.accessToken != "" {
		return nil
	}

	url := fmt.Sprintf("https://id.twitch.tv/oauth2/token?client_id=%s&client_secret=%s&grant_type=client_credentials",
		t.clientID, t.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token request returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	t.accessToken = result.AccessToken
	return nil
}

// GetLiveStatus retrieves the live status for a Twitch channel.
// Twitch API requires a two-step process: first convert username to user ID,
// then query the streams endpoint. The handle parameter should be a Twitch username.
func (t *TwitchAdapter) GetLiveStatus(ctx context.Context, handle string) (*domain.PlatformLiveStatus, error) {
	if err := t.ensureAccessToken(ctx); err != nil {
		return nil, err
	}

	// Twitch API uses numeric user IDs, not usernames
	userID, err := t.getUserID(ctx, handle)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.twitch.tv/helix/streams?user_id=%s", userID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Client-ID", t.clientID)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.accessToken))

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("twitch api returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			UserLogin    string `json:"user_login"`
			Title        string `json:"title"`
			ThumbnailURL string `json:"thumbnail_url"`
			ViewerCount  int    `json:"viewer_count"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Empty data array means the channel is not currently streaming
	if len(result.Data) == 0 {
		return &domain.PlatformLiveStatus{
			IsLive: false,
		}, nil
	}

	stream := result.Data[0]
	return &domain.PlatformLiveStatus{
		IsLive:      true,
		StreamURL:   fmt.Sprintf("https://www.twitch.tv/%s", stream.UserLogin),
		Title:       stream.Title,
		Thumbnail:   stream.ThumbnailURL,
		ViewerCount: stream.ViewerCount,
	}, nil
}

// getUserID retrieves the numeric user ID for a given username.
// Twitch's Helix API requires user IDs for most operations, not usernames.
func (t *TwitchAdapter) getUserID(ctx context.Context, username string) (string, error) {
	url := fmt.Sprintf("https://api.twitch.tv/helix/users?login=%s", username)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Client-ID", t.clientID)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.accessToken))

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("twitch api returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Data) == 0 {
		return "", fmt.Errorf("user not found")
	}

	return result.Data[0].ID, nil
}

// SearchStreamer searches for streamers on Twitch
func (t *TwitchAdapter) SearchStreamer(ctx context.Context, query string) ([]*domain.PlatformStreamer, error) {
	if err := t.ensureAccessToken(ctx); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.twitch.tv/helix/search/channels?query=%s", query)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Client-ID", t.clientID)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.accessToken))

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("twitch api returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			BroadcasterLogin string `json:"broadcaster_login"`
			DisplayName      string `json:"display_name"`
			ThumbnailURL     string `json:"thumbnail_url"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	streamers := make([]*domain.PlatformStreamer, 0, len(result.Data))
	for _, item := range result.Data {
		streamers = append(streamers, &domain.PlatformStreamer{
			Handle:    item.BroadcasterLogin,
			Name:      item.DisplayName,
			Platform:  "twitch",
			Thumbnail: item.ThumbnailURL,
		})
	}

	return streamers, nil
}

// GetChannelInfo retrieves detailed information about a Twitch channel
func (t *TwitchAdapter) GetChannelInfo(ctx context.Context, handle string) (*domain.PlatformChannelInfo, error) {
	if err := t.ensureAccessToken(ctx); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.twitch.tv/helix/users?login=%s", handle)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Client-ID", t.clientID)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.accessToken))

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("twitch api returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			Login           string `json:"login"`
			DisplayName     string `json:"display_name"`
			Description     string `json:"description"`
			ProfileImageURL string `json:"profile_image_url"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("channel not found")
	}

	user := result.Data[0]
	return &domain.PlatformChannelInfo{
		Handle:      user.Login,
		Name:        user.DisplayName,
		Description: user.Description,
		Thumbnail:   user.ProfileImageURL,
		Platform:    "twitch",
	}, nil
}
