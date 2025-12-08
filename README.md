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

```bash
export GOOGLE_CLIENT_ID="your-google-client-id"
export GOOGLE_CLIENT_SECRET="your-google-client-secret"
export GOOGLE_REDIRECT_URL="http://localhost:8080/auth/callback"
```

### Running

```bash
# Run the server
./server
```

The server will start on `http://localhost:8080`

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

- `GET /` - Home page with most viewed streamers
- `GET /streamer/:id` - Streamer detail page with heatmap
- `GET /login` - Initiate Google OAuth flow
- `GET /auth/callback` - OAuth callback handler
- `GET /logout` - End user session

### Authenticated Routes

- `GET /dashboard` - User dashboard with followed streamers
- `POST /search` - Search for streamers across platforms
- `POST /follow/:id` - Follow a streamer
- `POST /unfollow/:id` - Unfollow a streamer
- `GET /calendar` - Weekly TV programme calendar

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