# Project Structure

## Directory Layout

```
.
├── cmd/server/          # Application entry point
│   └── main.go          # Server initialization and routing
├── internal/            # Private application code
│   ├── adapter/         # Platform API integrations (YouTube, Twitch, Kick)
│   ├── auth/            # OAuth and session management
│   ├── cache/           # Caching layer (empty, future use)
│   ├── domain/          # Core business models and interfaces
│   │   ├── interfaces.go  # Service and adapter interfaces
│   │   └── models.go      # Domain entities
│   ├── handler/         # HTTP request handlers
│   │   ├── public.go      # Unauthenticated routes
│   │   └── authenticated.go # Protected routes
│   ├── logger/          # Logging utilities (empty, future use)
│   ├── middleware/      # HTTP middleware (empty, future use)
│   ├── repository/      # Data persistence layer
│   │   ├── interfaces.go  # Repository interfaces
│   │   └── sqlite/        # SQLite implementation
│   ├── service/         # Business logic layer
│   └── task/            # Background tasks (empty, future use)
├── pkg/                 # Public libraries (empty)
├── static/              # Static assets (CSS, JS, images)
├── templates/           # HTML templates
└── data/                # SQLite database file location (runtime)
```

## Code Organization Principles

### Domain Layer (`internal/domain/`)
- Contains pure business models with no external dependencies
- Defines service and adapter interfaces
- No implementation details, only contracts

### Service Layer (`internal/service/`)
- Implements business logic defined in domain interfaces
- Orchestrates between repositories and adapters
- Contains validation and error handling
- Each service has a corresponding `_test.go` file

### Repository Layer (`internal/repository/`)
- Abstracts data persistence behind interfaces
- SQLite implementation in `sqlite/` subdirectory
- Handles database migrations
- Each repository focuses on a single entity or aggregate

### Handler Layer (`internal/handler/`)
- Separated into public and authenticated handlers
- Handlers receive services via dependency injection
- Responsible for HTTP concerns (parsing, rendering, status codes)
- Template rendering with fallback to simple HTML

### Adapter Layer (`internal/adapter/`)
- Implements `PlatformAdapter` interface for each streaming platform
- Encapsulates platform-specific API calls
- Returns normalized domain models

## Naming Conventions

- **Files**: lowercase with underscores (e.g., `live_status.go`)
- **Test files**: `_test.go` suffix (e.g., `streamer_test.go`)
- **Interfaces**: Defined in `interfaces.go` files, named with purpose (e.g., `StreamerService`)
- **Constructors**: `New<Type>` pattern (e.g., `NewStreamerService`)
- **Errors**: Exported sentinel errors prefixed with `Err` (e.g., `ErrStreamerNotFound`)

## Testing Conventions

- Table-driven tests preferred for multiple scenarios
- Mock implementations use `mock<Type>` naming
- Test names follow `Test<Function>_<Scenario>` pattern
- Use `t.Fatal` for setup failures, `t.Error` for assertion failures
- Mock repositories implement full interface with configurable errors

## Code Style

- **Comments**: Only comment to explain implicit assumptions or broader context. Avoid and remove all redundant comments that merely restate what the code does
- **File length**: Keep files under 300 lines. Break large files into logical units; if unclear, create a `utils.go` file for the package
- **Function length**: Keep functions under 50 lines for readability. Long functions should be split into smaller, focused functions

## Maintenance Guidelines

- **Module structure**: Review and update folder/module organization after major additions. Consider dependency chains and group files into logical modules
- **Documentation sync**: When project structure changes, update steering docs and specs to reflect the current system state
