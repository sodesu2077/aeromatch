package util

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
	LevelPanic
)

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	case LevelPanic:
		return "PANIC"
	default:
		return "UNKNOWN"
	}
}

type LoggerConfig struct {
	Level      LogLevel
	Format     string
	Output     io.Writer
	File       string
	MaxSize    int64 // Maximum file size in bytes
	MaxBackups int   // Maximum number of old log files to retain
	MaxAge     int   // Maximum number of days to retain log files
}

type Logger struct {
	config     LoggerConfig
	logger     *log.Logger
	mu         sync.Mutex
	file       *os.File
	callerInfo bool // Enable or disable caller information
}

var (
	defaultLogger *Logger
	once          sync.Once
)

func DefaultConfig() LoggerConfig {
	return LoggerConfig{
		Level:  LevelInfo,
		Format: "text",
		Output: os.Stdout,
	}
}

func Init(level LogLevel, format string, output io.Writer) {
	once.Do(func() {
		config := DefaultConfig()
		config.Level = level
		config.Format = format
		config.Output = output

		var err error
		defaultLogger, err = NewLogger(config)
		if err != nil {
			log.Printf("Failed to create logger: %v", err)
			// Fallback to standard logger
			defaultLogger = &Logger{
				config: config,
				logger: log.New(os.Stdout, "", log.LstdFlags),
			}
		}
	})
}

// InitFile initializes the logger with file output
func InitFile(level LogLevel, format, filePath string, maxSize int64, maxBackups, maxAge int) error {
	config := DefaultConfig()
	config.Level = level
	config.Format = format
	config.File = filePath
	config.MaxSize = maxSize
	config.MaxBackups = maxBackups
	config.MaxAge = maxAge

	logger, err := NewLogger(config)
	if err != nil {
		return err
	}

	defaultLogger = logger
	return nil
}

// NewLogger creates a new logger instance
func NewLogger(config LoggerConfig) (*Logger, error) {
	l := &Logger{
		config:     config,
		callerInfo: true,
	}

	var output io.Writer = config.Output

	// Setup file output if specified
	if config.File != "" {
		file, err := setupLogFile(config.File)
		if err != nil {
			return nil, fmt.Errorf("failed to setup log file: %w", err)
		}
		l.file = file
		output = file
	}

	// Create the logger
	l.logger = log.New(output, "", 0) // We'll handle prefixes ourselves

	return l, nil
}

// setupLogFile sets up log file with rotation
func setupLogFile(filePath string) (*os.File, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil { // permissions: rwxr-xr-x
		return nil, err
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config.Level = level
}

func (l *Logger) SetCallerInfo(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.callerInfo = enabled
}

// logInternal is the internal logging method
func (l *Logger) logInternal(level LogLevel, msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if we should log this level
	if level < l.config.Level {
		return
	}

	// Format the message
	formattedMsg := fmt.Sprintf(msg, args...)

	// Get caller information if enabled
	var callerInfo string
	if l.callerInfo && level >= LevelDebug {
		callerInfo = l.getCallerInfo()
	}

	var logEntry string
	switch l.config.Format {
	case "json":
		logEntry = l.formatJSON(level, formattedMsg, callerInfo)
	default:
		logEntry = l.formatText(level, formattedMsg, callerInfo)
	}

	l.logger.Println(logEntry)

	// For fatal and panic, handle appropriately
	switch level {
	case LevelFatal:
		os.Exit(1) // TODO: Implement proper fatal error handling
	case LevelPanic:
		panic(formattedMsg)
	}
}

// formatText formats a log entry in text format
func (l *Logger) formatText(level LogLevel, msg, callerInfo string) string {
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")

	entry := fmt.Sprintf("%s %-5s %s", timestamp, level.String(), msg)
	if callerInfo != "" {
		entry += " " + callerInfo
	}

	return entry
}

// formatJSON formats a log entry in JSON format
func (l *Logger) formatJSON(level LogLevel, msg, callerInfo string) string {
	timestamp := time.Now().Format(time.RFC3339Nano)

	entry := fmt.Sprintf(`{"time":"%s","level":"%s","message":%q`,
		timestamp, level.String(), msg)

	if callerInfo != "" {
		// Parse caller info into components
		if parts := strings.Split(callerInfo, ":"); len(parts) >= 2 { // TODO: Implement proper caller info parsing
			entry += fmt.Sprintf(`,"file":%q,"line":%q`, parts[0], parts[1])
		}
	}

	entry += "}"
	return entry
}

// getCallerInfo returns the caller file and line number
func (l *Logger) getCallerInfo() string {
	// Skip 4 callers: getCallerInfo → logInternal → public method (Debug/Info/etc.) → actual caller
	_, file, line, ok := runtime.Caller(4)
	if !ok {
		return ""
	}

	// Get just the filename, not the full path
	filename := filepath.Base(file)
	return fmt.Sprintf("%s:%d", filename, line)
}

// Close closes the logger and any open files
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Public logging methods

func (l *Logger) Debug(msg string, args ...interface{}) {
	l.logInternal(LevelDebug, msg, args...)
}

func (l *Logger) Info(msg string, args ...interface{}) {
	l.logInternal(LevelInfo, msg, args...)
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	l.logInternal(LevelWarn, msg, args...)
}

func (l *Logger) Error(msg string, args ...interface{}) {
	l.logInternal(LevelError, msg, args...)
}

func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.logInternal(LevelFatal, msg, args...)
}

func (l *Logger) Panic(msg string, args ...interface{}) {
	l.logInternal(LevelPanic, msg, args...)
}

func GetLevel() LogLevel {
	if defaultLogger != nil {
		return defaultLogger.config.Level
	}
	return LevelInfo
}

func SetGlobalLevel(level LogLevel) {
	if defaultLogger != nil {
		defaultLogger.SetLevel(level)
	}
}

// Sync flushes any buffered log entries
func Sync() {
	if defaultLogger != nil && defaultLogger.file != nil {
		defaultLogger.file.Sync()
	}
}
