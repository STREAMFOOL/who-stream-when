package domain

import (
	"context"
	"time"
)

// StreamerService manages streamer data and operations.
// Handles streamer retrieval, search, creation, and platform-specific queries.
// Supports idempotent streamer creation from search results.
type StreamerService interface {
	// GetStreamer retrieves a streamer by ID
	GetStreamer(ctx context.Context, id string) (*Streamer, error)

	// ListStreamers retrieves a limited list of streamers
	ListStreamers(ctx context.Context, limit int) ([]*Streamer, error)

	// SearchStreamers searches for streamers by name or handle
	SearchStreamers(ctx context.Context, query string) ([]*Streamer, error)

	// AddStreamer creates a new streamer record
	AddStreamer(ctx context.Context, streamer *Streamer) error

	// UpdateStreamer updates an existing streamer record
	UpdateStreamer(ctx context.Context, streamer *Streamer) error

	// GetStreamersByPlatform retrieves all streamers available on a specific platform
	GetStreamersByPlatform(ctx context.Context, platform string) ([]*Streamer, error)

	// GetOrCreateStreamer retrieves an existing streamer or creates one if not found
	// Idempotent: calling multiple times with same platform/handle returns same streamer
	// Used when adding streamers from search results
	GetOrCreateStreamer(ctx context.Context, platform, handle, name string) (*Streamer, error)
}

// LiveStatusService queries and caches live status across platforms
type LiveStatusService interface {
	GetLiveStatus(ctx context.Context, streamerID string) (*LiveStatus, error)
	RefreshLiveStatus(ctx context.Context, streamerID string) (*LiveStatus, error)
	GetAllLiveStatus(ctx context.Context) (map[string]*LiveStatus, error)
}

// HeatmapService generates activity patterns from historical data
type HeatmapService interface {
	GenerateHeatmap(ctx context.Context, streamerID string) (*Heatmap, error)
	RecordActivity(ctx context.Context, streamerID string, timestamp time.Time) error
	GetActivityStats(ctx context.Context, streamerID string) (*ActivityStats, error)
}

// PlatformAdapter abstracts platform-specific API interactions
type PlatformAdapter interface {
	GetLiveStatus(ctx context.Context, handle string) (*PlatformLiveStatus, error)
	SearchStreamer(ctx context.Context, query string) ([]*PlatformStreamer, error)
	GetChannelInfo(ctx context.Context, handle string) (*PlatformChannelInfo, error)
}

// UserService manages user accounts and authentication state.
// Supports both registered users (database storage) and guest users (session storage).
// Handles follow operations, guest data migration on registration, and user retrieval.
type UserService interface {
	// GetUser retrieves a registered user by ID
	GetUser(ctx context.Context, userID string) (*User, error)

	// CreateUser creates a new registered user from OAuth credentials
	CreateUser(ctx context.Context, googleID string, email string) (*User, error)

	// GetUserFollows retrieves all streamers followed by a registered user
	GetUserFollows(ctx context.Context, userID string) ([]*Streamer, error)

	// GetStreamersByIDs retrieves multiple streamers by their IDs
	// Used for both registered and guest user follow lists
	GetStreamersByIDs(ctx context.Context, streamerIDs []string) ([]*Streamer, error)

	// FollowStreamer adds a streamer to a registered user's follow list
	// Persists to database
	FollowStreamer(ctx context.Context, userID, streamerID string) error

	// UnfollowStreamer removes a streamer from a registered user's follow list
	UnfollowStreamer(ctx context.Context, userID, streamerID string) error

	// MigrateGuestData migrates session-based guest data to database storage
	// Called when a guest user registers or logs in
	// Migrates both follows and custom programme data
	// Uses transactions to ensure all-or-nothing semantics
	MigrateGuestData(ctx context.Context, userID string, guestFollows []string, guestProgramme *CustomProgramme) error
}

// TVProgrammeService generates weekly predictions based on activity patterns
type TVProgrammeService interface {
	GenerateProgramme(ctx context.Context, userID string, week time.Time) (*TVProgramme, error)
	GetPredictedLiveTime(ctx context.Context, streamerID string, dayOfWeek int) (*PredictedTime, error)
	GetMostViewedStreamers(ctx context.Context, limit int) ([]*Streamer, error)
	GetDefaultWeekView(ctx context.Context) (*WeekView, error)
}

// ProgrammeService manages custom programmes for both registered and guest users.
// Supports CRUD operations for custom programmes and generation of calendar views.
// Guest programmes are session-based, registered programmes are database-persisted.
type ProgrammeService interface {
	// CreateCustomProgramme creates a new custom programme for a registered user
	// Replaces any existing custom programme
	CreateCustomProgramme(ctx context.Context, userID string, streamerIDs []string) (*CustomProgramme, error)

	// GetCustomProgramme retrieves a registered user's custom programme
	// Returns nil if no custom programme exists (user should use global programme)
	GetCustomProgramme(ctx context.Context, userID string) (*CustomProgramme, error)

	// UpdateCustomProgramme updates an existing custom programme with new streamer selections
	UpdateCustomProgramme(ctx context.Context, userID string, streamerIDs []string) error

	// DeleteCustomProgramme removes a custom programme, reverting user to global programme
	DeleteCustomProgramme(ctx context.Context, userID string) error

	// GenerateCalendarFromProgramme creates a calendar view from a custom programme
	// Filters global calendar to only include selected streamers
	GenerateCalendarFromProgramme(ctx context.Context, programme *CustomProgramme, week time.Time) (*WeekView, error)

	// GenerateGlobalProgramme creates the default global calendar
	// Shows most viewed streamers ranked by follower count
	GenerateGlobalProgramme(ctx context.Context, week time.Time) (*WeekView, error)
}
