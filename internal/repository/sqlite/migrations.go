package sqlite

import (
	"database/sql"
	"fmt"
)

// Migration represents a database migration
type Migration struct {
	Version int
	Name    string
	Up      string
}

// migrations contains all database migrations in order
var migrations = []Migration{
	{
		Version: 1,
		Name:    "initial_schema",
		Up: `
			CREATE TABLE IF NOT EXISTS streamers (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			);

			CREATE TABLE IF NOT EXISTS streamer_platforms (
				streamer_id TEXT NOT NULL,
				platform TEXT NOT NULL,
				handle TEXT NOT NULL,
				PRIMARY KEY (streamer_id, platform),
				FOREIGN KEY (streamer_id) REFERENCES streamers(id) ON DELETE CASCADE
			);

			CREATE INDEX IF NOT EXISTS idx_streamer_platforms_platform ON streamer_platforms(platform);

			CREATE TABLE IF NOT EXISTS users (
				id TEXT PRIMARY KEY,
				google_id TEXT UNIQUE NOT NULL,
				email TEXT NOT NULL,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			);

			CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id);

			CREATE TABLE IF NOT EXISTS follows (
				user_id TEXT NOT NULL,
				streamer_id TEXT NOT NULL,
				created_at DATETIME NOT NULL,
				PRIMARY KEY (user_id, streamer_id),
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
				FOREIGN KEY (streamer_id) REFERENCES streamers(id) ON DELETE CASCADE
			);

			CREATE INDEX IF NOT EXISTS idx_follows_user_id ON follows(user_id);
			CREATE INDEX IF NOT EXISTS idx_follows_streamer_id ON follows(streamer_id);

			CREATE TABLE IF NOT EXISTS live_status (
				streamer_id TEXT PRIMARY KEY,
				is_live BOOLEAN NOT NULL,
				platform TEXT NOT NULL,
				stream_url TEXT,
				title TEXT,
				thumbnail TEXT,
				viewer_count INTEGER DEFAULT 0,
				updated_at DATETIME NOT NULL,
				FOREIGN KEY (streamer_id) REFERENCES streamers(id) ON DELETE CASCADE
			);

			CREATE TABLE IF NOT EXISTS activity_records (
				id TEXT PRIMARY KEY,
				streamer_id TEXT NOT NULL,
				start_time DATETIME NOT NULL,
				end_time DATETIME NOT NULL,
				platform TEXT NOT NULL,
				created_at DATETIME NOT NULL,
				FOREIGN KEY (streamer_id) REFERENCES streamers(id) ON DELETE CASCADE
			);

			CREATE INDEX IF NOT EXISTS idx_activity_records_streamer_id ON activity_records(streamer_id);
			CREATE INDEX IF NOT EXISTS idx_activity_records_start_time ON activity_records(start_time);

			CREATE TABLE IF NOT EXISTS heatmaps (
				streamer_id TEXT PRIMARY KEY,
				hours TEXT NOT NULL,
				days_of_week TEXT NOT NULL,
				data_points INTEGER NOT NULL,
				generated_at DATETIME NOT NULL,
				FOREIGN KEY (streamer_id) REFERENCES streamers(id) ON DELETE CASCADE
			);
		`,
	},
}

// Migrate runs all pending migrations
func Migrate(db *sql.DB) error {
	// Get current version
	currentVersion, err := getCurrentVersion(db)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	// Run pending migrations
	for _, migration := range migrations {
		if migration.Version <= currentVersion {
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Execute migration
		if _, err := tx.Exec(migration.Up); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %d (%s): %w", migration.Version, migration.Name, err)
		}

		// Record migration
		if _, err := tx.Exec(
			"INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)",
			migration.Version,
			migration.Name,
			sql.NullTime{Time: timeNow(), Valid: true},
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
		}
	}

	return nil
}

// getCurrentVersion returns the current schema version
func getCurrentVersion(db *sql.DB) (int, error) {
	// First, ensure the schema_migrations table exists
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME NOT NULL
		)
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	var version int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		// If table doesn't exist, version is 0
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to query version: %w", err)
	}
	return version, nil
}
