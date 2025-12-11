package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

// Config holds all application configuration loaded from environment variables
type Config struct {
	// Database configuration
	DatabasePath string

	// OAuth configuration
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// Platform API keys
	YouTubeAPIKey  string
	TwitchClientID string
	TwitchSecret   string
	KickClientID   string
	KickSecret     string

	// Server configuration
	ServerPort      string
	SessionSecret   string
	SessionDuration int
}

// Load reads configuration from environment variables and returns a Config instance
func Load() (*Config, error) {
	cfg := &Config{
		// Database configuration
		DatabasePath: getEnvOrDefault("DATABASE_PATH", "./data/who-live-when.db"),

		// OAuth configuration (required)
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  getEnvOrDefault("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/google/callback"),

		// Platform API keys (required)
		KickClientID: os.Getenv("KICK_CLIENT_ID"),
		KickSecret:   os.Getenv("KICK_CLIENT_SECRET"),

		// Platform API keys (optional - will log warnings if missing)
		YouTubeAPIKey:  os.Getenv("YOUTUBE_API_KEY"),
		TwitchClientID: os.Getenv("TWITCH_CLIENT_ID"),
		TwitchSecret:   os.Getenv("TWITCH_SECRET"),

		// Server configuration
		ServerPort:    getEnvOrDefault("SERVER_PORT", "8080"),
		SessionSecret: getEnvOrDefault("SESSION_SECRET", "session"),
	}

	// Parse session duration with default
	sessionDuration, err := strconv.Atoi(getEnvOrDefault("SESSION_DURATION", "604800"))
	if err != nil {
		return nil, fmt.Errorf("invalid SESSION_DURATION format: %w", err)
	}
	cfg.SessionDuration = sessionDuration

	// Validate required configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that all required configuration values are present and valid
func (c *Config) Validate() error {
	// Check required OAuth credentials
	if c.GoogleClientID == "" {
		return fmt.Errorf("GOOGLE_CLIENT_ID environment variable is required")
	}
	if c.GoogleClientSecret == "" {
		return fmt.Errorf("GOOGLE_CLIENT_SECRET environment variable is required")
	}

	// Validate database path is not empty
	if c.DatabasePath == "" {
		return fmt.Errorf("DATABASE_PATH cannot be empty")
	}

	// Validate server port format
	if c.ServerPort == "" {
		return fmt.Errorf("SERVER_PORT cannot be empty")
	}

	// Validate session duration is positive
	if c.SessionDuration <= 0 {
		return fmt.Errorf("SESSION_DURATION must be positive, got %d", c.SessionDuration)
	}

	return nil
}

// LogConfiguration logs all loaded configuration values, excluding secrets
func (c *Config) LogConfiguration() {
	log.Println("=== Application Configuration ===")
	log.Printf("Database Path: %s", c.DatabasePath)
	log.Printf("Google Client ID: %s", maskSecret(c.GoogleClientID))
	log.Printf("Google Redirect URL: %s", c.GoogleRedirectURL)
	log.Printf("YouTube API Key: %s", maskSecret(c.YouTubeAPIKey))
	log.Printf("Twitch Client ID: %s", maskSecret(c.TwitchClientID))
	log.Printf("Kick Client ID: %s", maskSecret(c.KickClientID))
	log.Printf("Server Port: %s", c.ServerPort)
	log.Printf("Session Duration: %d seconds", c.SessionDuration)

	// Log warnings for missing optional API keys
	if c.YouTubeAPIKey == "" {
		log.Println("WARNING: YOUTUBE_API_KEY not set - YouTube search will have limited functionality")
	}
	if c.TwitchClientID == "" || c.TwitchSecret == "" {
		log.Println("WARNING: TWITCH_CLIENT_ID or TWITCH_SECRET not set - Twitch search will have limited functionality")
	}
	if c.KickClientID == "" || c.KickSecret == "" {
		log.Println("WARNING: KICK_CLIENT_ID or KICK_CLIENT_SECRET not set - Kick API will have limited functionality")
	}

	log.Println("=================================")
}

// getEnvOrDefault returns the environment variable value or a default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// maskSecret masks a secret string for logging, showing only first 4 characters
func maskSecret(secret string) string {
	if secret == "" {
		return "[not set]"
	}
	if len(secret) <= 4 {
		return "****"
	}
	return secret[:4] + "****"
}
