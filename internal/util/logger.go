package util

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
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
	return nil, nil
}
