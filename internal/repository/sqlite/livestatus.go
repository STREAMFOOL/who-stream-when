package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/user/who-live-when/internal/domain"
)

// LiveStatusRepository implements repository.LiveStatusRepository for SQLite
type LiveStatusRepository struct {
	db *DB
}

// NewLiveStatusRepository creates a new LiveStatusRepository
func NewLiveStatusRepository(db *DB) *LiveStatusRepository {
	return &LiveStatusRepository{db: db}
}

// Create inserts a new live status record
func (r *LiveStatusRepository) Create(ctx context.Context, status *domain.LiveStatus) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO live_status (streamer_id, is_live, platform, stream_url, title, thumbnail, viewer_count, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		status.StreamerID,
		status.IsLive,
		status.Platform,
		nullString(status.StreamURL),
		nullString(status.Title),
		nullString(status.Thumbnail),
		status.ViewerCount,
		status.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert live status: %w", err)
	}
	return nil
}

// GetByStreamerID retrieves live status for a streamer
func (r *LiveStatusRepository) GetByStreamerID(ctx context.Context, streamerID string) (*domain.LiveStatus, error) {
	var status domain.LiveStatus
	var streamURL, title, thumbnail sql.NullString

	err := r.db.QueryRowContext(ctx, `
		SELECT streamer_id, is_live, platform, stream_url, title, thumbnail, viewer_count, updated_at
		FROM live_status
		WHERE streamer_id = ?
	`, streamerID).Scan(
		&status.StreamerID,
		&status.IsLive,
		&status.Platform,
		&streamURL,
		&title,
		&thumbnail,
		&status.ViewerCount,
		&status.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("live status not found for streamer: %s", streamerID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query live status: %w", err)
	}

	status.StreamURL = streamURL.String
	status.Title = title.String
	status.Thumbnail = thumbnail.String

	return &status, nil
}

// Update updates an existing live status record
func (r *LiveStatusRepository) Update(ctx context.Context, status *domain.LiveStatus) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE live_status
		SET is_live = ?, platform = ?, stream_url = ?, title = ?, thumbnail = ?, viewer_count = ?, updated_at = ?
		WHERE streamer_id = ?
	`,
		status.IsLive,
		status.Platform,
		nullString(status.StreamURL),
		nullString(status.Title),
		nullString(status.Thumbnail),
		status.ViewerCount,
		status.UpdatedAt,
		status.StreamerID,
	)
	if err != nil {
		return fmt.Errorf("failed to update live status: %w", err)
	}
	return nil
}

// GetAll retrieves all live status records
func (r *LiveStatusRepository) GetAll(ctx context.Context) ([]*domain.LiveStatus, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT streamer_id, is_live, platform, stream_url, title, thumbnail, viewer_count, updated_at
		FROM live_status
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query all live status: %w", err)
	}
	defer rows.Close()

	var statuses []*domain.LiveStatus
	for rows.Next() {
		var status domain.LiveStatus
		var streamURL, title, thumbnail sql.NullString

		if err := rows.Scan(
			&status.StreamerID,
			&status.IsLive,
			&status.Platform,
			&streamURL,
			&title,
			&thumbnail,
			&status.ViewerCount,
			&status.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan live status: %w", err)
		}

		status.StreamURL = streamURL.String
		status.Title = title.String
		status.Thumbnail = thumbnail.String

		statuses = append(statuses, &status)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating live status: %w", err)
	}

	return statuses, nil
}

// DeleteOlderThan deletes live status records older than the specified timestamp
func (r *LiveStatusRepository) DeleteOlderThan(ctx context.Context, timestamp time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM live_status WHERE updated_at < ?",
		timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to delete old live status: %w", err)
	}
	return nil
}

// nullString converts a string to sql.NullString
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
