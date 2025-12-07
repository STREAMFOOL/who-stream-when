package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/user/who-live-when/internal/domain"
)

// HeatmapRepository implements repository.HeatmapRepository for SQLite
type HeatmapRepository struct {
	db *DB
}

// NewHeatmapRepository creates a new HeatmapRepository
func NewHeatmapRepository(db *DB) *HeatmapRepository {
	return &HeatmapRepository{db: db}
}

// Create inserts a new heatmap
func (r *HeatmapRepository) Create(ctx context.Context, heatmap *domain.Heatmap) error {
	hoursJSON, err := json.Marshal(heatmap.Hours)
	if err != nil {
		return fmt.Errorf("failed to marshal hours: %w", err)
	}

	daysJSON, err := json.Marshal(heatmap.DaysOfWeek)
	if err != nil {
		return fmt.Errorf("failed to marshal days: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO heatmaps (streamer_id, hours, days_of_week, data_points, generated_at)
		VALUES (?, ?, ?, ?, ?)
	`,
		heatmap.StreamerID,
		string(hoursJSON),
		string(daysJSON),
		heatmap.DataPoints,
		heatmap.GeneratedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert heatmap: %w", err)
	}
	return nil
}

// GetByStreamerID retrieves a heatmap for a streamer
func (r *HeatmapRepository) GetByStreamerID(ctx context.Context, streamerID string) (*domain.Heatmap, error) {
	var heatmap domain.Heatmap
	var hoursJSON, daysJSON string

	err := r.db.QueryRowContext(ctx, `
		SELECT streamer_id, hours, days_of_week, data_points, generated_at
		FROM heatmaps
		WHERE streamer_id = ?
	`, streamerID).Scan(
		&heatmap.StreamerID,
		&hoursJSON,
		&daysJSON,
		&heatmap.DataPoints,
		&heatmap.GeneratedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("heatmap not found for streamer: %s", streamerID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query heatmap: %w", err)
	}

	if err := json.Unmarshal([]byte(hoursJSON), &heatmap.Hours); err != nil {
		return nil, fmt.Errorf("failed to unmarshal hours: %w", err)
	}

	if err := json.Unmarshal([]byte(daysJSON), &heatmap.DaysOfWeek); err != nil {
		return nil, fmt.Errorf("failed to unmarshal days: %w", err)
	}

	return &heatmap, nil
}

// Update updates an existing heatmap
func (r *HeatmapRepository) Update(ctx context.Context, heatmap *domain.Heatmap) error {
	hoursJSON, err := json.Marshal(heatmap.Hours)
	if err != nil {
		return fmt.Errorf("failed to marshal hours: %w", err)
	}

	daysJSON, err := json.Marshal(heatmap.DaysOfWeek)
	if err != nil {
		return fmt.Errorf("failed to marshal days: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		UPDATE heatmaps
		SET hours = ?, days_of_week = ?, data_points = ?, generated_at = ?
		WHERE streamer_id = ?
	`,
		string(hoursJSON),
		string(daysJSON),
		heatmap.DataPoints,
		heatmap.GeneratedAt,
		heatmap.StreamerID,
	)
	if err != nil {
		return fmt.Errorf("failed to update heatmap: %w", err)
	}
	return nil
}

// Delete removes a heatmap
func (r *HeatmapRepository) Delete(ctx context.Context, streamerID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM heatmaps WHERE streamer_id = ?", streamerID)
	if err != nil {
		return fmt.Errorf("failed to delete heatmap: %w", err)
	}
	return nil
}
