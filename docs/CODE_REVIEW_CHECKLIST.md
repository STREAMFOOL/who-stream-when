# Code Review Checklist

This document provides a checklist for reviewing code changes in the Who Live When project.

## General Code Quality

- [ ] Code follows Go conventions and idioms
- [ ] Functions are focused and under 50 lines when possible
- [ ] Files are under 300 lines (split into logical units if larger)
- [ ] No redundant comments that merely restate what code does
- [ ] Complex logic has explanatory comments about assumptions or context
- [ ] Error messages are descriptive and include context
- [ ] All exported functions and types have godoc comments

## Architecture & Design

- [ ] Changes follow Clean Architecture principles
- [ ] Layer separation is maintained (domain, service, repository, handler, adapter)
- [ ] Dependencies flow inward (handlers → services → repositories)
- [ ] Domain models have no external dependencies
- [ ] Services receive dependencies via constructors (dependency injection)
- [ ] Interfaces are defined in the domain layer

## Testing

- [ ] New functionality has unit tests
- [ ] Property-based tests are added for universal properties
- [ ] Tests follow table-driven pattern where appropriate
- [ ] Test names follow `Test<Function>_<Scenario>` pattern
- [ ] Tests use `t.Fatal` for setup failures, `t.Error` for assertions
- [ ] Mock implementations are minimal and focused
- [ ] Tests achieve reasonable coverage (aim for 80%+ on service layer)

## Error Handling

- [ ] Errors are wrapped with context using `fmt.Errorf` with `%w`
- [ ] Sentinel errors are exported and documented
- [ ] HTTP handlers return appropriate status codes
- [ ] Database errors are logged with context
- [ ] Platform API failures are logged and handled gracefully
- [ ] User-facing error messages are clear and actionable

## Security

- [ ] User input is validated before processing
- [ ] SQL queries use prepared statements (no string concatenation)
- [ ] Authentication is required for protected routes
- [ ] Session cookies are HttpOnly and Secure (in production)
- [ ] OAuth state tokens are validated
- [ ] API credentials are loaded from environment variables

## Performance

- [ ] Database queries are efficient (use indexes where appropriate)
- [ ] HTTP clients are reused (not created per request)
- [ ] Caching is used for expensive operations (live status, heatmaps)
- [ ] Timeouts are set for external API calls
- [ ] Connection pooling is configured for database

## Documentation

- [ ] README is updated for new features
- [ ] API documentation reflects endpoint changes
- [ ] Platform adapter guide is updated for new platforms
- [ ] Inline comments explain complex algorithms
- [ ] Environment variables are documented

## Database

- [ ] Migrations are idempotent (can be run multiple times safely)
- [ ] Schema changes are backward compatible when possible
- [ ] Indexes are added for frequently queried columns
- [ ] Foreign key constraints are used for referential integrity
- [ ] Transactions are used for multi-step operations

## Platform Adapters

- [ ] Implement all three interface methods (GetLiveStatus, SearchStreamer, GetChannelInfo)
- [ ] Handle offline status correctly (return IsLive: false, not error)
- [ ] Log API failures with context
- [ ] Set appropriate HTTP timeouts
- [ ] Parse responses with strongly-typed structs
- [ ] Handle rate limiting gracefully
- [ ] Return structured errors with context

## HTTP Handlers

- [ ] Request parameters are validated
- [ ] Errors are logged before returning to user
- [ ] Templates are rendered with proper error handling
- [ ] HTMX requests return appropriate fragments
- [ ] Session management is correct (create, validate, destroy)
- [ ] Redirects use appropriate status codes (302, 303)

## Services

- [ ] Business logic is separated from HTTP concerns
- [ ] Services coordinate between repositories and adapters
- [ ] Input validation happens at service layer
- [ ] Services return domain models, not database models
- [ ] Complex calculations have explanatory comments
- [ ] Services handle missing data gracefully

## Git Commit Messages

- [ ] Commit messages are descriptive
- [ ] Commits are atomic (one logical change per commit)
- [ ] Breaking changes are clearly marked
- [ ] Related issue/task numbers are referenced

## Before Merging

- [ ] All tests pass (`go test ./...`)
- [ ] Code is formatted (`go fmt ./...`)
- [ ] Linter passes (if using golangci-lint)
- [ ] No debug logging or commented-out code
- [ ] Documentation is updated
- [ ] Breaking changes are communicated

## Common Issues to Watch For

### Database

- **Issue**: SQL injection vulnerabilities
- **Solution**: Always use prepared statements with placeholders

- **Issue**: Database connection leaks
- **Solution**: Always defer `rows.Close()` and `tx.Rollback()`

- **Issue**: Race conditions in concurrent access
- **Solution**: Use transactions for multi-step operations

### Platform Adapters

- **Issue**: Treating offline status as an error
- **Solution**: Return `IsLive: false` when stream is not live

- **Issue**: Not handling rate limits
- **Solution**: Implement caching and exponential backoff

- **Issue**: Hardcoded credentials
- **Solution**: Load from environment variables

### Services

- **Issue**: Mixing HTTP concerns with business logic
- **Solution**: Keep services independent of HTTP layer

- **Issue**: Not validating input
- **Solution**: Validate at service layer before processing

- **Issue**: Returning database errors directly
- **Solution**: Wrap errors with context using `fmt.Errorf`

### Testing

- **Issue**: Tests depend on external services
- **Solution**: Use mocks or test doubles

- **Issue**: Tests are flaky (pass/fail randomly)
- **Solution**: Avoid time-dependent tests, use fixed test data

- **Issue**: Tests don't clean up resources
- **Solution**: Use `defer` for cleanup, use in-memory database for tests

## Performance Checklist

- [ ] No N+1 query problems (use joins or batch queries)
- [ ] Expensive operations are cached
- [ ] Database indexes exist for frequently queried columns
- [ ] HTTP clients are reused across requests
- [ ] Large result sets are paginated
- [ ] Timeouts prevent hanging requests

## Accessibility Checklist

- [ ] HTML templates use semantic elements
- [ ] Forms have proper labels
- [ ] Images have alt text
- [ ] Color is not the only indicator of state
- [ ] Keyboard navigation works
- [ ] ARIA attributes are used where appropriate
