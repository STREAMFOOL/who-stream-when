# Implementation Plan: Who Live When - Streamer Tracking MVP

## Overview

This implementation plan breaks down the feature design into discrete, manageable coding tasks. Each task builds incrementally on previous steps, starting with project structure and core interfaces, then implementing services, data models, and finally integrating all components with tests.

---

- [ ] 1. Set up project structure and core interfaces
  - Create Go project directory structure with `cmd/`, `internal/`, `pkg/` directories
  - Set up `go.mod` and `go.sum` with required dependencies (database driver, OAuth library, HTMX)
  - Create core interface definitions in `internal/domain/interfaces.go`
  - Set up HTTP server with basic routing in `cmd/server/main.go`
  - _Requirements: 1.1, 6.1_

- [ ] 2. Implement data models and storage layer with SQLite
  - Create data model structs in `internal/domain/models.go` (Streamer, LiveStatus, Heatmap, User, ActivityRecord, TVProgramme)
  - Set up SQLite database with connection pooling
  - Implement database schema and migrations using a migration tool
  - Create repository interfaces in `internal/repository/interfaces.go`
  - Implement SQLite-backed repositories for all data models
  - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [ ] 2.1 Write property test for streamer data persistence
  - **Property 1: Streamer Data Persistence**
  - **Validates: Requirements 1.1, 1.3**

- [ ] 2.2 Write property test for streamer update consistency
  - **Property 2: Streamer Update Consistency**
  - **Validates: Requirements 1.2**

- [ ] 2.3 Write property test for multi-platform handle isolation
  - **Property 3: Multi-Platform Handle Isolation**
  - **Validates: Requirements 1.4, 10.1**

- [ ] 3. Implement Streamer Service
  - Create `internal/service/streamer.go` with StreamerService implementation
  - Implement GetStreamer, ListStreamers, AddStreamer, UpdateStreamer methods
  - Implement GetStreamersByPlatform for multi-platform support
  - Add input validation for streamer data
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 10.1_

- [ ] 3.1 Write unit tests for Streamer Service
  - Test GetStreamer with valid and invalid IDs
  - Test AddStreamer with valid and invalid data
  - Test UpdateStreamer platform changes
  - Test GetStreamersByPlatform filtering

- [ ] 4. Implement Platform Adapters
  - Create `internal/adapter/platform.go` with PlatformAdapter interface
  - Implement YouTube adapter in `internal/adapter/youtube.go`
  - Implement Kick adapter in `internal/adapter/kick.go`
  - Implement Twitch adapter in `internal/adapter/twitch.go`
  - Each adapter should handle API calls, error handling, and response parsing
  - _Requirements: 10.1, 10.2, 10.3_

- [ ] 4.1 Write unit tests for Platform Adapters
  - Test each adapter's GetLiveStatus method with mock responses
  - Test error handling for unavailable platforms
  - Test SearchStreamer across all platforms

- [ ] 5. Implement Live Status Service
  - Create `internal/service/livestatus.go` with LiveStatusService implementation
  - Implement GetLiveStatus to query platform adapters
  - Implement RefreshLiveStatus with caching logic (1 hour TTL)
  - Implement GetAllLiveStatus for batch queries
  - Add error handling for platform failures with fallback to cached data
  - _Requirements: 2.1, 2.2, 2.3, 10.2, 10.3_

- [ ] 5.1 Write property test for live status completeness
  - **Property 4: Live Status Completeness**
  - **Validates: Requirements 2.1, 2.2, 2.3**

- [ ] 5.2 Write unit tests for Live Status Service
  - Test GetLiveStatus with live and offline streamers
  - Test caching behavior
  - Test error handling when platform is unavailable

- [ ] 6. Implement Activity Heatmap Service
  - Create `internal/service/heatmap.go` with HeatmapService implementation
  - Implement GenerateHeatmap with weighted calculation (80% last 3 months, 20% older)
  - Implement RecordActivity to store activity records
  - Implement GetActivityStats for statistics
  - Add logic to analyze historical data from past year
  - _Requirements: 3.1, 3.2, 3.3, 4.2_

- [ ] 6.1 Write property test for heatmap probability validity
  - **Property 5: Heatmap Probability Validity**
  - **Validates: Requirements 3.1, 3.2**

- [ ] 6.2 Write property test for weighted activity calculation
  - **Property 6: Weighted Activity Calculation**
  - **Validates: Requirements 3.3**

- [ ] 6.3 Write unit tests for Heatmap Service
  - Test GenerateHeatmap with various data distributions
  - Test weighting algorithm with known data
  - Test edge case: insufficient historical data

- [ ] 7. Implement TV Programme Service
  - Create `internal/service/tvprogramme.go` with TVProgrammeService implementation
  - Implement GenerateProgramme to create weekly schedules
  - Implement GetPredictedLiveTime using heatmap data
  - Use same weighting algorithm as heatmap service
  - Implement GetMostViewedStreamers and GetDefaultWeekView for home page
  - _Requirements: 4.1, 4.2, 4.3, 4.4_

- [ ] 7.1 Write property test for TV programme prediction consistency
  - **Property 7: TV Programme Prediction Consistency**
  - **Validates: Requirements 4.1, 4.2**

- [ ] 7.2 Write property test for week-based programme uniqueness
  - **Property 8: Week-Based Programme Uniqueness**
  - **Validates: Requirements 4.4**

- [ ] 7.3 Write unit tests for TV Programme Service
  - Test GenerateProgramme for different weeks
  - Test GetPredictedLiveTime accuracy
  - Test edge case: streamer with no historical data
  - Test GetMostViewedStreamers returns correct ordering

- [ ] 8. Implement User Service and Authentication
  - Create `internal/service/user.go` with UserService implementation
  - Implement GetUser, CreateUser methods
  - Implement FollowStreamer, UnfollowStreamer, GetUserFollows
  - Create `internal/auth/google.go` for Google OAuth integration
  - Implement session management with secure cookies
  - _Requirements: 6.1, 6.2, 6.3, 8.1, 8.2, 8.3_

- [ ] 8.1 Write property test for user session establishment
  - **Property 12: User Session Establishment**
  - **Validates: Requirements 6.2**

- [ ] 8.2 Write property test for session cleanup on logout
  - **Property 13: Session Cleanup on Logout**
  - **Validates: Requirements 6.3**

- [ ] 8.3 Write property test for follow operation idempotence
  - **Property 16: Follow Operation Idempotence**
  - **Validates: Requirements 8.1**

- [ ] 8.4 Write property test for unfollow removes relationship
  - **Property 17: Unfollow Removes Relationship**
  - **Validates: Requirements 8.2**

- [ ] 8.5 Write property test for followed streamer visibility
  - **Property 18: Followed Streamer Visibility**
  - **Validates: Requirements 8.1**

- [ ] 8.6 Write property test for activity tracking on follow
  - **Property 19: Activity Tracking on Follow**
  - **Validates: Requirements 8.4**

- [ ] 8.7 Write unit tests for User Service
  - Test CreateUser with Google OAuth data
  - Test FollowStreamer and UnfollowStreamer
  - Test GetUserFollows returns all followed streamers

- [ ] 9. Implement Streamer Search Service
  - Create `internal/service/search.go` with search logic
  - Implement SearchStreamers to query all platform adapters
  - Aggregate results from YouTube, Kick, and Twitch
  - Add deduplication logic for streamers appearing on multiple platforms
  - _Requirements: 7.1, 7.2, 7.4_

- [ ] 9.1 Write property test for multi-platform search coverage
  - **Property 14: Multi-Platform Search Coverage**
  - **Validates: Requirements 7.1, 7.4**

- [ ] 9.2 Write property test for search result completeness
  - **Property 15: Search Result Completeness**
  - **Validates: Requirements 7.2**

- [ ] 9.3 Write unit tests for Search Service
  - Test SearchStreamers with various queries
  - Test deduplication across platforms
  - Test edge case: no results found

- [ ] 10. Implement HTTP Handlers - Public Routes
  - Create `internal/handler/public.go` with public endpoints
  - Implement GET `/` for home page (default week view with most viewed streamers)
  - Implement GET `/streamer/:id` for streamer detail page
  - Implement GET `/login` for Google OAuth redirect
  - Implement GET `/auth/callback` for OAuth callback handling
  - Implement GET `/logout` for session cleanup
  - _Requirements: 2.1, 5.1, 6.1, 6.2, 6.3_

- [ ] 10.1 Write integration tests for public routes
  - Test home page displays only followed streamers
  - Test streamer detail page shows live status and heatmap
  - Test OAuth flow end-to-end

- [ ] 11. Implement HTTP Handlers - Authenticated Routes
  - Create `internal/handler/authenticated.go` with protected endpoints
  - Implement GET `/dashboard` for user dashboard
  - Implement POST `/search` for streamer search
  - Implement POST `/follow/:id` for following a streamer
  - Implement POST `/unfollow/:id` for unfollowing
  - Implement GET `/calendar` for TV programme calendar view
  - Add authentication middleware to verify user session
  - _Requirements: 5.2, 5.4, 7.1, 8.1, 8.2, 9.1, 9.2, 9.4_

- [ ] 11.1 Write integration tests for authenticated routes
  - Test search requires authentication
  - Test follow/unfollow operations
  - Test calendar displays correct week

- [ ] 12. Implement HTML Templates and Frontend
  - Create `templates/` directory structure
  - Implement base template with navigation and styling
  - Create `templates/home.html` for default week view with most viewed streamers
  - Create `templates/streamer.html` for streamer detail with heatmap
  - Create `templates/dashboard.html` for user dashboard
  - Create `templates/search.html` for search interface
  - Create `templates/calendar.html` for TV programme calendar
  - Add HTMX attributes for reactive updates (live status, search results)
  - _Requirements: 2.1, 3.1, 4.1, 5.1, 5.3, 9.1, 9.2_

- [ ] 12.1 Write unit tests for template rendering
  - Test templates render with correct data
  - Test conditional rendering (authenticated vs unregistered)

- [ ] 13. Implement Access Control Middleware
  - Create `internal/middleware/auth.go` with authentication checks
  - Implement RequireAuth middleware for protected routes
  - Implement OptionalAuth middleware for routes that work both ways
  - Add logic to restrict unregistered users from search and follow operations
  - Ensure default week view is accessible to both authenticated and unregistered users
  - _Requirements: 5.1, 5.2, 5.3, 5.4_

- [ ] 13.1 Write property test for read-only user visibility
  - **Property 9: Read-Only User Visibility**
  - **Validates: Requirements 5.1**

- [ ] 13.2 Write property test for search access control
  - **Property 10: Search Access Control**
  - **Validates: Requirements 5.2, 5.4**

- [ ] 13.3 Write property test for unregistered user feature restriction
  - **Property 11: Unregistered User Feature Restriction**
  - **Validates: Requirements 5.3**

- [ ] 13.4 Write unit tests for Access Control Middleware
  - Test RequireAuth blocks unauthenticated requests
  - Test OptionalAuth allows both authenticated and unauthenticated
  - Test follow operations blocked for unregistered users

- [ ] 14. Implement Activity Recording and Background Tasks
  - Create `internal/task/activity.go` for background activity tracking
  - Implement periodic task to check live status and record activity
  - Implement logic to record activity when streamer goes live
  - Store activity records with timestamps for heatmap generation
  - _Requirements: 8.4, 3.2, 3.3_

- [ ] 14.1 Write unit tests for Activity Recording
  - Test activity records are created when streamer goes live
  - Test activity data is used for heatmap generation

- [ ] 15. Implement Caching Layer
  - Create `internal/cache/cache.go` for caching logic
  - Implement TTL-based cache for live status (1 hour)
  - Implement cache invalidation on manual refresh
  - Add fallback to cached data when platform APIs are unavailable
  - _Requirements: 2.4, 10.4_

- [ ] 15.1 Write unit tests for Caching
  - Test cache TTL expiration
  - Test fallback to cached data on API failure

- [ ] 16. Implement Error Handling and Logging
  - Create `internal/logger/logger.go` for structured logging
  - Add error logging throughout services and handlers
  - Implement user-friendly error messages for common failures
  - Add monitoring for platform API failures
  - _Requirements: 2.4, 10.4_

- [ ] 16.1 Write unit tests for Error Handling
  - Test graceful handling of platform API failures
  - Test error messages are user-friendly

- [ ] 17. Checkpoint - Ensure all tests pass
  - Run all unit tests and verify they pass
  - Run all property-based tests and verify they pass
  - Run integration tests for critical paths
  - Ask the user if questions arise

- [ ] 18. Implement Calendar View Rendering
  - Create calendar rendering logic in `internal/service/calendar.go`
  - Implement week navigation (previous/next week)
  - Implement time slot mapping for predicted live times
  - Create `templates/calendar_week.html` for calendar display
  - Add HTMX for week navigation without page reload
  - _Requirements: 9.1, 9.2, 9.3, 9.4_

- [ ] 18.1 Write property test for calendar display accuracy
  - **Property 20: Calendar Display Accuracy**
  - **Validates: Requirements 9.1, 9.2**

- [ ] 18.2 Write property test for calendar navigation consistency
  - **Property 21: Calendar Navigation Consistency**
  - **Validates: Requirements 9.4**

- [ ] 18.3 Write integration tests for Calendar View
  - Test calendar displays correct week
  - Test navigation between weeks
  - Test predicted times appear in correct slots

- [ ] 19. Implement Platform Query Coverage
  - Ensure all multi-platform streamers query all associated platforms
  - Implement parallel queries for performance
  - Add timeout handling for slow platform APIs
  - _Requirements: 10.2, 10.3_

- [ ] 19.1 Write property test for platform query coverage
  - **Property 22: Platform Query Coverage**
  - **Validates: Requirements 10.2, 10.3**

- [ ] 19.2 Write unit tests for Multi-Platform Queries
  - Test all platforms are queried for multi-platform streamers
  - Test timeout handling

- [ ] 20. Final Checkpoint - Ensure all tests pass
  - Run complete test suite (unit, property-based, integration)
  - Verify all correctness properties are validated
  - Verify code coverage meets 80% minimum for service layer
  - Ask the user if questions arise

- [ ] 21. Documentation and Code Review
  - Add inline code comments for complex logic
  - Create README with setup and running instructions
  - Document API endpoints and their requirements
  - Document platform adapter implementation details
  - _Requirements: All_

---

## Notes

- Each task builds on previous tasks; ensure dependencies are satisfied before starting
- Property-based tests should run minimum 100 iterations
- Use Go's `testing` package for unit tests and `rapid` or `gopter` for property-based tests
- HTMX integration should be minimal and focused on reactive updates (live status, search results, calendar navigation)
- Platform adapters should be designed to allow easy addition of new platforms in the future
- All services should be dependency-injected for testability
