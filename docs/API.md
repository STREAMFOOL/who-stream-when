# API Documentation

This document describes the HTTP endpoints available in the Who Live When application.

## Authentication

The application uses Google OAuth 2.0 for authentication. Authenticated routes require a valid session cookie.

### Session Management

- **Cookie Name**: `session_id`
- **Cookie Attributes**: HttpOnly, Secure (in production), SameSite=Lax
- **Session Duration**: Persistent until logout

## Public Routes

These routes are accessible without authentication.

### GET /

**Description**: Home page displaying the default week view with most viewed streamers.

**Response**: HTML page with:
- Weekly calendar grid showing predicted streaming times
- Top 10 most followed streamers
- Live status indicators
- Navigation to login

**Example**:
```
GET / HTTP/1.1
Host: localhost:8080
```

---

### GET /streamer/:id

**Description**: Streamer detail page showing live status, activity heatmap, and platform information.

**Parameters**:
- `id` (path): Streamer UUID

**Response**: HTML page with:
- Streamer name and platforms
- Current live status with stream link (if live)
- Activity heatmap (24-hour x 7-day grid)
- Historical activity statistics

**Example**:
```
GET /streamer/123e4567-e89b-12d3-a456-426614174000 HTTP/1.1
Host: localhost:8080
```

---

### GET /login

**Description**: Initiates Google OAuth authentication flow.

**Response**: Redirect to Google OAuth consent screen

**Example**:
```
GET /login HTTP/1.1
Host: localhost:8080
```

---

### GET /auth/callback

**Description**: OAuth callback handler that processes Google authentication response.

**Parameters**:
- `code` (query): OAuth authorization code
- `state` (query): CSRF protection token

**Response**: 
- Success: Redirect to `/dashboard` with session cookie
- Failure: Redirect to `/login` with error message

**Example**:
```
GET /auth/callback?code=4/0AY0e-g7...&state=random-state HTTP/1.1
Host: localhost:8080
```

---

### GET /logout

**Description**: Ends user session and clears authentication cookies.

**Response**: Redirect to home page (`/`)

**Example**:
```
GET /logout HTTP/1.1
Host: localhost:8080
Cookie: session_id=abc123...
```

---

## Authenticated Routes

These routes require a valid session cookie. Unauthenticated requests will be redirected to `/login`.

### GET /dashboard

**Description**: User dashboard showing followed streamers and personalized TV programme.

**Authentication**: Required

**Response**: HTML page with:
- List of followed streamers with live status
- Weekly calendar of predicted streaming times
- Quick actions (search, follow/unfollow)

**Example**:
```
GET /dashboard HTTP/1.1
Host: localhost:8080
Cookie: session_id=abc123...
```

---

### POST /search

**Description**: Search for streamers across all platforms (YouTube, Twitch, Kick).

**Authentication**: Required

**Request Body** (form-encoded):
- `query` (string): Search term (streamer name or handle)

**Response**: HTML fragment with search results (HTMX-compatible)
- List of matching streamers
- Platform indicators
- Follow buttons

**Example**:
```
POST /search HTTP/1.1
Host: localhost:8080
Cookie: session_id=abc123...
Content-Type: application/x-www-form-urlencoded

query=shroud
```

---

### POST /follow/:id

**Description**: Follow a streamer and add them to the user's tracked list.

**Authentication**: Required

**Parameters**:
- `id` (path): Streamer UUID

**Response**: 
- Success: HTML fragment with updated follow status (HTMX-compatible)
- Error: 404 if streamer not found, 500 on server error

**Side Effects**:
- Adds streamer to user's followed list
- Begins activity tracking for heatmap generation
- Makes streamer discoverable to other users

**Example**:
```
POST /follow/123e4567-e89b-12d3-a456-426614174000 HTTP/1.1
Host: localhost:8080
Cookie: session_id=abc123...
```

---

### POST /unfollow/:id

**Description**: Unfollow a streamer and remove them from the user's tracked list.

**Authentication**: Required

**Parameters**:
- `id` (path): Streamer UUID

**Response**: 
- Success: HTML fragment with updated follow status (HTMX-compatible)
- Error: 404 if streamer not found, 500 on server error

**Example**:
```
POST /unfollow/123e4567-e89b-12d3-a456-426614174000 HTTP/1.1
Host: localhost:8080
Cookie: session_id=abc123...
```

---

### GET /calendar

**Description**: Weekly TV programme calendar showing predicted streaming times for followed streamers.

**Authentication**: Required

**Query Parameters**:
- `week` (optional): ISO 8601 date string for week start (defaults to current week)

**Response**: HTML page with:
- 24-hour x 7-day calendar grid
- Predicted streaming times with probability indicators
- Week navigation (previous/next)
- Streamer names in time slots

**Example**:
```
GET /calendar?week=2024-01-07 HTTP/1.1
Host: localhost:8080
Cookie: session_id=abc123...
```

---

## Error Responses

All endpoints may return the following error responses:

### 400 Bad Request
Invalid request parameters or malformed data.

### 401 Unauthorized
Missing or invalid authentication for protected routes.

### 404 Not Found
Requested resource (streamer, user) does not exist.

### 500 Internal Server Error
Server-side error during request processing.

---

## HTMX Integration

Several endpoints return HTML fragments designed for HTMX partial page updates:

- `POST /search`: Returns search results fragment
- `POST /follow/:id`: Returns updated follow button
- `POST /unfollow/:id`: Returns updated follow button
- Calendar navigation: Returns updated calendar grid

These endpoints use the `HX-Request` header to detect HTMX requests and return appropriate fragments instead of full pages.

---

## Rate Limiting

Currently, no rate limiting is implemented. Future versions may add:
- Per-user request limits
- Platform API call throttling
- Search query rate limits

---

## Caching

### Live Status Cache
- **TTL**: 1 hour
- **Invalidation**: Manual refresh or cache expiration
- **Fallback**: Returns cached data if platform APIs are unavailable

### Heatmap Cache
- **Storage**: Database (regenerated on demand)
- **Invalidation**: Regenerated when new activity data is recorded

---

## Platform API Integration

The application queries external platform APIs:

### YouTube Data API v3
- **Authentication**: API key
- **Endpoints Used**: 
  - `/search` (live streams, channels)
  - `/videos` (viewer count)
  - `/channels` (channel info)
- **Rate Limits**: 10,000 quota units per day

### Twitch Helix API
- **Authentication**: OAuth 2.0 client credentials
- **Endpoints Used**:
  - `/streams` (live status)
  - `/users` (user info)
  - `/search/channels` (search)
- **Rate Limits**: 800 requests per minute

### Kick API
- **Authentication**: None (public API)
- **Endpoints Used**:
  - `/api/v2/channels/:slug` (channel info, live status)
  - `/api/search` (search)
- **Rate Limits**: Undocumented (use conservative limits)

---

## Data Models

### Streamer
```json
{
  "id": "uuid",
  "name": "string",
  "handles": {
    "youtube": "channel_id",
    "twitch": "username",
    "kick": "slug"
  },
  "platforms": ["youtube", "twitch", "kick"],
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

### LiveStatus
```json
{
  "streamer_id": "uuid",
  "is_live": "boolean",
  "platform": "string",
  "stream_url": "string",
  "title": "string",
  "thumbnail": "string",
  "viewer_count": "integer",
  "updated_at": "timestamp"
}
```

### Heatmap
```json
{
  "streamer_id": "uuid",
  "hours": [24]float64,
  "days_of_week": [7]float64,
  "data_points": "integer",
  "generated_at": "timestamp"
}
```
