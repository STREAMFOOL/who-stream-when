package sqlite

import (
	"context"
	"fmt"

	"who-live-when/internal/domain"
)

// FollowRepository implements repository.FollowRepository for SQLite
type FollowRepository struct {
	db *DB
}

// NewFollowRepository creates a new FollowRepository
func NewFollowRepository(db *DB) *FollowRepository {
	return &FollowRepository{db: db}
}

// Create creates a follow relationship
func (r *FollowRepository) Create(ctx context.Context, userID, streamerID string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO follows (user_id, streamer_id, created_at) VALUES (?, ?, ?)",
		userID,
		streamerID,
		timeNow(),
	)
	if err != nil {
		return fmt.Errorf("failed to create follow: %w", err)
	}
	return nil
}

// Delete removes a follow relationship
func (r *FollowRepository) Delete(ctx context.Context, userID, streamerID string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM follows WHERE user_id = ? AND streamer_id = ?",
		userID,
		streamerID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete follow: %w", err)
	}
	return nil
}

// GetFollowedStreamers retrieves all streamers followed by a user
func (r *FollowRepository) GetFollowedStreamers(ctx context.Context, userID string) ([]*domain.Streamer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.name, s.created_at, s.updated_at
		FROM streamers s
		INNER JOIN follows f ON s.id = f.streamer_id
		WHERE f.user_id = ?
		ORDER BY s.name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query followed streamers: %w", err)
	}
	defer rows.Close()

	var streamers []*domain.Streamer
	for rows.Next() {
		var s domain.Streamer
		if err := rows.Scan(&s.ID, &s.Name, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan streamer: %w", err)
		}

		// Load platform handles
		handles, platforms, err := r.loadPlatforms(ctx, s.ID)
		if err != nil {
			return nil, err
		}

		s.Handles = handles
		s.Platforms = platforms
		streamers = append(streamers, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating streamers: %w", err)
	}

	return streamers, nil
}

// IsFollowing checks if a user is following a streamer
func (r *FollowRepository) IsFollowing(ctx context.Context, userID, streamerID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM follows WHERE user_id = ? AND streamer_id = ?",
		userID,
		streamerID,
	).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("failed to check follow status: %w", err)
	}

	return count > 0, nil
}

// GetFollowerCount returns the number of followers for a streamer
func (r *FollowRepository) GetFollowerCount(ctx context.Context, streamerID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM follows WHERE streamer_id = ?",
		streamerID,
	).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("failed to get follower count: %w", err)
	}

	return count, nil
}

// loadPlatforms loads platform handles for a streamer
func (r *FollowRepository) loadPlatforms(ctx context.Context, streamerID string) (map[string]string, []string, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT platform, handle FROM streamer_platforms WHERE streamer_id = ?",
		streamerID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query platforms: %w", err)
	}
	defer rows.Close()

	handles := make(map[string]string)
	var platforms []string

	for rows.Next() {
		var platform, handle string
		if err := rows.Scan(&platform, &handle); err != nil {
			return nil, nil, fmt.Errorf("failed to scan platform: %w", err)
		}
		handles[platform] = handle
		platforms = append(platforms, platform)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("error iterating platforms: %w", err)
	}

	return handles, platforms, nil
}
