package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"who-live-when/internal/domain"
)

// KickAdapter implements PlatformAdapter for Kick
type KickAdapter struct {
	httpClient   *http.Client
	clientID     string
	clientSecret string

	tokenMu     sync.RWMutex
	accessToken string
	tokenExpiry time.Time
}

// NewKickAdapter creates a new Kick adapter
func NewKickAdapter(clientID, clientSecret string) *KickAdapter {
	return &KickAdapter{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// getAccessToken returns a valid access token, fetching a new one if needed
func (k *KickAdapter) getAccessToken(ctx context.Context) (string, error) {
	k.tokenMu.RLock()
	if k.accessToken != "" && time.Now().Before(k.tokenExpiry) {
		token := k.accessToken
		k.tokenMu.RUnlock()
		return token, nil
	}
	k.tokenMu.RUnlock()

	k.tokenMu.Lock()
	defer k.tokenMu.Unlock()

	// Double-check after acquiring write lock
	if k.accessToken != "" && time.Now().Before(k.tokenExpiry) {
		return k.accessToken, nil
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", k.clientID)
	data.Set("client_secret", k.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://id.kick.com/oauth/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	k.accessToken = tokenResp.AccessToken
	// Expire 60 seconds early to avoid edge cases
	k.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return k.accessToken, nil
}

// GetLiveStatus retrieves the live status for a Kick channel.
// Kick's API returns channel info with an optional livestream object.
// If the livestream field is null, the channel is offline.
// The handle parameter should be a Kick slug (username).
func (k *KickAdapter) GetLiveStatus(ctx context.Context, handle string) (*domain.PlatformLiveStatus, error) {
	url := fmt.Sprintf("https://kick.com/api/v2/channels/%s", handle)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if err := k.setAuthHeaders(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to set auth headers: %w", err)
	}

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

	// Kick indicates offline status by returning null for the livestream field
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
	url := fmt.Sprintf("https://kick.com/api/search?searched_word=%s", query)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if err := k.setAuthHeaders(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to set auth headers: %w", err)
	}

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
	if err := k.setAuthHeaders(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to set auth headers: %w", err)
	}

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

// setAuthHeaders adds authentication headers to the request using OAuth2 token
func (k *KickAdapter) setAuthHeaders(ctx context.Context, req *http.Request) error {
	if k.clientID == "" || k.clientSecret == "" {
		return nil
	}

	token, err := k.getAccessToken(ctx)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	return nil
}

// CheckConnection verifies that the Kick API is reachable and credentials are valid.
// Returns nil if connection is successful, error otherwise.
func (k *KickAdapter) CheckConnection(ctx context.Context) error {
	// First, verify we can get an access token
	if k.clientID != "" && k.clientSecret != "" {
		if _, err := k.getAccessToken(ctx); err != nil {
			return fmt.Errorf("kick API authentication failed: %w", err)
		}
	}

	url := "https://api.kick.com/public/v1/channels?broadcaster_user_id=1"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if err := k.setAuthHeaders(ctx, req); err != nil {
		return fmt.Errorf("failed to set auth headers: %w", err)
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("kick API unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("kick API authentication failed (status %d)", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("kick API returned unexpected status: %d", resp.StatusCode)
	}

	return nil
}
