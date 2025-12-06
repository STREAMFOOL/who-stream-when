package domain

import (
	"context"
	"time"
)

// StreamerService manages streamer data and operations
type StreamerService interface {
	GetStreamer(ctx context.Context, id string) (*Streamer, error)
	ListStreamers(ctx context.Context, limit int) ([]*Streamer, error)
	SearchStreamers(ctx context.Context, query string) ([]*Streamer, error)
	AddStreamer(ctx context.Context, streamer *Streamer) error
	UpdateStreamer(ctx context.Context, streamer *Streamer) error
	GetStreamersByPlatform(ctx context.Context, platform string) ([]*Streamer, error)
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

// UserService manages user accounts and authentication state
type UserService interface {
	GetUser(ctx context.Context, userID string) (*User, error)
	CreateUser(ctx context.Context, googleID string, email string) (*User, error)
	GetUserFollows(ctx context.Context, userID string) ([]*Streamer, error)
	FollowStreamer(ctx context.Context, userID, streamerID string) error
	UnfollowStreamer(ctx context.Context, userID, streamerID string) error
}

// TVProgrammeService generates weekly predictions based on activity patterns
type TVProgrammeService interface {
	GenerateProgramme(ctx context.Context, userID string, week time.Time) (*TVProgramme, error)
	GetPredictedLiveTime(ctx context.Context, streamerID string, dayOfWeek int) (*PredictedTime, error)
	GetMostViewedStreamers(ctx context.Context, limit int) ([]*Streamer, error)
	GetDefaultWeekView(ctx context.Context) (*WeekView, error)
}
