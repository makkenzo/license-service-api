package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/makkenzo/license-service-api/internal/config"
	"go.uber.org/zap"
)

func NewPgxPool(ctx context.Context, cfg *config.DatabaseConfig, logger *zap.Logger) (*pgxpool.Pool, error) {
	pgxConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres connection string: %w", err)
	}

	pgxConfig.MaxConns = int32(cfg.MaxOpenConns)
	pgxConfig.MinConns = int32(cfg.MaxIdleConns)
	pgxConfig.MaxConnLifetime = cfg.ConnMaxLifetime

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, pgxConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres connection pool: %w", err)
	}

	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	defer cancelPing()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	logger.Info("Successfully connected to PostgreSQL")
	return pool, nil
}
