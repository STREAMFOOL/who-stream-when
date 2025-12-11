# Implementation Plan: User Experience Enhancements

- [x] 1. Implement configuration management with environment variables
  - Create configuration service that reads from environment variables
  - Add validation for required configuration values
  - Implement configuration logging (excluding secrets)
  - Update main.go to use configuration service
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 10.1, 10.2, 10.3, 10.4, 10.5_

- [x] 1.1 Write unit tests for configuration loading and validation
  - Test loading with valid environment variables
  - Test missing required variables
  - Test invalid format variables
  - Test default value fallbacks
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 10.1, 10.2_

- [x] 2. Enhance session manager for guest data storage
  - Extend SessionManager to store guest follows in cookies
  - Add methods for guest custom programme storage
  - Implement session data serialization/deserialization
  - Add session data size validation and compression if needed
  - _Requirements: 2.2, 3.2, 7.1, 7.2, 7.3_

- [x] 2.1 Write property test for session data persistence
  - **Property 4: Guest User Follow Session Storage**
  - **Validates: Requirements 2.2, 7.1**

- [x] 2.2 Write property test for session data across requests
  - **Property 11: Session Data Persistence Across Requests**
  - **Validates: Requirements 7.3**

- [x] 2.3 Write unit tests for session manager
  - Test cookie serialization/deserialization
  - Test session data size limits
  - Test corrupted session data handling
  - _Requirements: 2.2, 3.2, 7.1, 7.2_

- [x] 3. Create custom programme data model and repository
  - Define CustomProgramme struct in domain models
  - Create custom_programme table migration
  - Implement CustomProgrammeRepository interface
  - Implement SQLite repository for custom programmes
  - _Requirements: 3.1, 3.2_

- [x] 3.1 Write property test for custom programme persistence
  - **Property 6: Custom Programme Database Persistence**
  - **Validates: Requirements 3.1**

- [x] 3.2 Write unit tests for custom programme repository
  - Test CRUD operations
  - Test user isolation (users can't access other users' programmes)
  - Test concurrent updates
  - _Requirements: 3.1_

- [x] 4. Implement programme service for custom programme management
  - Create ProgrammeService with custom programme CRUD operations
  - Implement guest programme creation (session-based)
  - Implement calendar generation from custom programme
  - Implement global programme generation with ranking by followers
  - Add logic to determine which programme to show (custom vs global)
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 4.1, 4.2, 4.3, 4.4_

- [x] 4.1 Write property test for custom programme calendar filtering
  - **Property 8: Custom Programme Calendar Filtering**
  - **Validates: Requirements 3.3, 9.2**

- [x] 4.2 Write property test for streamer removal from programme
  - **Property 9: Streamer Removal from Programme**
  - **Validates: Requirements 3.4, 9.3**

- [x] 4.3 Write property test for global programme ranking
  - **Property 10: Global Programme Ranking**
  - **Validates: Requirements 4.2**

- [x] 4.4 Write unit tests for programme service
  - Test custom programme creation and retrieval
  - Test programme updates and deletions
  - Test default to global programme when no custom programme exists
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 4.1, 4.2, 4.3, 4.4_

- [x] 5. Remove authentication requirements from search functionality
  - Move search handler from authenticated to public handler
  - Update routing to allow unauthenticated search access
  - Update search UI to work for both authenticated and guest users
  - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [x] 5.1 Write property test for multi-platform search coverage
  - **Property 1: Multi-Platform Search Coverage**
  - **Validates: Requirements 1.2**

- [x] 5.2 Write property test for search result completeness
  - **Property 2: Search Result Completeness**
  - **Validates: Requirements 1.3**

- [x] 5.3 Write unit tests for public search handler
  - Test unauthenticated search access
  - Test search with no results
  - Test search result display
  - _Requirements: 1.1, 1.4_

- [-] 6. Implement universal follow functionality with dual storage
  - Update UserService to support both database and session-based follows
  - Modify follow/unfollow handlers to work for guest users
  - Implement GetGuestFollows method to retrieve streamers by IDs
  - Update follow UI to work for both user types
  - _Requirements: 2.1, 2.2, 2.3, 2.4_

- [x] 6.1 Write property test for registered user follow persistence
  - **Property 3: Registered User Follow Persistence**
  - **Validates: Requirements 2.1**

- [x] 6.2 Write property test for follow list completeness
  - **Property 5: Follow List Completeness**
  - **Validates: Requirements 2.3**

- [x] 6.3 Write unit tests for dual-storage follow functionality
  - Test registered user follows (database)
  - Test guest user follows (session)
  - Test follow/unfollow operations
  - Test session expiry for guest follows
  - _Requirements: 2.1, 2.2, 2.3, 2.4_

- [x] 7. Implement streamer creation from search results
  - Add CreateStreamerFromSearchResult method to StreamerService
  - Implement GetOrCreateStreamer for idempotent streamer creation
  - Update search results UI to show "Add Streamer" option
  - Add handler for adding streamers from search results
  - Begin live status tracking when streamer is added
  - _Requirements: 8.1, 8.2, 8.3, 8.4_

- [x] 7.1 Write property test for streamer creation from search
  - **Property 13: Streamer Creation from Search Result**
  - **Validates: Requirements 8.2**

- [x] 7.2 Write property test for streamer addition idempotence
  - **Property 14: Streamer Addition Idempotence**
  - **Validates: Requirements 8.4**

- [x] 7.3 Write unit tests for streamer creation
  - Test creating streamer from search result
  - Test duplicate detection
  - Test live status tracking initiation
  - _Requirements: 8.1, 8.2, 8.3, 8.4_

- [x] 8. Implement guest data migration on registration
  - Add MigrateGuestData method to UserService
  - Update authentication callback to migrate session data
  - Implement transactional migration (all or nothing)
  - Clear session data after successful migration
  - _Requirements: 7.5_

- [x] 8.1 Write property test for guest data migration
  - **Property 12: Guest Data Migration on Registration**
  - **Validates: Requirements 7.5**

- [x] 8.2 Write unit tests for data migration
  - Test successful migration of follows and programme
  - Test migration rollback on failure
  - Test session cleanup after migration
  - _Requirements: 7.5_

- [ ] 9. Create programme management interface
  - Create programme management page template
  - Add handlers for programme CRUD operations
  - Implement UI for adding/removing streamers from programme
  - Add "Clear Programme" functionality to revert to global
  - Display session-based notice for guest users
  - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5_

- [ ] 9.1 Write unit tests for programme management handlers
  - Test programme creation and updates
  - Test programme deletion
  - Test UI state for custom vs global programme
  - Test guest user notice display
  - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5_

- [ ] 10. Update home page to use custom or global programme
  - Modify home page handler to check for custom programme
  - Display custom programme if exists, otherwise global programme
  - Add UI indicator showing which programme type is displayed
  - Update calendar view to reflect programme source
  - _Requirements: 3.5, 4.1, 4.3, 4.4_

- [ ] 10.1 Write unit tests for home page programme selection
  - Test display of custom programme when exists
  - Test fallback to global programme
  - Test programme type indicator
  - _Requirements: 3.5, 4.1, 4.3, 4.4_

- [ ] 11. Update dashboard for registered users with custom programme
  - Modify dashboard to show custom programme if exists
  - Add link to programme management interface
  - Display followed streamers with option to add to programme
  - Show calendar based on custom or global programme
  - _Requirements: 3.3, 3.5, 9.1_

- [ ] 11.1 Write unit tests for dashboard programme display
  - Test custom programme display
  - Test global programme fallback
  - Test programme management link
  - _Requirements: 3.3, 3.5, 9.1_

- [ ] 12. Add comprehensive error handling and logging
  - Implement error handling for session storage failures
  - Add error handling for configuration validation
  - Implement graceful degradation for platform API failures
  - Add logging for all configuration and session operations
  - Implement user-friendly error messages for all failure modes
  - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5_

- [ ] 12.1 Write unit tests for error handling
  - Test configuration error messages
  - Test session error handling
  - Test graceful degradation
  - _Requirements: 10.1, 10.2, 10.3, 10.4_

- [ ] 13. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 14. Update documentation and templates
  - Update README with new environment variable requirements
  - Document guest user limitations and session behavior
  - Update API documentation with new endpoints
  - Add inline documentation for new services and methods
  - _Requirements: 5.4, 7.4, 9.5_

- [ ] 15. Final integration testing and validation
  - Test complete guest user flow: search → follow → programme → register
  - Test registered user custom programme creation and management
  - Test configuration loading with various environment setups
  - Verify all correctness properties are validated by tests
  - _Requirements: All_

- [ ] 16. Final Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.
