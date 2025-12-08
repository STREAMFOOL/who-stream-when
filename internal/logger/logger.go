// Package logger provides structured logging capabilities for the application.
// It supports multiple log levels (Debug, Info, Warn, Error) and structured fields.
//
// Example usage:
//
//	logger := logger.New(logger.LevelInfo)
//	logger.Info("User logged in", map[string]interface{}{
//	    "user_id": "123",
//	    "ip": "192.168.1.1",
//	})
//
// Or use the global logger:
//
//	logger.Info("Application started", nil)
package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

// Level represents the severity level of a log message
type Level int

const (
	// LevelDebug is for detailed debugging information
	LevelDebug Level = iota
	// LevelInfo is for general informational messages
	LevelInfo
	// LevelWarn is for warning messages
	LevelWarn
	// LevelError is for error messages
	LevelError
)

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging capabilities
type Logger struct {
	level  Level
	logger *log.Logger
}

// New creates a new Logger instance
func New(level Level) *Logger {
	return &Logger{
		level:  level,
		logger: log.New(os.Stdout, "", 0),
	}
}

// Default returns a default logger instance with Info level
func Default() *Logger {
	return New(LevelInfo)
}

// log writes a log message with the specified level
func (l *Logger) log(level Level, msg string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format(time.RFC3339)
	output := fmt.Sprintf("[%s] %s: %s", timestamp, level.String(), msg)

	if len(fields) > 0 {
		output += " |"
		for k, v := range fields {
			output += fmt.Sprintf(" %s=%v", k, v)
		}
	}

	l.logger.Println(output)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields map[string]interface{}) {
	l.log(LevelDebug, msg, fields)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields map[string]interface{}) {
	l.log(LevelInfo, msg, fields)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields map[string]interface{}) {
	l.log(LevelWarn, msg, fields)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields map[string]interface{}) {
	l.log(LevelError, msg, fields)
}

// WithContext returns a logger with context information
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return l
}

// WithField returns a logger with a single field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return l
}

// WithFields returns a logger with multiple fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	return l
}

// Global logger instance
var globalLogger = Default()

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger *Logger) {
	globalLogger = logger
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() *Logger {
	return globalLogger
}

// Debug logs a debug message using the global logger
func Debug(msg string, fields map[string]interface{}) {
	globalLogger.Debug(msg, fields)
}

// Info logs an info message using the global logger
func Info(msg string, fields map[string]interface{}) {
	globalLogger.Info(msg, fields)
}

// Warn logs a warning message using the global logger
func Warn(msg string, fields map[string]interface{}) {
	globalLogger.Warn(msg, fields)
}

// Error logs an error message using the global logger
func Error(msg string, fields map[string]interface{}) {
	globalLogger.Error(msg, fields)
}
