package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"who-live-when/internal/domain"
)

// CustomProgrammeRepository implements repository.CustomProgrammeRepository for SQLite
type CustomProgrammeRepository struct {
	db *DB
}

// NewCustomProgrammeRepository creates a new CustomProgrammeRepository
func NewCustomProgrammeRepository(db *DB) *CustomProgrammeRepository {
	return &CustomProgrammeRepository{db: db}
}

// Create inserts a new custom programme into the database
func (r *CustomProgrammeRepository) Create(ctx context.Context, programme *domain.CustomProgramme) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert custom programme
	_, err = tx.ExecContext(ctx,
		"INSERT INTO custom_programmes (id, user_id, created_at, updated_at) VALUES (?, ?, ?, ?)",
		programme.ID,
		programme.UserID,
		programme.CreatedAt,
		programme.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert custom programme: %w", err)
	}

	// Insert streamer associations
	for position, streamerID := range programme.StreamerIDs {
		_, err = tx.ExecContext(ctx,
			"INSERT INTO custom_programme_streamers (programme_id, streamer_id, position) VALUES (?, ?, ?)",
			programme.ID,
			streamerID,
			position,
		)
		if err != nil {
			return fmt.Errorf("failed to insert programme streamer: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetByUserID retrieves a custom programme by user ID
func (r *CustomProgrammeRepository) GetByUserID(ctx context.Context, userID string) (*domain.CustomProgramme, error) {
	var programme domain.CustomProgramme
	err := r.db.QueryRowContext(ctx,
		"SELECT id, user_id, created_at, updated_at FROM custom_programmes WHERE user_id = ?",
		userID,
	).Scan(&programme.ID, &programme.UserID, &programme.CreatedAt, &programme.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("custom programme not found for user: %s", userID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query custom programme: %w", err)
	}

	// Load streamer IDs
	streamerIDs, err := r.loadStreamerIDs(ctx, programme.ID)
	if err != nil {
		return nil, err
	}

	programme.StreamerIDs = streamerIDs

	return &programme, nil
}

// Update updates an existing custom programme
func (r *CustomProgrammeRepository) Update(ctx context.Context, programme *domain.CustomProgramme) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update custom programme
	_, err = tx.ExecContext(ctx,
		"UPDATE custom_programmes SET updated_at = ? WHERE id = ?",
		programme.UpdatedAt,
		programme.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update custom programme: %w", err)
	}

	// Delete existing streamer associations
	_, err = tx.ExecContext(ctx,
		"DELETE FROM custom_programme_streamers WHERE programme_id = ?",
		programme.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete programme streamers: %w", err)
	}

	// Insert new streamer associations
	for position, streamerID := range programme.StreamerIDs {
		_, err = tx.ExecContext(ctx,
			"INSERT INTO custom_programme_streamers (programme_id, streamer_id, position) VALUES (?, ?, ?)",
			programme.ID,
			streamerID,
			position,
		)
		if err != nil {
			return fmt.Errorf("failed to insert programme streamer: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Delete removes a custom programme from the database
func (r *CustomProgrammeRepository) Delete(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM custom_programmes WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("failed to delete custom programme: %w", err)
	}
	return nil
}

// loadStreamerIDs loads streamer IDs for a custom programme in order
func (r *CustomProgrammeRepository) loadStreamerIDs(ctx context.Context, programmeID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT streamer_id FROM custom_programme_streamers WHERE programme_id = ? ORDER BY position",
		programmeID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query programme streamers: %w", err)
	}
	defer rows.Close()

	var streamerIDs []string
	for rows.Next() {
		var streamerID string
		if err := rows.Scan(&streamerID); err != nil {
			return nil, fmt.Errorf("failed to scan streamer ID: %w", err)
		}
		streamerIDs = append(streamerIDs, streamerID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating programme streamers: %w", err)
	}

	return streamerIDs, nil
}
