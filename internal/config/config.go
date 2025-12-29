package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// configuration for the service

type Config struct {
	HTTP   HTTPConfig
	DB     DBConfig
	Ingest IngestConfig
}

type HTTPConfig struct {
	Port           int
	RequestTimeout time.Duration
}

type DBConfig struct {
	DatabaseURL string

	MinConns          int32
	MaxConns          int32
	MaxConnIdleTime   time.Duration
	MaxConnLifetime   time.Duration
	HealthCheckPeriod time.Duration

	ConnectTimeout time.Duration
}

type IngestConfig struct {
	BatchWindow time.Duration

	MaxBatch int

	QueueSize int
}

func Load() (Config, error) {
	var cfg Config

	// HTTP
	cfg.HTTP.Port = envInt("PORT", 8080)
	cfg.HTTP.RequestTimeout = envDuration("REQUEST_TIMEOUT", 200*time.Millisecond)

	// DB
	cfg.DB.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DB.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	cfg.DB.MaxConns = int32(envInt("DB_MAX_CONNS", 20))
	cfg.DB.MinConns = int32(envInt("DB_MIN_CONNS", 5))
	cfg.DB.MaxConnIdleTime = envDuration("DB_MAX_CONN_IDLE_TIME", 2*time.Minute)
	cfg.DB.MaxConnLifetime = envDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute)
	cfg.DB.HealthCheckPeriod = envDuration("DB_HEALTHCHECK_PERIOD", 30*time.Second)
	cfg.DB.ConnectTimeout = envDuration("DB_CONNECT_TIMEOUT", 3*time.Second)

	cfg.Ingest.BatchWindow = envDuration("WRITER_BATCH_WINDOW", 500*time.Millisecond)
	cfg.Ingest.MaxBatch = envInt("WRITER_MAX_BATCH", 800)
	cfg.Ingest.QueueSize = envInt("WRITER_QUEUE_SIZE", 50000)

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func validate(cfg Config) error {
	// HTTP
	if cfg.HTTP.Port < 1 || cfg.HTTP.Port > 65535 {
		return fmt.Errorf("PORT must be between 1 and 65535 (got %d)", cfg.HTTP.Port)
	}
	if cfg.HTTP.RequestTimeout <= 0 {
		return fmt.Errorf("REQUEST_TIMEOUT must be > 0 (got %s)", cfg.HTTP.RequestTimeout)
	}

	// DB
	if cfg.DB.MaxConns <= 0 {
		return fmt.Errorf("DB_MAX_CONNS must be > 0 (got %d)", cfg.DB.MaxConns)
	}
	if cfg.DB.MinConns < 0 {
		return fmt.Errorf("DB_MIN_CONNS must be >= 0 (got %d)", cfg.DB.MinConns)
	}
	if cfg.DB.MinConns > cfg.DB.MaxConns {
		return fmt.Errorf("DB_MIN_CONNS must be <= DB_MAX_CONNS (min=%d max=%d)", cfg.DB.MinConns, cfg.DB.MaxConns)
	}
	if cfg.DB.MaxConnIdleTime < 0 {
		return fmt.Errorf("DB_MAX_CONN_IDLE_TIME must be >= 0 (got %s)", cfg.DB.MaxConnIdleTime)
	}
	if cfg.DB.MaxConnLifetime < 0 {
		return fmt.Errorf("DB_MAX_CONN_LIFETIME must be >= 0 (got %s)", cfg.DB.MaxConnLifetime)
	}
	if cfg.DB.HealthCheckPeriod <= 0 {
		return fmt.Errorf("DB_HEALTHCHECK_PERIOD must be > 0 (got %s)", cfg.DB.HealthCheckPeriod)
	}
	if cfg.DB.ConnectTimeout <= 0 {
		return fmt.Errorf("DB_CONNECT_TIMEOUT must be > 0 (got %s)", cfg.DB.ConnectTimeout)
	}

	// Ingest
	if cfg.Ingest.BatchWindow <= 0 {
		return fmt.Errorf("WRITER_BATCH_WINDOW must be > 0 (got %s)", cfg.Ingest.BatchWindow)
	}
	if cfg.Ingest.MaxBatch <= 0 {
		return fmt.Errorf("WRITER_MAX_BATCH must be > 0 (got %d)", cfg.Ingest.MaxBatch)
	}
	if cfg.Ingest.QueueSize <= 0 {
		return fmt.Errorf("WRITER_QUEUE_SIZE must be > 0 (got %d)", cfg.Ingest.QueueSize)
	}
	return nil
}

// it panics if the value is set but invalid
func envInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		panic(fmt.Sprintf("%s must be an integer (got %q)", key, val))
	}
	return n
}

// TODO: Add validation for the duration
// e.g. 200ms", "2s", "1m"
func envDuration(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		panic(fmt.Sprintf("%s must be a valid duration (e.g. 200ms, 2s, 1m). got %q", key, val))
	}
	return d
}
