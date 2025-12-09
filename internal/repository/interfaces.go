package repository

import (
	"context"
	"time"

	"who-live-when/internal/domain"
)

// StreamerRepository handles streamer data persistence
type StreamerRepository interface {
	Create(ctx context.Context, streamer *domain.Streamer) error
	GetByID(ctx context.Context, id string) (*domain.Streamer, error)
	GetByIDs(ctx context.Context, ids []string) ([]*domain.Streamer, error)
	List(ctx context.Context, limit int) ([]*domain.Streamer, error)
	Update(ctx context.Context, streamer *domain.Streamer) error
	Delete(ctx context.Context, id string) error
	GetByPlatform(ctx context.Context, platform string) ([]*domain.Streamer, error)
	GetByPlatformHandle(ctx context.Context, platform, handle string) (*domain.Streamer, error)
}

// LiveStatusRepository handles live status data persistence
type LiveStatusRepository interface {
	Create(ctx context.Context, status *domain.LiveStatus) error
	GetByStreamerID(ctx context.Context, streamerID string) (*domain.LiveStatus, error)
	Update(ctx context.Context, status *domain.LiveStatus) error
	GetAll(ctx context.Context) ([]*domain.LiveStatus, error)
	DeleteOlderThan(ctx context.Context, timestamp time.Time) error
}

// ActivityRecordRepository handles activity record persistence
type ActivityRecordRepository interface {
	Create(ctx context.Context, record *domain.ActivityRecord) error
	GetByStreamerID(ctx context.Context, streamerID string, since time.Time) ([]*domain.ActivityRecord, error)
	GetAll(ctx context.Context, since time.Time) ([]*domain.ActivityRecord, error)
	Delete(ctx context.Context, id string) error
}

// UserRepository handles user data persistence
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByGoogleID(ctx context.Context, googleID string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id string) error
}

// FollowRepository handles user-streamer follow relationships
type FollowRepository interface {
	Create(ctx context.Context, userID, streamerID string) error
	Delete(ctx context.Context, userID, streamerID string) error
	GetFollowedStreamers(ctx context.Context, userID string) ([]*domain.Streamer, error)
	IsFollowing(ctx context.Context, userID, streamerID string) (bool, error)
	GetFollowerCount(ctx context.Context, streamerID string) (int, error)
}

// HeatmapRepository handles heatmap data persistence
type HeatmapRepository interface {
	Create(ctx context.Context, heatmap *domain.Heatmap) error
	GetByStreamerID(ctx context.Context, streamerID string) (*domain.Heatmap, error)
	Update(ctx context.Context, heatmap *domain.Heatmap) error
	Delete(ctx context.Context, streamerID string) error
}

// CustomProgrammeRepository handles custom programme data persistence
type CustomProgrammeRepository interface {
	Create(ctx context.Context, programme *domain.CustomProgramme) error
	GetByUserID(ctx context.Context, userID string) (*domain.CustomProgramme, error)
	Update(ctx context.Context, programme *domain.CustomProgramme) error
	Delete(ctx context.Context, userID string) error
}
