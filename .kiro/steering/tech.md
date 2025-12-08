# Technology Stack

## Language & Runtime

- **Go 1.24.4**: Primary language
- Standard library HTTP server (no external web framework)

## Database

- **SQLite** (modernc.org/sqlite): Embedded database with WAL mode enabled
- Migrations managed in `internal/repository/sqlite/migrations.go`
- Connection pooling configured (25 max open, 5 max idle)

## Key Dependencies

- `golang.org/x/oauth2`: Google OAuth authentication
- `github.com/google/uuid`: UUID generation
- `github.com/leanovate/gopter`: Property-based testing
- `modernc.org/sqlite`: Pure Go SQLite driver

## Architecture Patterns

- **Clean Architecture**: Domain-driven design with clear layer separation
  - `domain`: Core business models and interfaces
  - `service`: Business logic implementation
  - `repository`: Data persistence layer
  - `handler`: HTTP request handlers
  - `adapter`: External platform API integrations
  - `auth`: Authentication and session management

- **Dependency Injection**: Services receive dependencies via constructors
- **Repository Pattern**: Abstract data access behind interfaces
- **Adapter Pattern**: Platform-specific APIs (YouTube, Twitch, Kick) implement common interface

## Common Commands

```bash
# Build the server
go build -o server ./cmd/server

# Run the server
./server

# Run all tests
go test ./...

# Run tests for specific package
go test ./internal/service

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...

# Format code
go fmt ./...

# Run linter (if golangci-lint installed)
golangci-lint run
```

## Environment Variables

- `GOOGLE_CLIENT_ID`: Google OAuth client ID
- `GOOGLE_CLIENT_SECRET`: Google OAuth client secret
- `GOOGLE_REDIRECT_URL`: OAuth callback URL (defaults to http://localhost:8080/auth/callback)

## Server Configuration

- Port: 8080
- Read timeout: 15s
- Write timeout: 15s
- Idle timeout: 60s
- Graceful shutdown with 30s timeout
