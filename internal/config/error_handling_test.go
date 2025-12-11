package config

import (
	"os"
	"testing"
)

// TestConfigurationErrorMessages tests that configuration errors have clear messages
func TestConfigurationErrorMessages(t *testing.T) {
	tests := []struct {
		name          string
		setupEnv      func()
		expectedError string
		shouldContain []string
	}{
		{
			name: "missing GOOGLE_CLIENT_ID shows clear error",
			setupEnv: func() {
				clearEnv()
				os.Setenv("GOOGLE_CLIENT_SECRET", "secret")
			},
			expectedError: "GOOGLE_CLIENT_ID environment variable is required",
			shouldContain: []string{"GOOGLE_CLIENT_ID", "required"},
		},
		{
			name: "missing GOOGLE_CLIENT_SECRET shows clear error",
			setupEnv: func() {
				clearEnv()
				os.Setenv("GOOGLE_CLIENT_ID", "id")
			},
			expectedError: "GOOGLE_CLIENT_SECRET environment variable is required",
			shouldContain: []string{"GOOGLE_CLIENT_SECRET", "required"},
		},
		{
			name: "invalid SESSION_DURATION shows clear error",
			setupEnv: func() {
				clearEnv()
				os.Setenv("GOOGLE_CLIENT_ID", "id")
				os.Setenv("GOOGLE_CLIENT_SECRET", "secret")
				os.Setenv("SESSION_DURATION", "not-a-number")
			},
			expectedError: "",
			shouldContain: []string{"SESSION_DURATION", "invalid"},
		},
		{
			name: "negative SESSION_DURATION shows clear error",
			setupEnv: func() {
				clearEnv()
				os.Setenv("GOOGLE_CLIENT_ID", "id")
				os.Setenv("GOOGLE_CLIENT_SECRET", "secret")
				os.Setenv("SESSION_DURATION", "-100")
			},
			expectedError: "",
			shouldContain: []string{"SESSION_DURATION", "positive"},
		},
		{
			name: "empty DATABASE_PATH uses default",
			setupEnv: func() {
				clearEnv()
				os.Setenv("GOOGLE_CLIENT_ID", "id")
				os.Setenv("GOOGLE_CLIENT_SECRET", "secret")
				os.Setenv("DATABASE_PATH", "")
			},
			expectedError: "",
			shouldContain: []string{},
		},
		{
			name: "empty SERVER_PORT uses default",
			setupEnv: func() {
				clearEnv()
				os.Setenv("GOOGLE_CLIENT_ID", "id")
				os.Setenv("GOOGLE_CLIENT_SECRET", "secret")
				os.Setenv("SERVER_PORT", "")
			},
			expectedError: "",
			shouldContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer clearEnv()

			_, err := Load()
			if tt.expectedError != "" {
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}

				errMsg := err.Error()
				if errMsg != tt.expectedError {
					t.Errorf("Expected error message '%s', got '%s'", tt.expectedError, errMsg)
				}
			}

			if err != nil {
				errMsg := err.Error()
				// Check that error message contains expected keywords
				for _, keyword := range tt.shouldContain {
					if !contains(errMsg, keyword) {
						t.Errorf("Expected error message to contain '%s', got: %s", keyword, errMsg)
					}
				}
			}
		})
	}
}

// TestValidationErrorMessages tests that validation errors have clear messages
func TestValidationErrorMessages(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectedError string
		shouldContain []string
	}{
		{
			name: "missing GoogleClientID shows clear error",
			config: &Config{
				GoogleClientID:     "",
				GoogleClientSecret: "secret",
				DatabasePath:       "./test.db",
				ServerPort:         "8080",
				SessionDuration:    3600,
			},
			expectedError: "GOOGLE_CLIENT_ID environment variable is required",
			shouldContain: []string{"GOOGLE_CLIENT_ID", "required"},
		},
		{
			name: "missing GoogleClientSecret shows clear error",
			config: &Config{
				GoogleClientID:     "id",
				GoogleClientSecret: "",
				DatabasePath:       "./test.db",
				ServerPort:         "8080",
				SessionDuration:    3600,
			},
			expectedError: "GOOGLE_CLIENT_SECRET environment variable is required",
			shouldContain: []string{"GOOGLE_CLIENT_SECRET", "required"},
		},
		{
			name: "empty DatabasePath shows clear error",
			config: &Config{
				GoogleClientID:     "id",
				GoogleClientSecret: "secret",
				DatabasePath:       "",
				ServerPort:         "8080",
				SessionDuration:    3600,
			},
			expectedError: "DATABASE_PATH cannot be empty",
			shouldContain: []string{"DATABASE_PATH", "empty"},
		},
		{
			name: "empty ServerPort shows clear error",
			config: &Config{
				GoogleClientID:     "id",
				GoogleClientSecret: "secret",
				DatabasePath:       "./test.db",
				ServerPort:         "",
				SessionDuration:    3600,
			},
			expectedError: "SERVER_PORT cannot be empty",
			shouldContain: []string{"SERVER_PORT", "empty"},
		},
		{
			name: "zero SessionDuration shows clear error",
			config: &Config{
				GoogleClientID:     "id",
				GoogleClientSecret: "secret",
				DatabasePath:       "./test.db",
				ServerPort:         "8080",
				SessionDuration:    0,
			},
			expectedError: "SESSION_DURATION must be positive, got 0",
			shouldContain: []string{"SESSION_DURATION", "positive"},
		},
		{
			name: "negative SessionDuration shows clear error",
			config: &Config{
				GoogleClientID:     "id",
				GoogleClientSecret: "secret",
				DatabasePath:       "./test.db",
				ServerPort:         "8080",
				SessionDuration:    -100,
			},
			expectedError: "SESSION_DURATION must be positive, got -100",
			shouldContain: []string{"SESSION_DURATION", "positive"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err == nil {
				t.Fatalf("Expected error, got nil")
			}

			errMsg := err.Error()
			if errMsg != tt.expectedError {
				t.Errorf("Expected error message '%s', got '%s'", tt.expectedError, errMsg)
			}

			// Check that error message contains expected keywords
			for _, keyword := range tt.shouldContain {
				if !contains(errMsg, keyword) {
					t.Errorf("Expected error message to contain '%s', got: %s", keyword, errMsg)
				}
			}
		})
	}
}

// TestGracefulDegradationForMissingOptionalKeys tests that missing optional API keys don't cause failures
func TestGracefulDegradationForMissingOptionalKeys(t *testing.T) {
	tests := []struct {
		name       string
		setupEnv   func()
		shouldLoad bool
	}{
		{
			name: "missing YOUTUBE_API_KEY allows loading",
			setupEnv: func() {
				clearEnv()
				os.Setenv("GOOGLE_CLIENT_ID", "id")
				os.Setenv("GOOGLE_CLIENT_SECRET", "secret")
				os.Unsetenv("YOUTUBE_API_KEY")
			},
			shouldLoad: true,
		},
		{
			name: "missing TWITCH_CLIENT_ID allows loading",
			setupEnv: func() {
				clearEnv()
				os.Setenv("GOOGLE_CLIENT_ID", "id")
				os.Setenv("GOOGLE_CLIENT_SECRET", "secret")
				os.Unsetenv("TWITCH_CLIENT_ID")
			},
			shouldLoad: true,
		},
		{
			name: "missing TWITCH_SECRET allows loading",
			setupEnv: func() {
				clearEnv()
				os.Setenv("GOOGLE_CLIENT_ID", "id")
				os.Setenv("GOOGLE_CLIENT_SECRET", "secret")
				os.Unsetenv("TWITCH_SECRET")
			},
			shouldLoad: true,
		},
		{
			name: "all optional keys missing allows loading",
			setupEnv: func() {
				clearEnv()
				os.Setenv("GOOGLE_CLIENT_ID", "id")
				os.Setenv("GOOGLE_CLIENT_SECRET", "secret")
				os.Unsetenv("YOUTUBE_API_KEY")
				os.Unsetenv("TWITCH_CLIENT_ID")
				os.Unsetenv("TWITCH_SECRET")
			},
			shouldLoad: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer clearEnv()

			cfg, err := Load()
			if tt.shouldLoad {
				if err != nil {
					t.Errorf("Expected successful load, got error: %v", err)
				}
				if cfg == nil {
					t.Error("Expected config to be non-nil")
				}
			} else {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			}
		})
	}
}

// TestFeatureFlagErrorHandling tests error handling for feature flags
func TestFeatureFlagErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		flagsEnv       string
		expectedKick   bool
		expectedYT     bool
		expectedTwitch bool
	}{
		{
			name:           "invalid platform name is ignored",
			flagsEnv:       "kick,invalid,youtube",
			expectedKick:   true,
			expectedYT:     true,
			expectedTwitch: false,
		},
		{
			name:           "empty string defaults to kick",
			flagsEnv:       "",
			expectedKick:   true,
			expectedYT:     false,
			expectedTwitch: false,
		},
		{
			name:           "whitespace-only string results in no flags",
			flagsEnv:       "   ",
			expectedKick:   false,
			expectedYT:     false,
			expectedTwitch: false,
		},
		{
			name:           "mixed case platform names work",
			flagsEnv:       "KICK,YouTube,TWITCH",
			expectedKick:   true,
			expectedYT:     true,
			expectedTwitch: true,
		},
		{
			name:           "duplicate platform names are handled",
			flagsEnv:       "kick,kick,kick",
			expectedKick:   true,
			expectedYT:     false,
			expectedTwitch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("GOOGLE_CLIENT_ID", "id")
			os.Setenv("GOOGLE_CLIENT_SECRET", "secret")
			os.Setenv("FEATURE_FLAGS", tt.flagsEnv)
			defer clearEnv()

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}

			if cfg.FeatureFlags.IsEnabled(FeatureKick) != tt.expectedKick {
				t.Errorf("Kick enabled = %v, want %v", cfg.FeatureFlags.IsEnabled(FeatureKick), tt.expectedKick)
			}
			if cfg.FeatureFlags.IsEnabled(FeatureYouTube) != tt.expectedYT {
				t.Errorf("YouTube enabled = %v, want %v", cfg.FeatureFlags.IsEnabled(FeatureYouTube), tt.expectedYT)
			}
			if cfg.FeatureFlags.IsEnabled(FeatureTwitch) != tt.expectedTwitch {
				t.Errorf("Twitch enabled = %v, want %v", cfg.FeatureFlags.IsEnabled(FeatureTwitch), tt.expectedTwitch)
			}
		})
	}
}

// TestConfigurationLoggingDoesNotPanic tests that logging configuration doesn't panic
func TestConfigurationLoggingDoesNotPanic(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name: "logs valid configuration",
			config: &Config{
				DatabasePath:       "./test.db",
				GoogleClientID:     "test-id",
				GoogleClientSecret: "test-secret",
				GoogleRedirectURL:  "http://localhost:8080/callback",
				YouTubeAPIKey:      "youtube-key",
				TwitchClientID:     "twitch-id",
				TwitchSecret:       "twitch-secret",
				KickClientID:       "kick-id",
				KickSecret:         "kick-secret",
				ServerPort:         "8080",
				SessionSecret:      "session-secret",
				SessionDuration:    3600,
				FeatureFlags:       FeatureKick | FeatureYouTube,
			},
		},
		{
			name: "logs configuration with empty optional fields",
			config: &Config{
				DatabasePath:       "./test.db",
				GoogleClientID:     "test-id",
				GoogleClientSecret: "test-secret",
				GoogleRedirectURL:  "http://localhost:8080/callback",
				YouTubeAPIKey:      "",
				TwitchClientID:     "",
				TwitchSecret:       "",
				KickClientID:       "",
				KickSecret:         "",
				ServerPort:         "8080",
				SessionSecret:      "session-secret",
				SessionDuration:    3600,
				FeatureFlags:       FeatureKick,
			},
		},
		{
			name: "logs configuration with all feature flags",
			config: &Config{
				DatabasePath:       "./test.db",
				GoogleClientID:     "test-id",
				GoogleClientSecret: "test-secret",
				GoogleRedirectURL:  "http://localhost:8080/callback",
				YouTubeAPIKey:      "youtube-key",
				TwitchClientID:     "twitch-id",
				TwitchSecret:       "twitch-secret",
				KickClientID:       "kick-id",
				KickSecret:         "kick-secret",
				ServerPort:         "8080",
				SessionSecret:      "session-secret",
				SessionDuration:    3600,
				FeatureFlags:       FeatureKick | FeatureYouTube | FeatureTwitch,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("LogConfiguration panicked: %v", r)
				}
			}()

			tt.config.LogConfiguration()
		})
	}
}

// TestMaskSecretProtectsSecrets tests that maskSecret properly masks sensitive data
func TestMaskSecretProtectsSecrets(t *testing.T) {
	tests := []struct {
		name             string
		secret           string
		expected         string
		shouldNotContain string
	}{
		{
			name:             "masks long secret",
			secret:           "very-long-secret-key-12345",
			expected:         "very****",
			shouldNotContain: "long-secret-key-12345",
		},
		{
			name:             "masks medium secret",
			secret:           "medium-secret",
			expected:         "medi****",
			shouldNotContain: "secret",
		},
		{
			name:             "masks short secret",
			secret:           "abc",
			expected:         "****",
			shouldNotContain: "abc",
		},
		{
			name:             "handles empty secret",
			secret:           "",
			expected:         "[not set]",
			shouldNotContain: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskSecret(tt.secret)
			if got != tt.expected {
				t.Errorf("maskSecret(%q) = %q, want %q", tt.secret, got, tt.expected)
			}

			// Verify that sensitive data is not exposed
			if tt.shouldNotContain != "" && contains(got, tt.shouldNotContain) {
				t.Errorf("maskSecret exposed sensitive data: %q contains %q", got, tt.shouldNotContain)
			}
		})
	}
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
