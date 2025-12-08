package logger

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLogger_Levels(t *testing.T) {
	tests := []struct {
		name     string
		level    Level
		logFunc  func(*Logger, string, map[string]interface{})
		message  string
		fields   map[string]interface{}
		expected string
	}{
		{
			name:     "debug message",
			level:    LevelDebug,
			logFunc:  (*Logger).Debug,
			message:  "debug message",
			fields:   map[string]interface{}{"key": "value"},
			expected: "DEBUG: debug message | key=value",
		},
		{
			name:     "info message",
			level:    LevelInfo,
			logFunc:  (*Logger).Info,
			message:  "info message",
			fields:   map[string]interface{}{"count": 42},
			expected: "INFO: info message | count=42",
		},
		{
			name:     "warn message",
			level:    LevelWarn,
			logFunc:  (*Logger).Warn,
			message:  "warning message",
			fields:   map[string]interface{}{"status": "degraded"},
			expected: "WARN: warning message | status=degraded",
		},
		{
			name:     "error message",
			level:    LevelError,
			logFunc:  (*Logger).Error,
			message:  "error occurred",
			fields:   map[string]interface{}{"error": "connection failed"},
			expected: "ERROR: error occurred | error=connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := New(tt.level)
			logger.logger = log.New(&buf, "", 0)

			tt.logFunc(logger, tt.message, tt.fields)

			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("Expected output to contain %q, got %q", tt.expected, output)
			}
		})
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := New(LevelWarn)
	logger.logger = log.New(&buf, "", 0)

	// Debug and Info should be filtered out
	logger.Debug("debug message", nil)
	logger.Info("info message", nil)

	output := buf.String()
	if output != "" {
		t.Errorf("Expected no output for filtered levels, got %q", output)
	}

	// Warn and Error should pass through
	logger.Warn("warning message", nil)
	logger.Error("error message", nil)

	output = buf.String()
	if !strings.Contains(output, "WARN") || !strings.Contains(output, "ERROR") {
		t.Errorf("Expected WARN and ERROR in output, got %q", output)
	}
}

func TestLogger_NoFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New(LevelInfo)
	logger.logger = log.New(&buf, "", 0)

	logger.Info("simple message", nil)

	output := buf.String()
	if !strings.Contains(output, "INFO: simple message") {
		t.Errorf("Expected message without fields, got %q", output)
	}
	if strings.Contains(output, "|") {
		t.Errorf("Expected no field separator when no fields provided, got %q", output)
	}
}

func TestLogger_MultipleFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New(LevelInfo)
	logger.logger = log.New(&buf, "", 0)

	fields := map[string]interface{}{
		"user_id":    "123",
		"action":     "login",
		"ip_address": "192.168.1.1",
	}

	logger.Info("user action", fields)

	output := buf.String()
	if !strings.Contains(output, "user_id=123") {
		t.Error("Expected user_id field in output")
	}
	if !strings.Contains(output, "action=login") {
		t.Error("Expected action field in output")
	}
	if !strings.Contains(output, "ip_address=192.168.1.1") {
		t.Error("Expected ip_address field in output")
	}
}

func TestGlobalLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := New(LevelInfo)
	logger.logger = log.New(&buf, "", 0)

	SetGlobalLogger(logger)

	Info("global message", map[string]interface{}{"test": "value"})

	output := buf.String()
	if !strings.Contains(output, "INFO: global message") {
		t.Errorf("Expected global logger to work, got %q", output)
	}
}
