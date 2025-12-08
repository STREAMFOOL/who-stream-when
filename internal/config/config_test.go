package config

import (
	"os"
	"testing"
)

func TestLoad_ValidConfiguration(t *testing.T) {
	// Set up valid environment variables
	os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test-client-secret")
	os.Setenv("GOOGLE_REDIRECT_URL", "http://localhost:8080/callback")
	os.Setenv("YOUTUBE_API_KEY", "test-youtube-key")
	os.Setenv("TWITCH_CLIENT_ID", "test-twitch-id")
	os.Setenv("TWITCH_SECRET", "test-twitch-secret")
	os.Setenv("DATABASE_PATH", "./test.db")
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("SESSION_SECRET", "test-secret")
	os.Setenv("SESSION_DURATION", "3600")
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed with valid config: %v", err)
	}

	if cfg.GoogleClientID != "test-client-id" {
		t.Errorf("GoogleClientID = %s, want test-client-id", cfg.GoogleClientID)
	}
	if cfg.GoogleClientSecret != "test-client-secret" {
		t.Errorf("GoogleClientSecret = %s, want test-client-secret", cfg.GoogleClientSecret)
	}
	if cfg.GoogleRedirectURL != "http://localhost:8080/callback" {
		t.Errorf("GoogleRedirectURL = %s, want http://localhost:8080/callback", cfg.GoogleRedirectURL)
	}
	if cfg.YouTubeAPIKey != "test-youtube-key" {
		t.Errorf("YouTubeAPIKey = %s, want test-youtube-key", cfg.YouTubeAPIKey)
	}
	if cfg.TwitchClientID != "test-twitch-id" {
		t.Errorf("TwitchClientID = %s, want test-twitch-id", cfg.TwitchClientID)
	}
	if cfg.TwitchSecret != "test-twitch-secret" {
		t.Errorf("TwitchSecret = %s, want test-twitch-secret", cfg.TwitchSecret)
	}
	if cfg.DatabasePath != "./test.db" {
		t.Errorf("DatabasePath = %s, want ./test.db", cfg.DatabasePath)
	}
	if cfg.ServerPort != "9090" {
		t.Errorf("ServerPort = %s, want 9090", cfg.ServerPort)
	}
	if cfg.SessionSecret != "test-secret" {
		t.Errorf("SessionSecret = %s, want test-secret", cfg.SessionSecret)
	}
	if cfg.SessionDuration != 3600 {
		t.Errorf("SessionDuration = %d, want 3600", cfg.SessionDuration)
	}
}

func TestLoad_MissingGoogleClientID(t *testing.T) {
	clearEnv()
	os.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should fail when GOOGLE_CLIENT_ID is missing")
	}
	if err.Error() != "GOOGLE_CLIENT_ID environment variable is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoad_MissingGoogleClientSecret(t *testing.T) {
	clearEnv()
	os.Setenv("GOOGLE_CLIENT_ID", "test-id")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should fail when GOOGLE_CLIENT_SECRET is missing")
	}
	if err.Error() != "GOOGLE_CLIENT_SECRET environment variable is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoad_InvalidSessionDuration(t *testing.T) {
	os.Setenv("GOOGLE_CLIENT_ID", "test-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")
	os.Setenv("SESSION_DURATION", "invalid")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should fail when SESSION_DURATION is invalid")
	}
}

func TestLoad_DefaultValues(t *testing.T) {
	os.Setenv("GOOGLE_CLIENT_ID", "test-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.DatabasePath != "./data/who-live-when.db" {
		t.Errorf("DatabasePath = %s, want ./data/who-live-when.db", cfg.DatabasePath)
	}
	if cfg.GoogleRedirectURL != "http://localhost:8080/auth/callback" {
		t.Errorf("GoogleRedirectURL = %s, want http://localhost:8080/auth/callback", cfg.GoogleRedirectURL)
	}
	if cfg.ServerPort != "8080" {
		t.Errorf("ServerPort = %s, want 8080", cfg.ServerPort)
	}
	if cfg.SessionSecret != "session" {
		t.Errorf("SessionSecret = %s, want session", cfg.SessionSecret)
	}
	if cfg.SessionDuration != 604800 {
		t.Errorf("SessionDuration = %d, want 604800", cfg.SessionDuration)
	}
}

func TestValidate_EmptyDatabasePath(t *testing.T) {
	cfg := &Config{
		GoogleClientID:     "test-id",
		GoogleClientSecret: "test-secret",
		DatabasePath:       "",
		ServerPort:         "8080",
		SessionDuration:    3600,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should fail when DatabasePath is empty")
	}
}

func TestValidate_EmptyServerPort(t *testing.T) {
	cfg := &Config{
		GoogleClientID:     "test-id",
		GoogleClientSecret: "test-secret",
		DatabasePath:       "./test.db",
		ServerPort:         "",
		SessionDuration:    3600,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should fail when ServerPort is empty")
	}
}

func TestValidate_NegativeSessionDuration(t *testing.T) {
	cfg := &Config{
		GoogleClientID:     "test-id",
		GoogleClientSecret: "test-secret",
		DatabasePath:       "./test.db",
		ServerPort:         "8080",
		SessionDuration:    -1,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should fail when SessionDuration is negative")
	}
}

func TestValidate_ZeroSessionDuration(t *testing.T) {
	cfg := &Config{
		GoogleClientID:     "test-id",
		GoogleClientSecret: "test-secret",
		DatabasePath:       "./test.db",
		ServerPort:         "8080",
		SessionDuration:    0,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should fail when SessionDuration is zero")
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		want   string
	}{
		{"empty string", "", "[not set]"},
		{"short secret", "abc", "****"},
		{"normal secret", "abcdefgh", "abcd****"},
		{"long secret", "very-long-secret-key-12345", "very****"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskSecret(tt.secret)
			if got != tt.want {
				t.Errorf("maskSecret(%q) = %q, want %q", tt.secret, got, tt.want)
			}
		})
	}
}

func TestLogConfiguration(t *testing.T) {
	cfg := &Config{
		DatabasePath:       "./test.db",
		GoogleClientID:     "test-client-id",
		GoogleClientSecret: "test-secret",
		GoogleRedirectURL:  "http://localhost:8080/callback",
		YouTubeAPIKey:      "youtube-key",
		TwitchClientID:     "twitch-id",
		TwitchSecret:       "twitch-secret",
		ServerPort:         "8080",
		SessionSecret:      "session-secret",
		SessionDuration:    3600,
	}

	// This test just ensures LogConfiguration doesn't panic
	cfg.LogConfiguration()
}

// clearEnv clears all environment variables used by the config
func clearEnv() {
	os.Unsetenv("GOOGLE_CLIENT_ID")
	os.Unsetenv("GOOGLE_CLIENT_SECRET")
	os.Unsetenv("GOOGLE_REDIRECT_URL")
	os.Unsetenv("YOUTUBE_API_KEY")
	os.Unsetenv("TWITCH_CLIENT_ID")
	os.Unsetenv("TWITCH_SECRET")
	os.Unsetenv("DATABASE_PATH")
	os.Unsetenv("SERVER_PORT")
	os.Unsetenv("SESSION_SECRET")
	os.Unsetenv("SESSION_DURATION")
}
