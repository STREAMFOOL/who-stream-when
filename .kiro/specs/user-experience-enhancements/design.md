# Design Document: User Experience Enhancements

## Overview

This design enhances the "Who Live When" application by removing authentication barriers, enabling universal access to search and follow functionality, and introducing custom programme creation for all users. The key architectural changes include implementing a dual-storage strategy (database for registered users, session cookies for guests), externalizing configuration through environment variables, and adding a programme management interface.

The design maintains backward compatibility with the existing MVP while extending functionality to support guest users. Session-based storage uses secure HTTP-only cookies to persist guest data during their browsing session, with automatic migration to database storage upon registration. The configuration system uses environment variables with sensible defaults and clear error messages for missing required values.

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Frontend Layer                           │
│         (HTML/JS/CSS + HTMX Templates + Forms)              │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────┐
│                  HTTP Handler Layer                          │
│    (Public + Authenticated handlers, Session middleware)     │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────┐
│                 Service Layer                                │
│  (Business logic with dual storage: DB + Session support)    │
└────────────────────┬────────────────────────────────────────┘
                     │
        ┌────────────┼────────────┬──────────────┐
        │            │            │              │
┌───────▼──┐  ┌──────▼──┐  ┌─────▼──────┐  ┌───▼────────┐
│ Platform │  │ Storage │  │ Auth       │  │ Session    │
│ Adapters │  │ Layer   │  │ Service    │  │ Storage    │
└──────────┘  └─────────┘  └────────────┘  └────────────┘
```

### Key Architectural Changes

1. **Dual Storage Strategy**: Services support both database persistence (registered users) and session-based storage (guest users)
2. **Session Management Enhancement**: Extended session manager to handle complex data structures (follows, custom programmes)
3. **Configuration Externalization**: All environment-specific values moved to environment variables
4. **Universal Access**: Removed authentication requirements from search and follow operations
5. **Programme Management**: New service and handlers for custom programme CRUD operations

## Components and Interfaces

### 1. Enhanced Session Manager

Manages both authentication sessions and guest user data storage.

```go
type SessionManager interface {
    // Authentication sessions
    GetSession(r *http.Request) (string, error)
    SetSession(w http.ResponseWriter, userID string)
    ClearSession(w http.ResponseWriter)
    
    // Guest data storage
    GetGuestFollows(r *http.Request) ([]string, error)
    SetGuestFollows(w http.ResponseWriter, streamerIDs []string)
    GetGuestProgramme(r *http.Request) (*CustomProgramme, error)
    SetGuestProgramme(w http.ResponseWriter, programme *CustomProgramme)
    ClearGuestData(w http.ResponseWriter)
}
```

### 2. Programme Service

Manages custom programme creation and retrieval for all users.

```go
type ProgrammeService interface {
    // Custom programme operations
    CreateCustomProgramme(ctx context.Context, userID string, streamerIDs []string) (*CustomProgramme, error)
    GetCustomProgramme(ctx context.Context, userID string) (*CustomProgramme, error)
    UpdateCustomProgramme(ctx context.Context, userID string, streamerIDs []string) error
    DeleteCustomProgramme(ctx context.Context, userID string) error
    
    // Guest programme operations (session-based)
    CreateGuestProgramme(streamerIDs []string) (*CustomProgramme, error)
    
    // Programme rendering
    GenerateCalendarFromProgramme(ctx context.Context, programme *CustomProgramme, week time.Time) (*CalendarView, error)
    GenerateGlobalProgramme(ctx context.Context, week time.Time) (*CalendarView, error)
}
```

### 3. Enhanced User Service

Supports both registered and guest user operations.

```go
type UserService interface {
    // Existing methods
    GetUser(ctx context.Context, userID string) (*User, error)
    CreateUser(ctx context.Context, googleID string, email string) (*User, error)
    
    // Enhanced follow operations (support both DB and session)
    FollowStreamer(ctx context.Context, userID, streamerID string) error
    UnfollowStreamer(ctx context.Context, userID, streamerID string) error
    GetUserFollows(ctx context.Context, userID string) ([]*Streamer, error)
    
    // Guest follow operations
    GetGuestFollows(streamerIDs []string) ([]*Streamer, error)
    
    // Session migration
    MigrateGuestData(ctx context.Context, userID string, guestFollows []string, guestProgramme *CustomProgramme) error
}
```

### 4. Enhanced Streamer Service

Adds streamer creation from search results.

```go
type StreamerService interface {
    // Existing methods
    GetStreamer(ctx context.Context, id string) (*Streamer, error)
    ListStreamers(ctx context.Context, limit int) ([]*Streamer, error)
    SearchStreamers(ctx context.Context, query string) ([]*Streamer, error)
    
    // New methods
    CreateStreamerFromSearchResult(ctx context.Context, result *SearchResult) (*Streamer, error)
    GetOrCreateStreamer(ctx context.Context, platform, handle string) (*Streamer, error)
}
```

### 5. Configuration Service

Manages environment-based configuration with validation.

```go
type Config struct {
    // Database configuration
    DatabasePath string
    
    // OAuth configuration
    GoogleClientID     string
    GoogleClientSecret string
    GoogleRedirectURL  string
    
    // Platform API keys
    YouTubeAPIKey    string
    TwitchClientID   string
    TwitchSecret     string
    
    // Server configuration
    ServerPort       string
    SessionSecret    string
    SessionDuration  int
}

type ConfigService interface {
    Load() (*Config, error)
    Validate() error
    LogConfiguration() // Logs config without secrets
}
```

## Data Models

### CustomProgramme

```go
type CustomProgramme struct {
    ID          string
    UserID      string    // Empty for guest programmes
    StreamerIDs []string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### CalendarView

```go
type CalendarView struct {
    Week            time.Time
    Streamers       []*Streamer
    Entries         []CalendarEntry
    IsCustom        bool  // true if from custom programme
    IsGuestSession  bool  // true if guest user
}

type CalendarEntry struct {
    StreamerID  string
    DayOfWeek   int
    Hour        int
    Probability float64
}
```

### GuestSession

```go
type GuestSession struct {
    FollowedStreamerIDs []string
    CustomProgramme     *CustomProgramme
    CreatedAt           time.Time
}
```

## Correctness Properties

A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.

### Property 1: Multi-Platform Search Coverage
*For any* search query, the search service should query all three platforms (YouTube, Kick, Twitch) and aggregate results.
**Validates: Requirements 1.2**

### Property 2: Search Result Completeness
*For any* search result returned, it should contain streamer name, handle, platform information, and live status fields.
**Validates: Requirements 1.3**

### Property 3: Registered User Follow Persistence
*For any* registered user and streamer, following the streamer then retrieving follows should include that streamer in the results.
**Validates: Requirements 2.1**

### Property 4: Guest User Follow Session Storage
*For any* guest user and streamer, following the streamer should store the relationship in session data that persists across requests within the same session.
**Validates: Requirements 2.2, 7.1**

### Property 5: Follow List Completeness
*For any* user (registered or guest) with a set of follows, retrieving their follows should return all followed streamers without omissions.
**Validates: Requirements 2.3**

### Property 6: Custom Programme Database Persistence
*For any* registered user and set of streamer IDs, creating a custom programme then retrieving it should return the same streamer IDs.
**Validates: Requirements 3.1**

### Property 7: Custom Programme Session Persistence
*For any* guest user and set of streamer IDs, creating a custom programme should store it in session data that persists across requests within the same session.
**Validates: Requirements 3.2, 7.2**

### Property 8: Custom Programme Calendar Filtering
*For any* custom programme with specific streamer IDs, the generated calendar should only contain entries for streamers in that programme.
**Validates: Requirements 3.3, 9.2**

### Property 9: Streamer Removal from Programme
*For any* custom programme, removing a streamer ID should result in a calendar that does not contain entries for that streamer.
**Validates: Requirements 3.4, 9.3**

### Property 10: Global Programme Ranking
*For any* set of streamers in the system, the global programme should order them by follower count in descending order.
**Validates: Requirements 4.2**

### Property 11: Session Data Persistence Across Requests
*For any* guest user session data (follows and programme), making multiple requests within the same session should maintain that data without loss.
**Validates: Requirements 7.3**

### Property 12: Guest Data Migration on Registration
*For any* guest user with session data (follows and custom programme), registering or logging in should migrate all session data to database storage under their user account.
**Validates: Requirements 7.5**

### Property 13: Streamer Creation from Search Result
*For any* search result with platform and handle information, creating a streamer should produce a database record with matching platform and handle data.
**Validates: Requirements 8.2**

### Property 14: Streamer Addition Idempotence
*For any* streamer with specific platform and handle, adding them multiple times should result in a single streamer record without duplicates.
**Validates: Requirements 8.4**

## Error Handling

### Session Management Errors

- Handle cookie size limits by compressing session data or using server-side session storage
- Validate session data integrity before use
- Clear corrupted session data and start fresh
- Log session errors for monitoring

### Configuration Errors

- Fail fast on missing required environment variables with clear error messages
- Validate environment variable formats at startup
- Log all configuration errors with context
- Provide example values in error messages

### Guest User Limitations

- Display clear messaging about session-based storage limitations
- Prompt guest users to register when approaching session limits
- Handle session expiry gracefully with user-friendly messages

### Data Migration Errors

- Use transactions for guest-to-registered data migration
- Roll back on migration failure and preserve session data
- Log migration errors with user context
- Retry failed migrations with exponential backoff

### Storage Errors

- Handle database write failures for registered users
- Handle cookie write failures for guest users
- Provide fallback to read-only mode if storage fails
- Log all storage errors with operation context

## Testing Strategy

### Unit Testing

Unit tests verify specific examples and edge cases:
- Configuration loading with various environment variable combinations
- Session data serialization and deserialization
- Custom programme CRUD operations
- Guest data migration logic
- Streamer creation from search results
- Error handling for missing configuration
- Session expiry and cleanup

### Property-Based Testing

Property-based tests verify universal properties across all inputs using `gopter`:
- Search results always contain required fields (name, handle, platform, live status)
- Follow operations are idempotent (following twice = following once)
- Custom programme filtering correctly excludes non-selected streamers
- Session data round-trips correctly (store then retrieve = original data)
- Global programme ordering is always by follower count descending
- Guest data migration preserves all follows and programme data
- Streamer creation is idempotent (creating same streamer twice = one record)

Each property-based test should run a minimum of 100 iterations to ensure coverage across the input space.

### Integration Testing

Integration tests verify component interactions:
- End-to-end guest user flow: search → follow → create programme → view calendar
- Guest-to-registered migration: session data → login → database persistence
- Configuration loading and validation at application startup
- Custom programme creation and calendar generation
- Multi-platform search with real adapter responses (mocked)

### Test Coverage Goals

- Minimum 80% code coverage for service layer
- Minimum 70% code coverage for handler layer
- Minimum 80% code coverage for repository layer
- 100% coverage for configuration validation logic
- 100% coverage for session management logic
- All correctness properties have corresponding property-based tests

### Test Organization

- Property-based tests tagged with: `**Feature: user-experience-enhancements, Property {number}: {property_text}**`
- Each correctness property implemented by a single property-based test
- Unit tests co-located with source files using `_test.go` suffix
- Integration tests in separate `_integration_test.go` files
