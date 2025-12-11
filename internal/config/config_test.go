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
	if cfg.GoogleRedirectURL != "http://localhost:8080/auth/google/callback" {
		t.Errorf("GoogleRedirectURL = %s, want http://localhost:8080/auth/google/callback", cfg.GoogleRedirectURL)
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

// Feature Flag Tests

func TestFeatureFlags_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		flags    FeatureFlags
		check    FeatureFlags
		expected bool
	}{
		{"Kick enabled", FeatureKick, FeatureKick, true},
		{"YouTube enabled", FeatureYouTube, FeatureYouTube, true},
		{"Twitch enabled", FeatureTwitch, FeatureTwitch, true},
		{"Kick not enabled", FeatureYouTube, FeatureKick, false},
		{"YouTube not enabled", FeatureKick, FeatureYouTube, false},
		{"Twitch not enabled", FeatureKick, FeatureTwitch, false},
		{"Multiple flags - check Kick", FeatureKick | FeatureYouTube, FeatureKick, true},
		{"Multiple flags - check YouTube", FeatureKick | FeatureYouTube, FeatureYouTube, true},
		{"Multiple flags - check Twitch", FeatureKick | FeatureYouTube, FeatureTwitch, false},
		{"All flags enabled - check Kick", FeatureKick | FeatureYouTube | FeatureTwitch, FeatureKick, true},
		{"All flags enabled - check YouTube", FeatureKick | FeatureYouTube | FeatureTwitch, FeatureYouTube, true},
		{"All flags enabled - check Twitch", FeatureKick | FeatureYouTube | FeatureTwitch, FeatureTwitch, true},
		{"No flags enabled", FeatureFlags(0), FeatureKick, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.flags.IsEnabled(tt.check)
			if got != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFeatureFlags_Enable(t *testing.T) {
	tests := []struct {
		name     string
		initial  FeatureFlags
		enable   FeatureFlags
		expected FeatureFlags
	}{
		{"Enable Kick on empty", FeatureFlags(0), FeatureKick, FeatureKick},
		{"Enable YouTube on empty", FeatureFlags(0), FeatureYouTube, FeatureYouTube},
		{"Enable Twitch on empty", FeatureFlags(0), FeatureTwitch, FeatureTwitch},
		{"Enable YouTube when Kick enabled", FeatureKick, FeatureYouTube, FeatureKick | FeatureYouTube},
		{"Enable Twitch when Kick enabled", FeatureKick, FeatureTwitch, FeatureKick | FeatureTwitch},
		{"Enable Kick when already enabled", FeatureKick, FeatureKick, FeatureKick},
		{"Enable all platforms sequentially", FeatureKick | FeatureYouTube, FeatureTwitch, FeatureKick | FeatureYouTube | FeatureTwitch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := tt.initial
			flags.Enable(tt.enable)
			if flags != tt.expected {
				t.Errorf("Enable() resulted in %v, want %v", flags, tt.expected)
			}
		})
	}
}

func TestFeatureFlags_Disable(t *testing.T) {
	tests := []struct {
		name     string
		initial  FeatureFlags
		disable  FeatureFlags
		expected FeatureFlags
	}{
		{"Disable Kick when enabled", FeatureKick, FeatureKick, FeatureFlags(0)},
		{"Disable YouTube when enabled", FeatureYouTube, FeatureYouTube, FeatureFlags(0)},
		{"Disable Twitch when enabled", FeatureTwitch, FeatureTwitch, FeatureFlags(0)},
		{"Disable Kick when not enabled", FeatureYouTube, FeatureKick, FeatureYouTube},
		{"Disable YouTube from multiple", FeatureKick | FeatureYouTube, FeatureYouTube, FeatureKick},
		{"Disable Twitch from multiple", FeatureKick | FeatureTwitch, FeatureTwitch, FeatureKick},
		{"Disable Kick from all", FeatureKick | FeatureYouTube | FeatureTwitch, FeatureKick, FeatureYouTube | FeatureTwitch},
		{"Disable when already disabled", FeatureFlags(0), FeatureKick, FeatureFlags(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := tt.initial
			flags.Disable(tt.disable)
			if flags != tt.expected {
				t.Errorf("Disable() resulted in %v, want %v", flags, tt.expected)
			}
		})
	}
}

func TestFeatureFlags_GetEnabledPlatforms(t *testing.T) {
	tests := []struct {
		name     string
		flags    FeatureFlags
		expected []string
	}{
		{"No platforms enabled", FeatureFlags(0), []string{}},
		{"Only Kick enabled", FeatureKick, []string{"kick"}},
		{"Only YouTube enabled", FeatureYouTube, []string{"youtube"}},
		{"Only Twitch enabled", FeatureTwitch, []string{"twitch"}},
		{"Kick and YouTube enabled", FeatureKick | FeatureYouTube, []string{"kick", "youtube"}},
		{"Kick and Twitch enabled", FeatureKick | FeatureTwitch, []string{"kick", "twitch"}},
		{"YouTube and Twitch enabled", FeatureYouTube | FeatureTwitch, []string{"youtube", "twitch"}},
		{"All platforms enabled", FeatureKick | FeatureYouTube | FeatureTwitch, []string{"kick", "youtube", "twitch"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.flags.GetEnabledPlatforms()
			if len(got) != len(tt.expected) {
				t.Errorf("GetEnabledPlatforms() returned %d platforms, want %d", len(got), len(tt.expected))
				t.Errorf("Got: %v, Want: %v", got, tt.expected)
				return
			}
			for i, platform := range tt.expected {
				if got[i] != platform {
					t.Errorf("GetEnabledPlatforms()[%d] = %s, want %s", i, got[i], platform)
				}
			}
		})
	}
}

func TestFeatureFlags_EnableDisableSequence(t *testing.T) {
	var flags FeatureFlags

	// Start with no flags
	if flags.IsEnabled(FeatureKick) {
		t.Error("Kick should not be enabled initially")
	}

	// Enable Kick
	flags.Enable(FeatureKick)
	if !flags.IsEnabled(FeatureKick) {
		t.Error("Kick should be enabled after Enable()")
	}

	// Enable YouTube
	flags.Enable(FeatureYouTube)
	if !flags.IsEnabled(FeatureKick) || !flags.IsEnabled(FeatureYouTube) {
		t.Error("Both Kick and YouTube should be enabled")
	}

	// Enable Twitch
	flags.Enable(FeatureTwitch)
	if !flags.IsEnabled(FeatureKick) || !flags.IsEnabled(FeatureYouTube) || !flags.IsEnabled(FeatureTwitch) {
		t.Error("All platforms should be enabled")
	}

	// Disable YouTube
	flags.Disable(FeatureYouTube)
	if !flags.IsEnabled(FeatureKick) || flags.IsEnabled(FeatureYouTube) || !flags.IsEnabled(FeatureTwitch) {
		t.Error("Kick and Twitch should be enabled, YouTube should be disabled")
	}

	// Disable all
	flags.Disable(FeatureKick)
	flags.Disable(FeatureTwitch)
	if flags.IsEnabled(FeatureKick) || flags.IsEnabled(FeatureYouTube) || flags.IsEnabled(FeatureTwitch) {
		t.Error("No platforms should be enabled")
	}
}

func TestParseFeatureFlags_DefaultConfiguration(t *testing.T) {
	// Test default configuration (Kick enabled, YouTube/Twitch disabled)
	flags := parseFeatureFlags("kick")

	if !flags.IsEnabled(FeatureKick) {
		t.Error("Kick should be enabled by default")
	}
	if flags.IsEnabled(FeatureYouTube) {
		t.Error("YouTube should be disabled by default")
	}
	if flags.IsEnabled(FeatureTwitch) {
		t.Error("Twitch should be disabled by default")
	}

	platforms := flags.GetEnabledPlatforms()
	if len(platforms) != 1 || platforms[0] != "kick" {
		t.Errorf("Default configuration should only enable Kick, got: %v", platforms)
	}
}

func TestParseFeatureFlags_IndividualPlatforms(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected FeatureFlags
	}{
		{"Empty string", "", FeatureFlags(0)},
		{"Only Kick", "kick", FeatureKick},
		{"Only YouTube", "youtube", FeatureYouTube},
		{"Only Twitch", "twitch", FeatureTwitch},
		{"Kick uppercase", "KICK", FeatureKick},
		{"YouTube mixed case", "YouTube", FeatureYouTube},
		{"Twitch with spaces", " twitch ", FeatureTwitch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFeatureFlags(tt.input)
			if got != tt.expected {
				t.Errorf("parseFeatureFlags(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseFeatureFlags_MultiplePlatforms(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected FeatureFlags
	}{
		{"Kick and YouTube", "kick,youtube", FeatureKick | FeatureYouTube},
		{"Kick and Twitch", "kick,twitch", FeatureKick | FeatureTwitch},
		{"YouTube and Twitch", "youtube,twitch", FeatureYouTube | FeatureTwitch},
		{"All platforms", "kick,youtube,twitch", FeatureKick | FeatureYouTube | FeatureTwitch},
		{"All platforms with spaces", "kick, youtube, twitch", FeatureKick | FeatureYouTube | FeatureTwitch},
		{"All platforms mixed case", "Kick,YouTube,Twitch", FeatureKick | FeatureYouTube | FeatureTwitch},
		{"Duplicate platforms", "kick,kick,youtube", FeatureKick | FeatureYouTube},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFeatureFlags(tt.input)
			if got != tt.expected {
				t.Errorf("parseFeatureFlags(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLoad_DefaultFeatureFlags(t *testing.T) {
	os.Setenv("GOOGLE_CLIENT_ID", "test-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")
	// Don't set FEATURE_FLAGS to test default
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Default should be Kick enabled, others disabled
	if !cfg.FeatureFlags.IsEnabled(FeatureKick) {
		t.Error("Kick should be enabled by default")
	}
	if cfg.FeatureFlags.IsEnabled(FeatureYouTube) {
		t.Error("YouTube should be disabled by default")
	}
	if cfg.FeatureFlags.IsEnabled(FeatureTwitch) {
		t.Error("Twitch should be disabled by default")
	}
}

func TestLoad_CustomFeatureFlags(t *testing.T) {
	tests := []struct {
		name          string
		flagsEnv      string
		expectKick    bool
		expectYouTube bool
		expectTwitch  bool
	}{
		{"Only Kick", "kick", true, false, false},
		{"Only YouTube", "youtube", false, true, false},
		{"Only Twitch", "twitch", false, false, true},
		{"Kick and YouTube", "kick,youtube", true, true, false},
		{"All platforms", "kick,youtube,twitch", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("GOOGLE_CLIENT_ID", "test-id")
			os.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")
			os.Setenv("FEATURE_FLAGS", tt.flagsEnv)
			defer clearEnv()

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}

			if cfg.FeatureFlags.IsEnabled(FeatureKick) != tt.expectKick {
				t.Errorf("Kick enabled = %v, want %v", cfg.FeatureFlags.IsEnabled(FeatureKick), tt.expectKick)
			}
			if cfg.FeatureFlags.IsEnabled(FeatureYouTube) != tt.expectYouTube {
				t.Errorf("YouTube enabled = %v, want %v", cfg.FeatureFlags.IsEnabled(FeatureYouTube), tt.expectYouTube)
			}
			if cfg.FeatureFlags.IsEnabled(FeatureTwitch) != tt.expectTwitch {
				t.Errorf("Twitch enabled = %v, want %v", cfg.FeatureFlags.IsEnabled(FeatureTwitch), tt.expectTwitch)
			}
		})
	}
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
	os.Unsetenv("FEATURE_FLAGS")
}
