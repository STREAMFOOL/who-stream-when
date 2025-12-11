# Design Document: Who Live When - Streamer Tracking MVP

## Overview

The "Who Stream When" application is a Go-based web service that tracks streamer availability across multiple platforms (YouTube, Kick, Twitch) and provides activity insights to users. The system is designed with a layered architecture separating concerns into distinct components: HTTP handlers, business logic services, data persistence (SQLite), and external platform integrations.

The frontend uses plain HTML/JS/CSS with HTMX for reactive updates, enabling real-time status changes without full page reloads. The home page displays a default week view showing the most viewed streamers, providing immediate value to both registered and unregistered users. The architecture prioritizes flexibility and extensibility, allowing new platforms, features, and UI improvements without requiring core rewrites.

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Frontend Layer                           │
│              (HTML/JS/CSS + HTMX Templates)                 │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────┐
│                  HTTP Handler Layer                          │
│         (Go http.Handler implementations)                    │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────┐
│                 Service Layer                                │
│    (Business logic, orchestration, data transformation)      │
└────────────────────┬────────────────────────────────────────┘
                     │
        ┌────────────┼────────────┐
        │            │            │
┌───────▼──┐  ┌──────▼──┐  ┌─────▼──────┐
│ Platform │  │ Storage │  │ Auth       │
│ Adapters │  │ Layer   │  │ Service    │
└──────────┘  └─────────┘  └────────────┘
```

### Component Responsibilities

**HTTP Handler Layer**: Routes requests, validates input, calls services, renders templates
**Service Layer**: Implements business logic, coordinates between components, transforms data
**Platform Adapters**: Encapsulate API calls to YouTube, Kick, Twitch; handle platform-specific logic
**Storage Layer**: Manages data persistence (database operations)
**Auth Service**: Handles Google SSO, session management, user context

## Components and Interfaces

### 1. Streamer Service

Manages streamer data and operations.

```go
type StreamerService interface {
    GetStreamer(ctx context.Context, id string) (*Streamer, error)
    ListStreamers(ctx context.Context, limit int) ([]*Streamer, error)
    SearchStreamers(ctx context.Context, query string) ([]*Streamer, error)
    AddStreamer(ctx context.Context, streamer *Streamer) error
    UpdateStreamer(ctx context.Context, streamer *Streamer) error
    GetStreamersByPlatform(ctx context.Context, platform string) ([]*Streamer, error)
}
```

### 2. Live Status Service

Queries and caches live status across platforms.

```go
type LiveStatusService interface {
    GetLiveStatus(ctx context.Context, streamerID string) (*LiveStatus, error)
    RefreshLiveStatus(ctx context.Context, streamerID string) (*LiveStatus, error)
    GetAllLiveStatus(ctx context.Context) (map[string]*LiveStatus, error)
}
```

### 3. Activity Heatmap Service

Generates activity patterns from historical data.

```go
type HeatmapService interface {
    GenerateHeatmap(ctx context.Context, streamerID string) (*Heatmap, error)
    RecordActivity(ctx context.Context, streamerID string, timestamp time.Time) error
    GetActivityStats(ctx context.Context, streamerID string) (*ActivityStats, error)
}
```

### 4. Platform Adapter Interface

Abstracts platform-specific API interactions.

```go
type PlatformAdapter interface {
    GetLiveStatus(ctx context.Context, handle string) (*PlatformLiveStatus, error)
    SearchStreamer(ctx context.Context, query string) ([]*PlatformStreamer, error)
    GetChannelInfo(ctx context.Context, handle string) (*PlatformChannelInfo, error)
}
```

### 5. User Service

Manages user accounts and authentication state.

```go
type UserService interface {
    GetUser(ctx context.Context, userID string) (*User, error)
    CreateUser(ctx context.Context, googleID string, email string) (*User, error)
    GetUserFollows(ctx context.Context, userID string) ([]*Streamer, error)
    FollowStreamer(ctx context.Context, userID, streamerID string) error
    UnfollowStreamer(ctx context.Context, userID, streamerID string) error
}
```

### 6. TV Programme Service

Generates weekly predictions based on activity patterns.

```go
type TVProgrammeService interface {
    GenerateProgramme(ctx context.Context, userID string, week time.Time) (*TVProgramme, error)
    GetPredictedLiveTime(ctx context.Context, streamerID string, dayOfWeek int) (*PredictedTime, error)
    GetMostViewedStreamers(ctx context.Context, limit int) ([]*Streamer, error)
    GetDefaultWeekView(ctx context.Context) (*WeekView, error)
}
```

## Data Models

### Streamer

```go
type Streamer struct {
    ID        string                 // Unique identifier
    Name      string                 // Display name
    Handles   map[string]string      // Platform -> handle mapping
    Platforms []string               // List of supported platforms
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### LiveStatus

```go
type LiveStatus struct {
    StreamerID  string
    IsLive      bool
    Platform    string
    StreamURL   string
    Title       string
    Thumbnail   string
    ViewerCount int
    UpdatedAt   time.Time
}
```

### Heatmap

```go
type Heatmap struct {
    StreamerID string
    Hours      [24]float64  // Probability 0-1 for each hour
    DaysOfWeek [7]float64   // Probability 0-1 for each day
    DataPoints int          // Number of historical records
    GeneratedAt time.Time
}
```

### User

```go
type User struct {
    ID        string
    GoogleID  string
    Email     string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### ActivityRecord

```go
type ActivityRecord struct {
    ID        string
    StreamerID string
    StartTime time.Time
    EndTime   time.Time
    Platform  string
    CreatedAt time.Time
}
```

### TVProgramme

```go
type TVProgramme struct {
    UserID      string
    Week        time.Time
    Entries     []ProgrammeEntry
    GeneratedAt time.Time
}

type ProgrammeEntry struct {
    StreamerID  string
    DayOfWeek   int
    Hour        int
    Probability float64
}

type WeekView struct {
    Week            time.Time
    Streamers       []*Streamer
    Entries         []ProgrammeEntry
    ViewCount       map[string]int  // Streamer ID -> view count
}
```

## Correctness Properties

A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.

### Property 1: Streamer Data Persistence
*For any* streamer with valid name, handles, and platforms, adding it to the system and retrieving it should return identical data.
**Validates: Requirements 1.1, 1.3**

### Property 2: Streamer Update Consistency
*For any* existing streamer, updating its platform information and retrieving it should reflect the updated platforms.
**Validates: Requirements 1.2**

### Property 3: Multi-Platform Handle Isolation
*For any* streamer with handles on multiple platforms, each platform's handle should be stored and retrieved independently without cross-contamination.
**Validates: Requirements 1.4, 10.1**

### Property 4: Live Status Completeness
*For any* streamer in the system, retrieving the streamer list should include live status data for all streamers.
**Validates: Requirements 2.1, 2.2, 2.3**

### Property 5: Heatmap Probability Validity
*For any* streamer with historical activity data, the generated heatmap should have probability values between 0 and 1 for all hours and days.
**Validates: Requirements 3.1, 3.2**

### Property 6: Weighted Activity Calculation
*For any* streamer with activity records spanning more than 3 months, the heatmap should weight the last 3 months at 80% and older data at 20% when calculating probabilities.
**Validates: Requirements 3.3**

### Property 7: TV Programme Prediction Consistency
*For any* registered user viewing their TV programme for a given week, the predictions should be based on the same weighted historical data as the heatmap.
**Validates: Requirements 4.1, 4.2**

### Property 8: Week-Based Programme Uniqueness
*For any* registered user, TV programmes for different weeks should contain different predicted times based on the day-of-week patterns.
**Validates: Requirements 4.4**

### Property 9: Read-Only User Visibility
*For any* unregistered user, the streamer list should only include streamers that have been followed by at least one registered user.
**Validates: Requirements 5.1**

### Property 10: Search Access Control
*For any* unregistered user attempting to search, the system should reject the request and require authentication.
**Validates: Requirements 5.2, 5.4**

### Property 11: Unregistered User Feature Restriction
*For any* unregistered user viewing a streamer, the response should not include follow functionality or the ability to add new streamers.
**Validates: Requirements 5.3**

### Property 12: User Session Establishment
*For any* successful Google OAuth authentication, the system should create a new user account (if not exists) or retrieve the existing one and establish a valid session.
**Validates: Requirements 6.2**

### Property 13: Session Cleanup on Logout
*For any* authenticated user who logs out, subsequent requests should be treated as unauthenticated.
**Validates: Requirements 6.3**

### Property 14: Multi-Platform Search Coverage
*For any* registered user search query, the system should query all three platforms (YouTube, Kick, Twitch) and return results from all available matches.
**Validates: Requirements 7.1, 7.4**

### Property 15: Search Result Completeness
*For any* search result, the returned data should include streamer name, handle, and all available platforms.
**Validates: Requirements 7.2**

### Property 16: Follow Operation Idempotence
*For any* registered user following a streamer, following the same streamer multiple times should result in a single follow relationship.
**Validates: Requirements 8.1**

### Property 17: Unfollow Removes Relationship
*For any* registered user who has followed a streamer, unfollowing should remove the streamer from their followed list.
**Validates: Requirements 8.2**

### Property 18: Followed Streamer Visibility
*For any* registered user, their followed streamers should be visible to other users and appear in search results.
**Validates: Requirements 8.1**

### Property 19: Activity Tracking on Follow
*For any* streamer that is followed by a user, the system should begin recording activity data for heatmap generation.
**Validates: Requirements 8.4**

### Property 20: Calendar Display Accuracy
*For any* registered user viewing the calendar, all followed streamers with predictions should appear in their corresponding time slots.
**Validates: Requirements 9.1, 9.2**

### Property 21: Calendar Navigation Consistency
*For any* registered user navigating between weeks, the calendar should display the correct week's data and allow navigation to previous and next weeks.
**Validates: Requirements 9.4**

### Property 22: Platform Query Coverage
*For any* multi-platform streamer, querying live status should check all associated platforms.
**Validates: Requirements 10.2, 10.3**

## Error Handling

### Platform API Failures

- Implement exponential backoff for failed API calls
- Cache last known status for up to 1 hour
- Display cached data with a "last updated" timestamp
- Log all API failures for monitoring

### Data Validation

- Validate streamer handles are non-empty strings
- Validate platform names against supported list (YouTube, Kick, Twitch)
- Validate timestamps are within reasonable bounds
- Reject duplicate streamer entries

### Authentication Errors

- Handle Google OAuth token expiration gracefully
- Redirect to login on session timeout
- Display user-friendly error messages for auth failures

### Database Errors (SQLite)

- Implement connection pooling with retry logic
- Log database errors with context
- Return appropriate HTTP status codes (500 for server errors, 409 for conflicts)
- Handle SQLite locking with appropriate timeouts

### Storage Technology

- Use SQLite for data persistence (file-based, no external dependencies)
- Implement database migrations for schema management
- Use prepared statements to prevent SQL injection
- Implement transaction support for multi-step operations

## Testing Strategy

### Unit Testing

Unit tests verify specific examples and edge cases:
- Streamer data validation and storage operations
- Heatmap calculations with various data distributions
- Activity pattern weighting (80% recent, 20% older)
- User follow/unfollow operations
- Platform adapter error handling

### Property-Based Testing

Property-based tests verify universal properties across all inputs using a PBT library (Go: `rapid` or `gopter`):
- Heatmap probability values always sum to valid ranges
- Activity records maintain temporal consistency
- User follows are idempotent (following twice = following once)
- Streamer data round-trips through storage correctly
- TV programme predictions remain consistent for the same input data

### Integration Testing

Integration tests verify component interactions:
- End-to-end user registration and login flow
- Streamer search across multiple platforms
- Live status updates propagate correctly
- Calendar view displays followed streamers accurately

### Test Coverage Goals

- Minimum 80% code coverage for service layer
- 100% coverage for data validation logic
- All correctness properties have corresponding property-based tests
- Critical paths (authentication, data persistence) have integration tests
