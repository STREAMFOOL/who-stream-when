package sqlite

import (
	"context"
	"fmt"
	"time"

	"who-live-when/internal/domain"
)

// ActivityRecordRepository implements repository.ActivityRecordRepository for SQLite
type ActivityRecordRepository struct {
	db *DB
}

// NewActivityRecordRepository creates a new ActivityRecordRepository
func NewActivityRecordRepository(db *DB) *ActivityRecordRepository {
	return &ActivityRecordRepository{db: db}
}

// Create inserts a new activity record
func (r *ActivityRecordRepository) Create(ctx context.Context, record *domain.ActivityRecord) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO activity_records (id, streamer_id, start_time, end_time, platform, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		record.ID,
		record.StreamerID,
		record.StartTime,
		record.EndTime,
		record.Platform,
		record.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert activity record: %w", err)
	}
	return nil
}

// GetByStreamerID retrieves activity records for a streamer since a given time
func (r *ActivityRecordRepository) GetByStreamerID(ctx context.Context, streamerID string, since time.Time) ([]*domain.ActivityRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, streamer_id, start_time, end_time, platform, created_at
		FROM activity_records
		WHERE streamer_id = ? AND start_time >= ?
		ORDER BY start_time DESC
	`, streamerID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query activity records: %w", err)
	}
	defer rows.Close()

	var records []*domain.ActivityRecord
	for rows.Next() {
		var record domain.ActivityRecord
		if err := rows.Scan(
			&record.ID,
			&record.StreamerID,
			&record.StartTime,
			&record.EndTime,
			&record.Platform,
			&record.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan activity record: %w", err)
		}
		records = append(records, &record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating activity records: %w", err)
	}

	return records, nil
}

// GetAll retrieves all activity records since a given time
func (r *ActivityRecordRepository) GetAll(ctx context.Context, since time.Time) ([]*domain.ActivityRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, streamer_id, start_time, end_time, platform, created_at
		FROM activity_records
		WHERE start_time >= ?
		ORDER BY start_time DESC
	`, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query all activity records: %w", err)
	}
	defer rows.Close()

	var records []*domain.ActivityRecord
	for rows.Next() {
		var record domain.ActivityRecord
		if err := rows.Scan(
			&record.ID,
			&record.StreamerID,
			&record.StartTime,
			&record.EndTime,
			&record.Platform,
			&record.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan activity record: %w", err)
		}
		records = append(records, &record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating activity records: %w", err)
	}

	return records, nil
}

// Delete removes an activity record
func (r *ActivityRecordRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM activity_records WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete activity record: %w", err)
	}
	return nil
}
