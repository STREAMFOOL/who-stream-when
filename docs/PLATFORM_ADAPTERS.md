# Platform Adapter Implementation Guide

This document describes how platform adapters work and how to add new streaming platforms.

## Overview

Platform adapters abstract the differences between streaming platform APIs, providing a unified interface for querying live status, searching streamers, and retrieving channel information.

## Architecture

All platform adapters implement the `PlatformAdapter` interface defined in `internal/domain/interfaces.go`:

```go
type PlatformAdapter interface {
    GetLiveStatus(ctx context.Context, handle string) (*PlatformLiveStatus, error)
    SearchStreamer(ctx context.Context, query string) ([]*PlatformStreamer, error)
    GetChannelInfo(ctx context.Context, handle string) (*PlatformChannelInfo, error)
}
```

## Existing Implementations

### YouTube Adapter

**File**: `internal/adapter/youtube.go`

**Authentication**: API Key (YouTube Data API v3)

**Key Features**:
- Searches for live streams using the `/search` endpoint with `eventType=live`
- Retrieves viewer count from `/videos` endpoint with `liveStreamingDetails`
- Handles channel search and info retrieval
- Uses channel ID as the handle format

**Handle Format**: YouTube Channel ID (e.g., `UCXuqSBlHAE6Xw-yeJA0Tunw`)

**API Endpoints**:
- `GET /youtube/v3/search` - Search for channels and live streams
- `GET /youtube/v3/videos` - Get video details including viewer count
- `GET /youtube/v3/channels` - Get channel information

**Rate Limits**: 10,000 quota units per day (each request costs 1-100 units)

**Error Handling**:
- Returns offline status if no live streams found
- Logs API errors with context
- Returns structured errors for HTTP failures

**Configuration**:
```go
adapter := NewYouTubeAdapter(apiKey)
```

---

### Twitch Adapter

**File**: `internal/adapter/twitch.go`

**Authentication**: OAuth 2.0 Client Credentials (Twitch Helix API)

**Key Features**:
- Automatically obtains and manages OAuth access tokens
- Converts usernames to user IDs for API calls
- Queries `/streams` endpoint for live status
- Supports channel search and user info retrieval

**Handle Format**: Twitch username (e.g., `shroud`)

**API Endpoints**:
- `POST /oauth2/token` - Obtain access token
- `GET /helix/users` - Get user ID from username
- `GET /helix/streams` - Get live stream information
- `GET /helix/search/channels` - Search for channels

**Rate Limits**: 800 requests per minute per client ID

**Error Handling**:
- Automatically refreshes access token if expired
- Returns offline status if stream data is empty
- Handles user not found errors gracefully

**Configuration**:
```go
adapter := NewTwitchAdapter(clientID, clientSecret)
```

**Token Management**:
The adapter automatically requests an OAuth token on first use and caches it for subsequent requests. Token refresh is not currently implemented (tokens expire after ~60 days).

---

### Kick Adapter

**File**: `internal/adapter/kick.go`

**Authentication**: OAuth 2.0 Client Credentials

**Key Features**:
- Uses Kick's public v1 API (`api.kick.com`)
- OAuth2 client credentials flow for authentication
- Automatic token caching and refresh (60 seconds before expiry)
- Thread-safe token management with mutex
- Checks `livestream` field for live status

**Handle Format**: Kick slug/username (e.g., `xqc`)

**API Endpoints**:
- `POST https://id.kick.com/oauth/token` - Obtain access token
- `GET /api/v2/channels/:slug` - Get channel info and live status
- `GET /api/search` - Search for channels
- `GET /public/v1/channels` - Public API for connection check

**Rate Limits**: Undocumented (use conservative request patterns)

**Error Handling**:
- Returns offline status if `livestream` field is null
- Handles 404 for non-existent channels
- Returns structured errors for API and authentication failures

**Configuration**:
```go
adapter := NewKickAdapter(clientID, clientSecret)
```

**Token Management**:
The adapter automatically requests an OAuth token using client credentials on first API call. Tokens are cached in memory and refreshed 60 seconds before expiry. The implementation is thread-safe using read-write mutex.

---

## Adding a New Platform

To add support for a new streaming platform, follow these steps:

### 1. Create Adapter File

Create a new file in `internal/adapter/` (e.g., `internal/adapter/newplatform.go`):

```go
package adapter

import (
    "context"
    "net/http"
    "time"
    
    "who-live-when/internal/domain"
)

type NewPlatformAdapter struct {
    apiKey     string
    httpClient *http.Client
}

func NewNewPlatformAdapter(apiKey string) *NewPlatformAdapter {
    return &NewPlatformAdapter{
        apiKey: apiKey,
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}
```

### 2. Implement Interface Methods

Implement all three required methods:

```go
func (n *NewPlatformAdapter) GetLiveStatus(ctx context.Context, handle string) (*domain.PlatformLiveStatus, error) {
    // Query platform API for live status
    // Return PlatformLiveStatus with:
    // - IsLive: true/false
    // - StreamURL: direct link to stream (if live)
    // - Title: stream title
    // - Thumbnail: preview image URL
    // - ViewerCount: current viewer count
}

func (n *NewPlatformAdapter) SearchStreamer(ctx context.Context, query string) ([]*domain.PlatformStreamer, error) {
    // Search platform for channels matching query
    // Return slice of PlatformStreamer with:
    // - Handle: platform-specific identifier
    // - Name: display name
    // - Platform: "newplatform"
    // - Thumbnail: profile image URL
}

func (n *NewPlatformAdapter) GetChannelInfo(ctx context.Context, handle string) (*domain.PlatformChannelInfo, error) {
    // Get detailed channel information
    // Return PlatformChannelInfo with:
    // - Handle: platform-specific identifier
    // - Name: display name
    // - Description: channel bio/description
    // - Thumbnail: profile image URL
    // - Platform: "newplatform"
}
```

### 3. Add Tests

Create `internal/adapter/newplatform_test.go`:

```go
package adapter

import (
    "context"
    "testing"
)

func TestNewPlatformAdapter_GetLiveStatus(t *testing.T) {
    adapter := NewNewPlatformAdapter("test-api-key")
    
    tests := []struct {
        name    string
        handle  string
        wantErr bool
    }{
        {
            name:    "valid channel",
            handle:  "test-channel",
            wantErr: false,
        },
        {
            name:    "invalid channel",
            handle:  "nonexistent",
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            status, err := adapter.GetLiveStatus(context.Background(), tt.handle)
            if (err != nil) != tt.wantErr {
                t.Errorf("GetLiveStatus() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !tt.wantErr && status == nil {
                t.Error("GetLiveStatus() returned nil status")
            }
        })
    }
}
```

### 4. Register in Service Layer

Update `internal/service/livestatus.go` to include the new adapter:

```go
func NewLiveStatusService(
    streamerRepo repository.StreamerRepository,
    youtubeAdapter *adapter.YouTubeAdapter,
    twitchAdapter *adapter.TwitchAdapter,
    kickAdapter *adapter.KickAdapter,
    newPlatformAdapter *adapter.NewPlatformAdapter, // Add here
) domain.LiveStatusService {
    return &liveStatusService{
        streamerRepo: streamerRepo,
        adapters: map[string]domain.PlatformAdapter{
            "youtube":     youtubeAdapter,
            "twitch":      twitchAdapter,
            "kick":        kickAdapter,
            "newplatform": newPlatformAdapter, // Add here
        },
    }
}
```

### 5. Update Main Server

Update `cmd/server/main.go` to initialize the new adapter:

```go
// Initialize platform adapters
youtubeAdapter := adapter.NewYouTubeAdapter(os.Getenv("YOUTUBE_API_KEY"))
twitchAdapter := adapter.NewTwitchAdapter(
    os.Getenv("TWITCH_CLIENT_ID"),
    os.Getenv("TWITCH_CLIENT_SECRET"),
)
kickAdapter := adapter.NewKickAdapter(
    os.Getenv("KICK_CLIENT_ID"),
    os.Getenv("KICK_CLIENT_SECRET"),
)
newPlatformAdapter := adapter.NewNewPlatformAdapter(os.Getenv("NEWPLATFORM_API_KEY"))

// Pass to service
liveStatusService := service.NewLiveStatusService(
    streamerRepo,
    youtubeAdapter,
    twitchAdapter,
    kickAdapter,
    newPlatformAdapter,
)
```

### 6. Update Domain Models

If needed, update `internal/domain/models.go` to include the new platform in validation:

```go
var SupportedPlatforms = []string{"youtube", "twitch", "kick", "newplatform"}
```

---

## Best Practices

### Error Handling

- Always return structured errors with context
- Log API failures with relevant details (handle, status code)
- Return offline status instead of errors when stream is not live
- Handle rate limiting gracefully

### HTTP Client Configuration

- Set reasonable timeouts (10 seconds recommended)
- Reuse HTTP clients (don't create new ones per request)
- Use context for cancellation support
- Set appropriate headers (User-Agent, Accept, etc.)

### Authentication

- Cache access tokens when possible
- Implement token refresh logic for OAuth
- Store credentials securely (environment variables)
- Handle authentication failures gracefully

### Response Parsing

- Use strongly-typed structs for JSON decoding
- Handle missing or null fields gracefully
- Validate data before returning
- Log parsing errors with context

### Testing

- Test with mock HTTP responses
- Cover both success and error cases
- Test edge cases (empty results, malformed data)
- Use table-driven tests for multiple scenarios

---

## Common Patterns

### Checking Live Status

Most platforms indicate live status in one of these ways:

1. **Dedicated live endpoint**: Query returns data only if live (YouTube, Twitch)
2. **Status field**: Response includes `is_live` or similar boolean (Kick)
3. **Stream object**: Presence of stream object indicates live status (Kick)

### Handle Formats

Different platforms use different identifier formats:

- **YouTube**: Channel ID (24-character alphanumeric)
- **Twitch**: Username (lowercase alphanumeric with underscores)
- **Kick**: Slug (lowercase alphanumeric with hyphens)

Store the platform-specific format in the `Streamer.Handles` map.

### Rate Limiting

Implement rate limiting strategies:

1. **Client-side throttling**: Limit requests per second
2. **Caching**: Store results with TTL to reduce API calls
3. **Batch requests**: Use batch endpoints when available
4. **Exponential backoff**: Retry with increasing delays on rate limit errors

---

## Troubleshooting

### Common Issues

**Issue**: "401 Unauthorized" errors
- **Solution**: Check API credentials are set correctly
- **Solution**: Verify OAuth token is valid and not expired

**Issue**: "404 Not Found" for valid channels
- **Solution**: Verify handle format matches platform requirements
- **Solution**: Check if channel exists on the platform

**Issue**: Slow response times
- **Solution**: Increase HTTP client timeout
- **Solution**: Implement caching layer
- **Solution**: Use parallel requests for multiple platforms

**Issue**: Inconsistent live status
- **Solution**: Implement caching with appropriate TTL
- **Solution**: Add retry logic for transient failures
- **Solution**: Fall back to cached data on API errors

---

## Future Improvements

Potential enhancements for platform adapters:

1. **Webhook support**: Subscribe to live status changes instead of polling
2. **Batch operations**: Query multiple channels in single request
3. **Advanced caching**: Implement Redis or similar for distributed caching
4. **Circuit breaker**: Prevent cascading failures when platform APIs are down
5. **Metrics**: Track API call success rates, latency, and errors
6. **Rate limit handling**: Automatic backoff and retry with rate limit headers
