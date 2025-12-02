package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Logger provides structured logging with levels
type Logger struct {
	level   string
	verbose bool
	logger  *log.Logger
}

// NewLogger creates a new logger instance
func NewLogger(config LoggingConfig) *Logger {
	return &Logger{
		level:   strings.ToLower(config.Level),
		verbose: config.Verbose,
		logger:  log.New(os.Stdout, "", 0),
	}
}

// shouldLog determines if a message should be logged based on level
func (l *Logger) shouldLog(level string) bool {
	levels := map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
	}

	currentLevel, ok := levels[l.level]
	if !ok {
		currentLevel = levels["info"]
	}

	messageLevel, ok := levels[level]
	if !ok {
		return true
	}

	return messageLevel >= currentLevel
}

// formatMessage formats a log message with timestamp and level
func (l *Logger) formatMessage(level, format string, args ...interface{}) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	return fmt.Sprintf("[%s] [%s] %s", timestamp, strings.ToUpper(level), message)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.shouldLog("debug") {
		l.logger.Println(l.formatMessage("debug", format, args...))
	}
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	if l.shouldLog("info") {
		l.logger.Println(l.formatMessage("info", format, args...))
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	if l.shouldLog("warn") {
		l.logger.Println(l.formatMessage("warn", format, args...))
	}
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	if l.shouldLog("error") {
		l.logger.Println(l.formatMessage("error", format, args...))
	}
}
