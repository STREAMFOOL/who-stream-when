// Package config manages application configuration loaded from environment variables.
//
// Configuration includes:
// - Database connection settings
// - OAuth credentials
// - Platform API keys
// - Server settings
// - Feature flags for platform enablement
//
// All configuration is loaded from environment variables with sensible defaults.
// Required variables will cause the application to fail fast with clear error messages.
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// FeatureFlags represents enabled platform features using bit flags.
// This allows efficient storage and checking of which platforms are enabled.
// Use IsEnabled, Enable, and Disable methods to interact with flags.
type FeatureFlags uint8

const (
	FeatureKick    FeatureFlags = 1 << iota // 0b001 - Kick platform
	FeatureYouTube                          // 0b010 - YouTube platform
	FeatureTwitch                           // 0b100 - Twitch platform
)

// IsEnabled checks if a specific platform feature is enabled
func (f FeatureFlags) IsEnabled(flag FeatureFlags) bool {
	return f&flag != 0
}

// Enable adds a feature flag
func (f *FeatureFlags) Enable(flag FeatureFlags) {
	*f |= flag
}

// Disable removes a feature flag
func (f *FeatureFlags) Disable(flag FeatureFlags) {
	*f &^= flag
}

// GetEnabledPlatforms returns a list of enabled platform names
func (f FeatureFlags) GetEnabledPlatforms() []string {
	platforms := []string{}
	if f.IsEnabled(FeatureKick) {
		platforms = append(platforms, "kick")
	}
	if f.IsEnabled(FeatureYouTube) {
		platforms = append(platforms, "youtube")
	}
	if f.IsEnabled(FeatureTwitch) {
		platforms = append(platforms, "twitch")
	}
	return platforms
}

// Config holds all application configuration loaded from environment variables.
// Configuration is validated on load to ensure all required values are present.
// Secrets are masked when logged to prevent accidental exposure.
type Config struct {
	// Database configuration
	// DatabasePath: Path to SQLite database file (default: ./data/who-live-when.db)
	DatabasePath string

	// OAuth configuration (required)
	// GoogleClientID: OAuth client ID from Google Cloud Console
	// GoogleClientSecret: OAuth client secret from Google Cloud Console
	// GoogleRedirectURL: Callback URL for OAuth flow (default: http://localhost:8080/auth/google/callback)
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// Platform API keys (optional - enables platform-specific features)
	// YouTubeAPIKey: YouTube Data API v3 key
	// TwitchClientID: Twitch application client ID
	// TwitchSecret: Twitch application secret
	// KickClientID: Kick application client ID
	// KickSecret: Kick application secret
	YouTubeAPIKey  string
	TwitchClientID string
	TwitchSecret   string
	KickClientID   string
	KickSecret     string

	// Server configuration
	// ServerPort: Port to listen on (default: 8080)
	// SessionSecret: Secret key for session encryption (default: "session")
	// SessionDuration: Session lifetime in seconds (default: 604800 = 7 days)
	ServerPort      string
	SessionSecret   string
	SessionDuration int

	// Feature flags control which platforms are enabled
	// Use FeatureFlags.IsEnabled() to check if a platform is available
	FeatureFlags FeatureFlags
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

	// Parse feature flags with default (Kick enabled, others disabled)
	cfg.FeatureFlags = parseFeatureFlags(getEnvOrDefault("FEATURE_FLAGS", "kick"))

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

	// Log feature flag status
	enabledPlatforms := c.FeatureFlags.GetEnabledPlatforms()
	log.Printf("Feature Flags - Enabled Platforms: %v", enabledPlatforms)
	log.Printf("  - Kick: %v", c.FeatureFlags.IsEnabled(FeatureKick))
	log.Printf("  - YouTube: %v", c.FeatureFlags.IsEnabled(FeatureYouTube))
	log.Printf("  - Twitch: %v", c.FeatureFlags.IsEnabled(FeatureTwitch))

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

// parseFeatureFlags parses a comma-separated list of platform names into FeatureFlags
// Default: "kick" (only Kick enabled)
// Example: "kick,youtube" or "kick,youtube,twitch"
func parseFeatureFlags(flagsStr string) FeatureFlags {
	var flags FeatureFlags

	if flagsStr == "" {
		return flags
	}

	platforms := strings.Split(strings.ToLower(flagsStr), ",")
	for _, platform := range platforms {
		platform = strings.TrimSpace(platform)
		switch platform {
		case "kick":
			flags.Enable(FeatureKick)
		case "youtube":
			flags.Enable(FeatureYouTube)
		case "twitch":
			flags.Enable(FeatureTwitch)
		}
	}

	return flags
}
