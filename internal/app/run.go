package app

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/cun0/insider-case/internal/config"
	"github.com/cun0/insider-case/internal/httpserver"
	"github.com/cun0/insider-case/internal/ingest"
	"github.com/cun0/insider-case/internal/jsonlog"
	"github.com/cun0/insider-case/internal/repo"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(version, buildTime string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	level := jsonlog.LevelInfo
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		if parsed, err := jsonlog.ParseLevel(v); err == nil {
			level = parsed
		}
	}

	logger := jsonlog.New(os.Stdout, level)

	pool, err := openPool(cfg)
	if err != nil {
		return err
	}
	// pool.Close() is called in onShutdown to keep the lifecycle in one place.

	eventRepo := repo.NewEventRepo(pool)
	metricsRepo := repo.NewMetricsRepo(pool)

	writer := ingest.NewSingleWriter(eventRepo, ingest.Config{
		BatchWindow: defaultDuration(cfg.Ingest.BatchWindow, 2*time.Millisecond),
		MaxBatch:    defaultInt(cfg.Ingest.MaxBatch, 800),
		QueueSize:   defaultInt(cfg.Ingest.QueueSize, 50_000),
	}, logger)
	_ = writer.Start()

	handler := httpserver.BuildHandler(httpserver.Config{
		RequestTimeout: defaultDuration(cfg.HTTP.RequestTimeout, 3*time.Second),
	}, logger, writer, eventRepo, metricsRepo)

	logger.PrintInfo("service started", map[string]string{
		"version":    version,
		"build_time": buildTime,
	})

	return httpserver.Serve(cfg.HTTP, logger, handler, func(ctx context.Context) error {
		stopErr := writer.Stop(ctx)
		pool.Close()
		if stopErr != nil && !errors.Is(stopErr, context.Canceled) && !errors.Is(stopErr, context.DeadlineExceeded) {
			return stopErr
		}
		return nil
	})
}

func openPool(cfg config.Config) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.DB.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if cfg.DB.MaxConns > 0 {
		pc.MaxConns = int32(cfg.DB.MaxConns)
	}
	if cfg.DB.MinConns > 0 {
		pc.MinConns = int32(cfg.DB.MinConns)
	}
	if cfg.DB.MaxConnIdleTime > 0 {
		pc.MaxConnIdleTime = cfg.DB.MaxConnIdleTime
	}
	if cfg.DB.MaxConnLifetime > 0 {
		pc.MaxConnLifetime = cfg.DB.MaxConnLifetime
	}
	if cfg.DB.HealthCheckPeriod > 0 {
		pc.HealthCheckPeriod = cfg.DB.HealthCheckPeriod
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), pc)
	if err != nil {
		return nil, err
	}

	to := defaultDuration(cfg.DB.ConnectTimeout, 5*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), to)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

func defaultDuration(v, def time.Duration) time.Duration {
	if v <= 0 {
		return def
	}
	return v
}

func defaultInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
