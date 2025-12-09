package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"who-live-when/internal/domain"
)

// StreamerRepository implements repository.StreamerRepository for SQLite
type StreamerRepository struct {
	db *DB
}

// NewStreamerRepository creates a new StreamerRepository
func NewStreamerRepository(db *DB) *StreamerRepository {
	return &StreamerRepository{db: db}
}

// Create inserts a new streamer into the database
func (r *StreamerRepository) Create(ctx context.Context, streamer *domain.Streamer) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert streamer
	_, err = tx.ExecContext(ctx,
		"INSERT INTO streamers (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)",
		streamer.ID,
		streamer.Name,
		streamer.CreatedAt,
		streamer.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert streamer: %w", err)
	}

	// Insert platform handles
	for platform, handle := range streamer.Handles {
		_, err = tx.ExecContext(ctx,
			"INSERT INTO streamer_platforms (streamer_id, platform, handle) VALUES (?, ?, ?)",
			streamer.ID,
			platform,
			handle,
		)
		if err != nil {
			return fmt.Errorf("failed to insert platform handle: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetByID retrieves a streamer by ID
func (r *StreamerRepository) GetByID(ctx context.Context, id string) (*domain.Streamer, error) {
	var streamer domain.Streamer
	err := r.db.QueryRowContext(ctx,
		"SELECT id, name, created_at, updated_at FROM streamers WHERE id = ?",
		id,
	).Scan(&streamer.ID, &streamer.Name, &streamer.CreatedAt, &streamer.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("streamer not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query streamer: %w", err)
	}

	// Load platform handles
	handles, platforms, err := r.loadPlatforms(ctx, id)
	if err != nil {
		return nil, err
	}

	streamer.Handles = handles
	streamer.Platforms = platforms

	return &streamer, nil
}

// List retrieves a list of streamers with a limit
func (r *StreamerRepository) List(ctx context.Context, limit int) ([]*domain.Streamer, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, name, created_at, updated_at FROM streamers ORDER BY created_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query streamers: %w", err)
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

// Update updates an existing streamer
func (r *StreamerRepository) Update(ctx context.Context, streamer *domain.Streamer) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update streamer
	_, err = tx.ExecContext(ctx,
		"UPDATE streamers SET name = ?, updated_at = ? WHERE id = ?",
		streamer.Name,
		streamer.UpdatedAt,
		streamer.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update streamer: %w", err)
	}

	// Delete existing platform handles
	_, err = tx.ExecContext(ctx,
		"DELETE FROM streamer_platforms WHERE streamer_id = ?",
		streamer.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete platform handles: %w", err)
	}

	// Insert new platform handles
	for platform, handle := range streamer.Handles {
		_, err = tx.ExecContext(ctx,
			"INSERT INTO streamer_platforms (streamer_id, platform, handle) VALUES (?, ?, ?)",
			streamer.ID,
			platform,
			handle,
		)
		if err != nil {
			return fmt.Errorf("failed to insert platform handle: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Delete removes a streamer from the database
func (r *StreamerRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM streamers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete streamer: %w", err)
	}
	return nil
}

// GetByIDs retrieves streamers by a list of IDs
func (r *StreamerRepository) GetByIDs(ctx context.Context, ids []string) ([]*domain.Streamer, error) {
	if len(ids) == 0 {
		return []*domain.Streamer{}, nil
	}

	// Build placeholders for IN clause
	placeholders := ""
	args := make([]any, len(ids))
	for i, id := range ids {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		"SELECT id, name, created_at, updated_at FROM streamers WHERE id IN (%s) ORDER BY name",
		placeholders,
	)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query streamers by IDs: %w", err)
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

// GetByPlatform retrieves streamers by platform
func (r *StreamerRepository) GetByPlatform(ctx context.Context, platform string) ([]*domain.Streamer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT s.id, s.name, s.created_at, s.updated_at
		FROM streamers s
		INNER JOIN streamer_platforms sp ON s.id = sp.streamer_id
		WHERE sp.platform = ?
		ORDER BY s.created_at DESC
	`, platform)
	if err != nil {
		return nil, fmt.Errorf("failed to query streamers by platform: %w", err)
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

// loadPlatforms loads platform handles for a streamer
func (r *StreamerRepository) loadPlatforms(ctx context.Context, streamerID string) (map[string]string, []string, error) {
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
