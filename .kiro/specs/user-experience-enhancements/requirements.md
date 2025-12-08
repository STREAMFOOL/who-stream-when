# Requirements Document: User Experience Enhancements

## Introduction

This feature enhances the "Who Live When" application by removing access restrictions for unregistered users, enabling custom programme creation for all users, and improving system configuration. The enhancements focus on making the application more accessible while maintaining the value of user accounts for personalization. Key improvements include allowing all users to search and follow streamers, enabling custom programme creation for both registered and unregistered users (with session-based storage for guests), and externalizing configuration through environment variables for better deployment flexibility.

## Glossary

- **Custom Programme**: A personalized weekly schedule created by selecting specific streamers to track
- **Global Programme**: The default weekly schedule showing the most viewed streamers across the platform
- **Session-based Storage**: Temporary storage mechanism for unregistered users that persists data during their browser session
- **Environment Variable**: External configuration value read from the system environment at application startup
- **Guest User**: An unregistered user accessing the application without authentication
- **Database Connection String**: Configuration parameter specifying how to connect to the SQLite database

## Requirements

### Requirement 1: Universal Search Access

**User Story:** As any user (registered or unregistered), I want to search for streamers across all platforms, so that I can discover new content creators without needing to create an account.

#### Acceptance Criteria

1. WHEN any user accesses the search functionality THEN the system SHALL allow the search without requiring authentication
2. WHEN a search query is submitted THEN the system SHALL query YouTube, Kick, and Twitch platforms and return matching streamers
3. WHEN search results are displayed THEN the system SHALL show streamer name, handle, platform information, and current live status
4. WHEN no results are found THEN the system SHALL display a message indicating no streamers match the query

### Requirement 2: Universal Follow Functionality

**User Story:** As any user (registered or unregistered), I want to follow streamers, so that I can build a personalized list of content creators to track.

#### Acceptance Criteria

1. WHEN a registered user follows a streamer THEN the system SHALL persist the follow relationship in the database
2. WHEN an unregistered user follows a streamer THEN the system SHALL store the follow relationship in the browser session
3. WHEN a user views their followed streamers THEN the system SHALL display all streamers they have followed regardless of authentication status
4. WHEN an unregistered user closes their browser THEN the system SHALL clear their session-based follows

### Requirement 3: Custom Programme Creation for All Users

**User Story:** As any user (registered or unregistered), I want to create a custom programme by selecting specific streamers, so that I can view a personalized weekly schedule.

#### Acceptance Criteria

1. WHEN a registered user creates a custom programme THEN the system SHALL persist the programme configuration in the database
2. WHEN an unregistered user creates a custom programme THEN the system SHALL store the programme configuration in the browser session
3. WHEN a user adds streamers to their custom programme THEN the system SHALL update the weekly calendar to show only selected streamers
4. WHEN a user removes streamers from their custom programme THEN the system SHALL update the calendar to exclude those streamers
5. WHERE a user has not created a custom programme THEN the system SHALL display the global programme showing most viewed streamers

### Requirement 4: Global Programme Default View

**User Story:** As a user without a custom programme, I want to see the global programme showing popular streamers, so that I have immediate value without configuration.

#### Acceptance Criteria

1. WHEN a user accesses the home page without a custom programme THEN the system SHALL display the global programme with most viewed streamers
2. WHEN the global programme is displayed THEN the system SHALL show streamers ranked by follower count
3. WHEN a user creates a custom programme THEN the system SHALL switch from global to custom programme view
4. WHEN a user deletes their custom programme THEN the system SHALL revert to displaying the global programme

### Requirement 5: Environment-Based Configuration

**User Story:** As a system administrator, I want to configure the application through environment variables, so that I can deploy to different environments without code changes.

#### Acceptance Criteria

1. WHEN the application starts THEN the system SHALL read the database connection string from an environment variable
2. WHEN the application starts THEN the system SHALL read Google OAuth credentials from environment variables
3. WHEN the application starts THEN the system SHALL read platform API keys from environment variables
4. WHERE an environment variable is not set THEN the system SHALL use a documented default value or fail with a clear error message
5. WHEN environment variables are updated THEN the system SHALL apply the new configuration on restart

### Requirement 6: Enhanced Test Coverage

**User Story:** As a developer, I want comprehensive test coverage across all components, so that I can confidently make changes without introducing regressions.

#### Acceptance Criteria

1. WHEN tests are executed THEN the system SHALL achieve minimum 80% code coverage for service layer
2. WHEN tests are executed THEN the system SHALL achieve minimum 70% code coverage for handler layer
3. WHEN tests are executed THEN the system SHALL achieve minimum 80% code coverage for repository layer
4. WHEN critical paths are tested THEN the system SHALL include property-based tests for data transformations
5. WHEN integration points are tested THEN the system SHALL verify correct interaction between layers

### Requirement 7: Session Management for Guest Users

**User Story:** As an unregistered user, I want my follows and custom programme to persist during my browsing session, so that I can use the application without losing my selections.

#### Acceptance Criteria

1. WHEN an unregistered user follows a streamer THEN the system SHALL store the follow in a session cookie
2. WHEN an unregistered user creates a custom programme THEN the system SHALL store the programme in a session cookie
3. WHEN an unregistered user navigates between pages THEN the system SHALL maintain their follows and custom programme
4. WHEN the session expires or browser closes THEN the system SHALL clear all session-based data
5. WHEN an unregistered user registers or logs in THEN the system SHALL migrate their session data to persistent storage

### Requirement 8: Streamer Addition from Search Results

**User Story:** As any user, I want to add streamers from search results directly to the system, so that they become available for following and tracking.

#### Acceptance Criteria

1. WHEN a user searches for a streamer not in the system THEN the system SHALL display an option to add the streamer
2. WHEN a user adds a streamer from search results THEN the system SHALL create a streamer record with platform information
3. WHEN a streamer is added THEN the system SHALL begin tracking their live status and activity data
4. WHEN a streamer already exists in the system THEN the system SHALL display existing streamer information without duplication

### Requirement 9: Custom Programme Management Interface

**User Story:** As any user, I want to manage my custom programme through a dedicated interface, so that I can easily add, remove, and reorder streamers.

#### Acceptance Criteria

1. WHEN a user accesses the programme management interface THEN the system SHALL display their current custom programme or indicate they are using the global programme
2. WHEN a user adds a streamer to their custom programme THEN the system SHALL update the programme and refresh the calendar view
3. WHEN a user removes a streamer from their custom programme THEN the system SHALL update the programme and refresh the calendar view
4. WHEN a user clears their custom programme THEN the system SHALL revert to the global programme view
5. WHERE a user is unregistered THEN the system SHALL display a notice that their programme is session-based

### Requirement 10: Configuration Validation and Error Handling

**User Story:** As a system administrator, I want clear error messages when configuration is invalid, so that I can quickly diagnose and fix deployment issues.

#### Acceptance Criteria

1. WHEN a required environment variable is missing THEN the system SHALL log a clear error message and fail to start
2. WHEN an environment variable has an invalid format THEN the system SHALL log the validation error and fail to start
3. WHEN the database connection string is invalid THEN the system SHALL log the connection error with details
4. WHEN platform API credentials are invalid THEN the system SHALL log a warning and continue with limited functionality
5. WHEN the application starts successfully THEN the system SHALL log all loaded configuration values (excluding secrets)
