# Who Live When

A multi-platform streaming schedule tracker that helps you follow content creators across YouTube, Twitch, and Kick.

## Features

- **Multi-Platform Tracking**: Aggregate live status from YouTube, Twitch, and Kick in one place
- **Activity Heatmaps**: Probability-based predictions of when streamers go live based on historical data
- **TV Programme View**: Weekly calendar showing predicted streaming times for your followed streamers
- **Live Status Monitoring**: Real-time tracking of who is currently streaming
- **Smart Search**: Discover streamers across all platforms with a single search
- **Google OAuth**: Secure authentication to follow streamers and personalize your experience

## Quick Start

### Prerequisites

- Go 1.24.4 or later
- SQLite (embedded via modernc.org/sqlite)

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd who-live-when

# Install dependencies
go mod download

# Build the server
go build -o server ./cmd/server
```

### Configuration

Set the following environment variables:

#### Required Variables

```bash
export GOOGLE_CLIENT_ID="your-google-client-id"
export GOOGLE_CLIENT_SECRET="your-google-client-secret"
```

#### Optional Variables

```bash
# OAuth callback URL (defaults to http://localhost:8080/auth/google/callback)
export GOOGLE_REDIRECT_URL="http://localhost:8080/auth/google/callback"

# Database path (defaults to ./data/who-live-when.db)
export DATABASE_PATH="./data/who-live-when.db"

# Server port (defaults to 8080)
export SERVER_PORT="8080"

# Session configuration (defaults to 604800 seconds = 7 days)
export SESSION_DURATION="604800"
export SESSION_SECRET="your-session-secret"

# Platform API credentials (optional - enables platform-specific features)
export KICK_CLIENT_ID="your-kick-client-id"
export KICK_CLIENT_SECRET="your-kick-client-secret"
export YOUTUBE_API_KEY="your-youtube-api-key"
export TWITCH_CLIENT_ID="your-twitch-client-id"
export TWITCH_CLIENT_SECRET="your-twitch-client-secret"

# Feature flags (comma-separated, defaults to "kick")
# Controls which platforms are enabled: kick, youtube, twitch
export FEATURE_FLAGS="kick,youtube,twitch"
```

#### Configuration Notes

- **Feature Flags**: By default, only Kick is enabled. Set `FEATURE_FLAGS` to enable additional platforms (e.g., `"kick,youtube,twitch"`)
- **Session Duration**: Specified in seconds. Guest user data persists for this duration
- **Platform API Keys**: YouTube and Twitch are optional. If not provided, those platforms will have limited functionality

### Running

```bash
# Run the server
./server
```

The server will start on `http://localhost:8080`

## Guest User Features

The application supports both registered and unregistered (guest) users:

### Guest Users

Guest users can:
- Search for streamers across enabled platforms
- Follow streamers (stored in browser session)
- Create custom programmes (stored in browser session)
- View the global programme and live status

**Limitations**:
- Data persists only during the browser session (cleared when browser closes)
- Session duration is configurable via `SESSION_DURATION` environment variable
- No cross-device synchronization

### Registered Users

Registered users (via Google OAuth) can:
- All guest features plus persistent storage
- Custom programmes saved to database
- Follows persisted across sessions and devices
- Automatic migration of guest data upon registration

### Guest Data Migration

When a guest user registers or logs in:
1. All session-based follows are migrated to the database
2. Custom programme is migrated to the database
3. Session data is cleared
4. User can continue with full account features

## Architecture

This project follows Clean Architecture principles with clear separation of concerns:

```
internal/
├── domain/          # Core business models and interfaces
├── service/         # Business logic implementation
├── repository/      # Data persistence layer (SQLite)
├── handler/         # HTTP request handlers
├── adapter/         # Platform API integrations (YouTube, Twitch, Kick)
└── auth/            # OAuth and session management
```

### Key Design Patterns

- **Repository Pattern**: Abstract data access behind interfaces
- **Adapter Pattern**: Platform-specific APIs implement common interface
- **Dependency Injection**: Services receive dependencies via constructors
- **Clean Architecture**: Domain-driven design with layer separation

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for specific package
go test ./internal/service

# Run tests with verbose output
go test -v ./...

# Run property-based tests (included in test suite)
go test -v ./internal/middleware
go test -v ./internal/service
```

### Code Formatting

```bash
# Format all code
go fmt ./...

# Run linter (if golangci-lint is installed)
golangci-lint run
```

### Database Management

The application uses SQLite with automatic migrations on startup. The database file is created at `data/who-live-when.db`.

To reset the database:
```bash
rm data/who-live-when.db
./server  # Migrations run automatically
```

### Project Structure

- `cmd/server/` - Application entry point and server initialization
- `internal/adapter/` - Platform API integrations with tests
- `internal/auth/` - Google OAuth implementation
- `internal/domain/` - Core models and interface definitions
- `internal/handler/` - HTTP handlers (public and authenticated routes)
- `internal/repository/` - Data persistence with SQLite implementation
- `internal/service/` - Business logic for streamers, heatmaps, TV programmes, etc.
- `static/` - CSS, JavaScript, and images
- `templates/` - HTML templates

## Technology Stack

- **Language**: Go 1.24.4
- **Database**: SQLite with WAL mode
- **Authentication**: Google OAuth 2.0
- **HTTP Server**: Go standard library
- **Testing**: Go testing package + gopter for property-based tests

### Key Dependencies

- `modernc.org/sqlite` - Pure Go SQLite driver
- `golang.org/x/oauth2` - OAuth 2.0 client
- `github.com/google/uuid` - UUID generation
- `github.com/leanovate/gopter` - Property-based testing

## API Endpoints

### Public Routes

- `GET /` - Home page with most viewed streamers (global programme)
- `GET /search` - Dedicated search page for discovering streamers (accessible to all users)
- `GET /streamer/:id` - Streamer detail page with heatmap
- `GET /login` - Initiate Google OAuth flow
- `GET /auth/google/callback` - OAuth callback handler
- `GET /logout` - End user session

### Universal Routes (Guest & Authenticated)

- `POST /search` - Search for streamers across enabled platforms
- `POST /follow/:id` - Follow a streamer (database for registered, session for guests)
- `POST /unfollow/:id` - Unfollow a streamer
- `GET /programme` - View custom or global programme

### Authenticated Routes

- `GET /dashboard` - User dashboard with followed streamers and custom programme
- `GET /programme/manage` - Custom programme management interface
- `POST /programme/create` - Create a custom programme
- `POST /programme/update` - Update custom programme streamers
- `POST /programme/delete` - Delete custom programme and revert to global
- `GET /calendar` - Weekly TV programme calendar (custom or global)

## How It Works

### Activity Heatmaps

The system analyzes historical streaming data from the past year to generate probability heatmaps:

- **Weighted Calculation**: 80% weight on last 3 months, 20% on older data
- **Time Slots**: Hourly probability distribution across the week
- **Prediction**: Most likely streaming times based on historical patterns
- **Formula**: `P(hour) = 0.8 * P_recent(hour) + 0.2 * P_older(hour)`

### Live Status Tracking

- Queries platform APIs (YouTube, Twitch, Kick) for real-time status
- Caches results for 1 hour to reduce API calls
- Falls back to cached data if platform APIs are unavailable
- Parallel queries for multi-platform streamers

### TV Programme Generation

Creates a weekly schedule showing predicted streaming times:

- Uses heatmap data to predict most likely live times
- Combines day and hour probabilities: `P(day, hour) = P(day) * P(hour)`
- Filters low-probability slots (< 5%) to reduce calendar clutter
- Displays followed streamers in a calendar view
- Supports week navigation for future planning

## Documentation

Additional documentation is available in the `docs/` directory:

- **[API.md](docs/API.md)**: Complete API endpoint documentation
- **[PLATFORM_ADAPTERS.md](docs/PLATFORM_ADAPTERS.md)**: Guide for implementing new platform adapters

## Contributing

When adding new features:

1. Follow the existing architecture patterns (Clean Architecture, Repository Pattern)
2. Add tests for new functionality (unit tests + property-based tests where applicable)
3. Update documentation (README, API docs, inline comments)
4. Ensure code passes `go fmt` and `golangci-lint`
5. Keep functions focused and under 50 lines when possible