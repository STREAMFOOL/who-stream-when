# Design Document: Working Service MVP

## Overview

This design transforms the "Who Live When" application from a broken prototype into a functional service. The key changes are:

1. **Fix Navigation**: Remove any JavaScript/HTMX interference with standard link navigation
2. **Remove Authentication**: Strip Google SSO and make all features accessible to everyone
3. **Real Data Integration**: Seed actual Kick streamers and display live data from Kick API
4. **UI Cleanup**: Fix the nested header issue and improve the overall layout
5. **Search & Add Flow**: Allow users to search Kick and add streamers to track

## Architecture

The existing clean architecture remains intact with modifications:

```
┌─────────────────────────────────────────────────────────────┐
│                      HTTP Layer                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │PublicHandler│  │ProgrammeHdlr│  │StreamerHandler(new) │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                     Service Layer                            │
│  ┌───────────────┐  ┌─────────────┐  ┌──────────────────┐   │
│  │StreamerService│  │SearchService│  │LiveStatusService │   │
│  └───────────────┘  └─────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                    Repository Layer                          │
│  ┌───────────────┐  ┌─────────────────────────────────────┐ │
│  │StreamerRepo   │  │ActivityRepo, LiveStatusRepo, etc.   │ │
│  └───────────────┘  └─────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                     Adapter Layer                            │
│  ┌─────────────────────────────────────────────────────────┐│
│  │                    KickAdapter                           ││
│  │  - GetLiveStatus(handle) → PlatformLiveStatus           ││
│  │  - SearchStreamer(query) → []PlatformStreamer           ││
│  │  - GetChannelInfo(handle) → PlatformChannelInfo         ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

## Components and Interfaces

### 1. Navigation Fix (Templates)

**Problem**: The current templates use HTMX attributes that may interfere with standard navigation.

**Solution**: Ensure all navigation links are standard `<a href="...">` tags without HTMX attributes that prevent default behavior.

**Files to modify**:
- `templates/base.html` - Remove any hx-* attributes from nav links
- `templates/home.html` - Remove nested base template inclusion causing duplicate headers

### 2. Authentication Removal

**Components to modify**:

```go
// Remove from main.go:
// - Google OAuth configuration
// - Session manager initialization (keep for guest programme storage)
// - Auth middleware from routes

// Remove from handlers:
// - IsAuthenticated checks in templates
// - Login/logout routes
// - Auth callback route
```

**Routes after removal**:
- `GET /` - Home page (public)
- `GET /streamer/{id}` - Streamer detail (public)
- `GET /programme` - Programme management (public)
- `POST /programme/*` - Programme operations (public)
- `POST /search` - Search streamers (public)
- `POST /streamer/add` - Add streamer from search (new, public)

### 3. Database Seeding

**New component**: `internal/seed/seed.go`

```go
type Seeder struct {
    streamerRepo repository.StreamerRepository
    kickAdapter  domain.PlatformAdapter
}

func (s *Seeder) SeedPopularStreamers(ctx context.Context) error
```

**Default streamers to seed** (popular Kick streamers):
- xQc
- Adin Ross  
- Kai Cenat
- Amouranth
- Trainwreckstv

### 4. UI/Template Fixes

**Problem identified from screenshot**:
- Nested headers (navbar appears twice)
- "Login to Follow" buttons everywhere
- "Status Unknown" without context
- Inconsistent card styling

**Template structure fix**:

```html
<!-- base.html - single navbar -->
{{define "base"}}
<!DOCTYPE html>
<html>
<head>...</head>
<body>
    <nav class="navbar"><!-- Single nav --></nav>
    <main>{{block "content" .}}{{end}}</main>
    <footer>...</footer>
</body>
</html>
{{end}}

<!-- home.html - no duplicate base -->
{{template "base" .}}
{{define "content"}}
<!-- Content only, no nested navbar -->
{{end}}
```

### 5. Search and Add Streamer Flow

**New handler**: `HandleAddStreamerFromSearch`

```go
// POST /streamer/add
// Request: { "platform": "kick", "handle": "xqc" }
// Response: Redirect to /streamer/{id}

func (h *PublicHandler) HandleAddStreamerFromSearch(w http.ResponseWriter, r *http.Request) {
    // 1. Parse platform and handle from form
    // 2. Call KickAdapter.GetChannelInfo(handle)
    // 3. Create streamer in database via StreamerService
    // 4. Redirect to streamer detail page
}
```

## Data Models

### Existing Models (no changes needed)

```go
type Streamer struct {
    ID        string
    Name      string
    Platforms []string
    Handles   map[string]string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type PlatformLiveStatus struct {
    IsLive      bool
    StreamURL   string
    Title       string
    Thumbnail   string
    ViewerCount int
}

type PlatformChannelInfo struct {
    Handle      string
    Name        string
    Description string
    Thumbnail   string
    Platform    string
}
```

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system-essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Public Access Without Authentication

*For any* HTTP request to any endpoint (/, /programme, /search, /streamer/{id}), the System SHALL return a successful response (2xx status) without requiring authentication cookies or headers.

**Validates: Requirements 2.1, 2.3, 2.4, 2.5**

### Property 2: Streamer Information Display Completeness

*For any* streamer retrieved from the database, when rendered in the UI (home page, search results, or detail page), the output SHALL contain the streamer's name, at least one platform handle, and platform identifier.

**Validates: Requirements 3.2, 5.2**

### Property 3: Live Status Display Completeness

*For any* live status response where `IsLive == true`, the rendered output SHALL contain the stream title (if non-empty), viewer count, and a valid stream URL.

**Validates: Requirements 3.4, 6.2**

### Property 4: Visual Status Indicator Consistency

*For any* streamer card rendered on the home page, the CSS class applied to the status badge SHALL be `status-live` when the streamer is live, and `status-offline` when the streamer is offline.

**Validates: Requirements 4.3**

### Property 5: Platform Link Generation

*For any* streamer with a Kick handle, the rendered detail page SHALL contain a clickable link with href matching the pattern `https://kick.com/{handle}`.

**Validates: Requirements 6.4**

## Error Handling

### Kick API Errors

| Error Condition | User-Facing Message | Technical Action |
|----------------|---------------------|------------------|
| API timeout | "Status Unknown - Unable to reach Kick" | Log error, return cached status if available |
| 404 Not Found | "Streamer not found on Kick" | Remove from database or mark inactive |
| Rate limited | "Status Unknown - Please try again" | Implement exponential backoff |
| Auth failure | "Status Unknown - Service configuration issue" | Alert ops, use cached data |

### Database Errors

| Error Condition | User-Facing Message | Technical Action |
|----------------|---------------------|------------------|
| Connection lost | "Service temporarily unavailable" | Retry with backoff, return 503 |
| Query timeout | "Loading took too long" | Log slow query, return partial data |

## Testing Strategy

### Dual Testing Approach

This implementation uses both unit tests and property-based tests:

- **Unit tests**: Verify specific examples, edge cases, and integration points
- **Property-based tests**: Verify universal properties hold across all valid inputs

### Property-Based Testing Framework

**Library**: `github.com/leanovate/gopter` (already in project dependencies)

**Configuration**: Each property test runs minimum 100 iterations.

### Test Files Structure

```
internal/
├── handler/
│   ├── public_test.go           # Unit tests for handlers
│   └── public_property_test.go  # Property tests for response properties
├── service/
│   ├── streamer_test.go         # Unit tests
│   └── streamer_property_test.go # Property tests for service logic
└── adapter/
    └── kick_test.go             # Unit tests for API integration
```

### Property Test Annotations

Each property-based test MUST include a comment linking to the design document:

```go
// **Feature: working-service-mvp, Property 1: Public Access Without Authentication**
// **Validates: Requirements 2.1, 2.3, 2.4, 2.5**
func TestPublicAccessProperty(t *testing.T) {
    // Property test implementation
}
```

### Unit Test Coverage

| Component | Test Focus |
|-----------|------------|
| PublicHandler | Route accessibility, response codes, template rendering |
| StreamerService | CRUD operations, search integration |
| KickAdapter | API response parsing, error handling |
| Templates | HTML structure, no duplicate elements |
| Seeder | Database population, idempotency |
