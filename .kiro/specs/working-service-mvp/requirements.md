# Requirements Document

## Introduction

This specification addresses critical issues preventing the "Who Live When" application from being a functional, usable service. The current state has several problems: broken navigation (clicking Home/Programme updates URL but doesn't navigate), Google SSO dependency blocking all users, poor UI/UX with nested headers and confusing layout, and no actual streamer data being displayed. This spec focuses on making the service work end-to-end for all users without authentication requirements.

## Glossary

- **System**: The Who Live When web application
- **User**: Any person accessing the application (no authentication required)
- **Streamer**: A content creator on the Kick platform whose live status is tracked
- **Programme**: A schedule view showing streamers and their predicted live times
- **Live Status**: Real-time information about whether a streamer is currently broadcasting
- **Navigation**: The process of moving between pages in the application via links or buttons
- **SPA-like behavior**: Single Page Application behavior where URL changes without full page reload

## Requirements

### Requirement 1

**User Story:** As a user, I want navigation links to work correctly, so that I can browse different pages of the application.

#### Acceptance Criteria

1. WHEN a user clicks the "Home" link in the navigation bar THEN the System SHALL perform a full page navigation to the home page
2. WHEN a user clicks the "Programme" link in the navigation bar THEN the System SHALL perform a full page navigation to the programme page
3. WHEN a user clicks any navigation link THEN the System SHALL load the corresponding page content without requiring a manual page refresh
4. WHEN the System renders navigation links THEN the System SHALL use standard HTML anchor tags with href attributes

### Requirement 2

**User Story:** As a user, I want to access all application features without logging in, so that I can use the service immediately.

#### Acceptance Criteria

1. WHEN the System starts THEN the System SHALL provide full functionality to all users without requiring authentication
2. WHEN the System renders the navigation bar THEN the System SHALL exclude the "Login with Google" button
3. WHEN a user accesses the home page THEN the System SHALL display streamer information without authentication checks
4. WHEN a user accesses the programme page THEN the System SHALL allow programme management without authentication
5. WHEN a user accesses the search functionality THEN the System SHALL return results without authentication requirements

### Requirement 3

**User Story:** As a user, I want to see actual streamer data from Kick, so that I can track real content creators.

#### Acceptance Criteria

1. WHEN the System initializes THEN the System SHALL seed the database with at least 5 popular Kick streamers
2. WHEN the System displays a streamer THEN the System SHALL show the streamer's name, handle, and platform information retrieved from Kick API
3. WHEN the System checks live status THEN the System SHALL query the Kick API for current streaming state
4. WHEN the Kick API returns live status THEN the System SHALL display viewer count, stream title, and stream URL
5. WHEN the Kick API is unavailable THEN the System SHALL display a clear "Status Unknown" message with explanation

### Requirement 4

**User Story:** As a user, I want a clean, professional dashboard interface, so that I can easily understand and use the application.

#### Acceptance Criteria

1. WHEN the System renders the home page THEN the System SHALL display exactly one navigation header at the top of the page
2. WHEN the System renders streamer cards THEN the System SHALL display them in a consistent grid layout without nested duplicate headers
3. WHEN the System displays live status THEN the System SHALL use clear visual indicators (green for live, gray for offline)
4. WHEN the System renders the page THEN the System SHALL maintain consistent spacing and typography throughout
5. WHEN the System displays platform information THEN the System SHALL show platform badges in a uniform style

### Requirement 5

**User Story:** As a user, I want to search for Kick streamers and add them to my view, so that I can track streamers I'm interested in.

#### Acceptance Criteria

1. WHEN a user submits a search query THEN the System SHALL query the Kick API for matching streamers
2. WHEN search results are returned THEN the System SHALL display streamer name, handle, and profile information
3. WHEN a user selects a streamer from search results THEN the System SHALL add that streamer to the local database
4. WHEN a streamer is added THEN the System SHALL immediately display that streamer on the home page
5. WHEN search returns no results THEN the System SHALL display a helpful message suggesting alternative searches

### Requirement 6

**User Story:** As a user, I want the streamer detail page to show comprehensive information, so that I can learn about streamers I'm interested in.

#### Acceptance Criteria

1. WHEN a user navigates to a streamer detail page THEN the System SHALL display the streamer's profile information from Kick
2. WHEN displaying a live streamer THEN the System SHALL show current stream title, viewer count, and a link to watch
3. WHEN displaying an offline streamer THEN the System SHALL show last known activity information if available
4. WHEN the System renders platform handles THEN the System SHALL display clickable links to the streamer's Kick channel
