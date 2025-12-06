# Requirements Document: Who Live When - Streamer Tracking MVP

## Introduction

"Who Live When" is a streamer-tracking application that helps users discover and monitor live streaming activity across multiple platforms (YouTube, Kick, Twitch). The MVP focuses on core functionality: tracking streamer availability, displaying live status with platform information, and providing activity heatmaps. The system supports two user modes: read-only access for unregistered users viewing tracked streamers, and full access for registered users who can search, follow, and contribute streamer data. The application uses Go with its built-in template library for the backend and plain HTML/JS/CSS with HTMX for frontend reactivity.

## Glossary

- **Streamer**: A content creator who broadcasts on one or more streaming platforms
- **Platform**: A streaming service (YouTube, Kick, Twitch)
- **Live Status**: Current state indicating whether a streamer is actively broadcasting
- **Heatmap**: Visual representation of streamer activity patterns across hours of the day
- **Follow**: Action by a registered user to subscribe to a streamer's activity notifications and make them discoverable
- **Read-only Mode**: Access level for unregistered users limited to viewing pre-tracked streamers
- **Google SSO**: Single Sign-On authentication using Google OAuth
- **Activity Pattern**: Historical data of when a streamer typically goes live
- **TV Programme**: Weekly schedule showing predicted live times for followed streamers
- **Handle**: Platform-specific username or identifier for a streamer

## Requirements

### Requirement 1: Streamer Availability Tracking

**User Story:** As a system, I want to track which streamers are available on which platforms, so that users can discover and monitor streaming activity.

#### Acceptance Criteria

1. WHEN a streamer is added to the system THEN the system SHALL store the streamer's name, handle, and associated platforms
2. WHEN a streamer's platform information is updated THEN the system SHALL reflect the changes in the database
3. WHEN retrieving streamer data THEN the system SHALL return complete information including all associated platforms
4. WHERE a streamer has multiple platform handles THEN the system SHALL maintain separate identifiers for each platform

### Requirement 2: Live Status Display

**User Story:** As a user, I want to see which streamers are currently live and on which platform, so that I can quickly access their streams.

#### Acceptance Criteria

1. WHEN a user views the home page THEN the system SHALL display a default week view showing the most viewed streamers
2. WHEN a streamer goes live THEN the system SHALL update the live status and display a direct link to the stream
3. WHEN a streamer is offline THEN the system SHALL display offline status without a stream link
4. IF a platform link cannot be retrieved THEN the system SHALL display the platform name without a clickable link

### Requirement 3: Activity Heatmap Generation

**User Story:** As a user, I want to see a heatmap of streamer activity patterns, so that I can understand when a streamer typically goes live.

#### Acceptance Criteria

1. WHEN viewing a streamer's profile THEN the system SHALL display a heatmap showing activity by hour of day
2. WHEN generating the heatmap THEN the system SHALL analyze historical live data from the past year
3. WHEN calculating activity probability THEN the system SHALL weight recent data (last 3 months) at 80% and older data at 20%
4. WHEN a streamer has insufficient historical data THEN the system SHALL display a message indicating limited data availability

### Requirement 4: Weekly TV Programme Generation

**User Story:** As a registered user, I want to see a weekly schedule of when my followed streamers are likely to go live, so that I can plan my viewing time.

#### Acceptance Criteria

1. WHEN a registered user views their TV programme THEN the system SHALL display a weekly calendar showing predicted live times
2. WHEN generating predictions THEN the system SHALL use activity patterns from the past year with 80% weight on the last 3 months
3. WHEN a streamer has no historical data THEN the system SHALL display the streamer without predictions
4. WHEN the week changes THEN the system SHALL update the TV programme to reflect the new week

### Requirement 5: Read-only User Access

**User Story:** As an unregistered user, I want to browse tracked streamers without registration, so that I can quickly check streamer status.

#### Acceptance Criteria

1. WHEN an unregistered user accesses the application THEN the system SHALL display a default week view with the most viewed streamers
2. WHEN an unregistered user attempts to search for a streamer THEN the system SHALL prevent the search and prompt for registration
3. WHEN an unregistered user views a streamer THEN the system SHALL display live status and heatmap without follow functionality
4. WHILE a user is unregistered THEN the system SHALL not allow adding new streamers to the tracked list

### Requirement 6: Google SSO Authentication

**User Story:** As a user, I want to register and log in using Google SSO, so that I can access registered user features without creating a new password.

#### Acceptance Criteria

1. WHEN a user clicks the login button THEN the system SHALL redirect to Google OAuth authentication
2. WHEN Google authentication succeeds THEN the system SHALL create or retrieve the user account and establish a session
3. WHEN a user logs out THEN the system SHALL clear the session and return to the public view
4. IF Google authentication fails THEN the system SHALL display an error message and return to the login page

### Requirement 7: Streamer Search for Registered Users

**User Story:** As a registered user, I want to search for any streamer by name or handle, so that I can discover and follow new streamers.

#### Acceptance Criteria

1. WHEN a registered user enters a search query THEN the system SHALL search external platform APIs for matching streamers
2. WHEN search results are returned THEN the system SHALL display streamer name, handle, and available platforms
3. WHEN a search returns no results THEN the system SHALL display a message indicating no streamers were found
4. WHEN a registered user searches THEN the system SHALL query at least YouTube, Kick, and Twitch platforms

### Requirement 8: Follow Functionality for Registered Users

**User Story:** As a registered user, I want to follow streamers and make them discoverable to other users, so that I can build a community of tracked content creators.

#### Acceptance Criteria

1. WHEN a registered user follows a streamer THEN the system SHALL add the streamer to their followed list and make it available for search by other users
2. WHEN a registered user unfollows a streamer THEN the system SHALL remove the streamer from their followed list
3. WHEN a registered user views their followed streamers THEN the system SHALL display all streamers they have followed
4. WHEN a streamer is followed by a user THEN the system SHALL begin collecting live activity data for heatmap generation

### Requirement 9: Calendar View for Followed Streamers

**User Story:** As a registered user, I want to see my followed streamers displayed in a calendar view similar to Google Calendar, so that I can visualize their activity patterns and plan my viewing schedule.

#### Acceptance Criteria

1. WHEN a registered user views their dashboard THEN the system SHALL display a weekly calendar grid showing followed streamers
2. WHEN a streamer is predicted to go live THEN the system SHALL display the streamer name in the corresponding time slot
3. WHEN a user clicks on a calendar entry THEN the system SHALL display streamer details and a link to their stream if currently live
4. WHEN the calendar is displayed THEN the system SHALL show the current week with navigation to previous and next weeks

### Requirement 10: Multi-Platform Support

**User Story:** As the system, I want to support YouTube, Kick, and Twitch platforms, so that users can track streamers across major streaming services.

#### Acceptance Criteria

1. WHEN a streamer is added THEN the system SHALL support associating them with YouTube, Kick, and/or Twitch
2. WHEN querying live status THEN the system SHALL check all associated platforms for the streamer
3. WHEN displaying platform information THEN the system SHALL show which platforms the streamer uses
4. WHEN a platform API is unavailable THEN the system SHALL display cached data or indicate the platform is temporarily unavailable
