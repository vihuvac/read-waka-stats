// Package logging provides leveled logging for GitHub Actions.
package logging

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// Level controls verbosity.
type Level int

const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
	LevelDebug
)

// Logger writes colored, leveled messages to stderr.
type Logger struct {
	level  Level
	logger *log.Logger
}

// New creates a logger. When debug is true, Info and Debug messages are shown.
func New(debug bool) *Logger {
	level := LevelError
	if debug {
		level = LevelDebug
	}
	return &Logger{
		level:  level,
		logger: log.New(os.Stderr, "", 0),
	}
}

const (
	colorReset  = "\u001b[0m"
	colorRed    = "\u001b[31m"
	colorGreen  = "\u001b[32m"
	colorYellow = "\u001b[33m"
	colorBlue   = "\u001b[34m"
)

// Success logs a green informational message (always shown at Info+ when debug, or as info when not debug for milestones).
func (l *Logger) Success(format string, args ...any) {
	if l.level >= LevelInfo {
		l.logger.Printf("%s%s%s", colorGreen, fmt.Sprintf(format, args...), colorReset)
	} else {
		// Milestone success messages are useful even without debug.
		l.logger.Printf("%s%s%s", colorGreen, fmt.Sprintf(format, args...), colorReset)
	}
}

// Info logs a blue debug-oriented message.
func (l *Logger) Info(format string, args ...any) {
	if l.level < LevelInfo {
		return
	}
	l.logger.Printf("%s%s%s", colorBlue, fmt.Sprintf(format, args...), colorReset)
}

// Warn logs a yellow warning.
func (l *Logger) Warn(format string, args ...any) {
	if l.level < LevelWarn && l.level < LevelError {
		return
	}
	l.logger.Printf("%s%s%s", colorYellow, fmt.Sprintf(format, args...), colorReset)
}

// Error logs a red error (always shown).
func (l *Logger) Error(format string, args ...any) {
	l.logger.Printf("%s%s%s", colorRed, fmt.Sprintf(format, args...), colorReset)
}

// Fatal logs an error and exits.
func (l *Logger) Fatal(format string, args ...any) {
	l.Error(format, args...)
	os.Exit(1)
}

// ParseBoolTruth returns whether s is a truthy Actions input.
func ParseBoolTruth(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "t", "y", "yes":
		return true
	default:
		return false
	}
}
