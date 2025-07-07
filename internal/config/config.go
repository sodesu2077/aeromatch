package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	Server  ServerConfig
	Engine  EngineConfig
	Storage StorageConfig
	Logging LoggingConfig
	Metrics MetricsConfig
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	GRPCPort       int
	WSPort         int
	MetricsPort    int
	PProfPort      int
	EnablePProf    bool
	MaxMessageSize int
}

// EngineConfig holds matching engine configuration
type EngineConfig struct {
	BufferSize          int
	OrderBookBufferSize int
	SnapshotInterval    time.Duration
	MaxOrderBookDepth   int
	MatchTimeout        time.Duration
}

// StorageConfig holds storage configuration
type StorageConfig struct {
	Enabled        bool
	Type           string
	DSN            string
	LoadOnStartup  bool
	SaveOnShutdown bool
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string
	Format string
	File   string
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled         bool
	RefreshInterval time.Duration
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	_ = godotenv.Load() // Ignore error if .env doesn't exist

	return &Config{
		Server:  loadServerConfig(),
		Engine:  loadEngineConfig(),
		Storage: loadStorageConfig(),
		Logging: loadLoggingConfig(),
		Metrics: loadMetricsConfig(),
	}, nil
}

// loadServerConfig loads server-related configuration
func loadServerConfig() ServerConfig {
	return ServerConfig{
		GRPCPort:       getEnvInt("AEROMATCH_GRPC_PORT", 50051),
		WSPort:         getEnvInt("AEROMATCH_WS_PORT", 8080),
		MetricsPort:    getEnvInt("AEROMATCH_METRICS_PORT", 9090),
		PProfPort:      getEnvInt("AEROMATCH_PPROF_PORT", 6060),
		EnablePProf:    getEnvBool("AEROMATCH_ENABLE_PPROF", false),
		MaxMessageSize: getEnvInt("AEROMATCH_MAX_MESSAGE_SIZE", 64*1024*1024), // 64MB
	}
}

// loadEngineConfig loads engine-related configuration
func loadEngineConfig() EngineConfig {
	return EngineConfig{
		BufferSize:          getEnvInt("AEROMATCH_BUFFER_SIZE", 1000000),
		OrderBookBufferSize: getEnvInt("AEROMATCH_ORDER_BOOK_BUFFER", 10000),
		SnapshotInterval:    getEnvDuration("AEROMATCH_SNAPSHOT_INTERVAL", 100*time.Millisecond),
		MaxOrderBookDepth:   getEnvInt("AEROMATCH_MAX_ORDER_BOOK_DEPTH", 100),
		MatchTimeout:        getEnvDuration("AEROMATCH_MATCH_TIMEOUT", 10*time.Millisecond),
	}
}

// loadStorageConfig loads storage-related configuration
func loadStorageConfig() StorageConfig {
	return StorageConfig{
		Enabled:        getEnvBool("AEROMATCH_STORAGE_ENABLED", false),
		Type:           getEnvString("AEROMATCH_STORAGE_TYPE", "memory"),
		DSN:            getEnvString("AEROMATCH_STORAGE_DSN", ""),
		LoadOnStartup:  getEnvBool("AEROMATCH_STORAGE_LOAD_ON_STARTUP", false),
		SaveOnShutdown: getEnvBool("AEROMATCH_STORAGE_SAVE_ON_SHUTDOWN", false),
	}
}

// loadLoggingConfig loads logging-related configuration
func loadLoggingConfig() LoggingConfig {
	return LoggingConfig{
		Level:  getEnvString("AEROMATCH_LOG_LEVEL", "info"),
		Format: getEnvString("AEROMATCH_LOG_FORMAT", "text"),
		File:   getEnvString("AEROMATCH_LOG_FILE", ""), // Empty = stdout
	}
}

// loadMetricsConfig loads metrics-related configuration
func loadMetricsConfig() MetricsConfig {
	return MetricsConfig{
		Enabled:         getEnvBool("AEROMATCH_METRICS_ENABLED", true),
		RefreshInterval: getEnvDuration("AEROMATCH_METRICS_REFRESH_INTERVAL", 5*time.Second),
	}
}

// Helper functions for environment variable parsing

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
		// Also accept "true"/"false" strings
		switch strings.ToLower(value) {
		case "true", "yes", "1":
			return true
		case "false", "no", "0":
			return false
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.GRPCPort <= 0 || c.Server.GRPCPort > 65535 {
		return fmt.Errorf("invalid GRPC port: %d", c.Server.GRPCPort)
	}

	if c.Engine.BufferSize <= 0 {
		return fmt.Errorf("invalid buffer size: %d", c.Engine.BufferSize)
	}

	if c.Storage.Enabled && c.Storage.DSN == "" && c.Storage.Type != "memory" {
		return fmt.Errorf("DSN required for non-memory storage")
	}

	return nil
}

// String returns a safe string representation (without sensitive data)
func (c *Config) String() string {
	return fmt.Sprintf(
		"Server{GRPC:%d, WS:%d, Metrics:%d}, Engine{Buffer:%d}, Storage{Type:%s, Enabled:%v}",
		c.Server.GRPCPort, c.Server.WSPort, c.Server.MetricsPort,
		c.Engine.BufferSize, c.Storage.Type, c.Storage.Enabled,
	)
}
