package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/makkenzo/license-service-api/internal/domain/apikey"
	"go.uber.org/zap"
)

type APIKeyRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewAPIKeyRepository(db *pgxpool.Pool, logger *zap.Logger) *APIKeyRepository {
	return &APIKeyRepository{
		db:     db,
		logger: logger.Named("APIKeyRepository"),
	}
}

var _ apikey.Repository = (*APIKeyRepository)(nil)

func (r *APIKeyRepository) FindByPrefix(ctx context.Context, prefix string) (*apikey.APIKey, error) {
	query := `
		SELECT id, key_hash, prefix, description, product_id, is_enabled, created_at, last_used_at
		FROM api_keys
		WHERE prefix = $1 AND is_enabled = TRUE
	`
	row := r.db.QueryRow(ctx, query, prefix)

	var key apikey.APIKey
	var productID sql.Null[uuid.UUID]
	var lastUsed sql.NullTime

	err := row.Scan(
		&key.ID,
		&key.KeyHash,
		&key.Prefix,
		&key.Description,
		&productID,
		&key.IsEnabled,
		&key.CreatedAt,
		&lastUsed,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Debug("API key not found or disabled by prefix", zap.String("prefix", prefix))
			return nil, apikey.ErrAPIKeyNotFound
		}
		r.logger.Error("Failed to find api key by prefix", zap.String("prefix", prefix), zap.Error(err))
		return nil, fmt.Errorf("db error finding api key: %w", err)
	}

	if productID.Valid {
		key.ProductID = productID.V
	}
	if lastUsed.Valid {
		key.LastUsedAt = &lastUsed.Time
	}

	return &key, nil
}

func (r *APIKeyRepository) Create(ctx context.Context, key *apikey.APIKey) (uuid.UUID, error) {
	query := `
		INSERT INTO api_keys (key_hash, prefix, description, product_id, is_enabled)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	var insertedID uuid.UUID
	var productIDArg interface{}

	if key.ProductID != uuid.Nil {
		productIDArg = key.ProductID
	} else {
		productIDArg = nil
	}

	err := r.db.QueryRow(ctx, query,
		key.KeyHash,
		key.Prefix,
		key.Description,
		productIDArg,
		key.IsEnabled,
	).Scan(&insertedID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {

			constraint := pgErr.ConstraintName
			r.logger.Warn("Failed to create API key due to unique constraint violation",
				zap.String("constraint", constraint),
				zap.String("prefix", key.Prefix),
			)
			return uuid.Nil, fmt.Errorf("api key constraint violation (%s)", constraint)
		}
		r.logger.Error("Failed to create api key in database", zap.Error(err))
		return uuid.Nil, fmt.Errorf("db error creating api key: %w", err)
	}

	r.logger.Info("API key created successfully", zap.String("id", insertedID.String()), zap.String("prefix", key.Prefix))
	return insertedID, nil
}

func (r *APIKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID, lastUsed time.Time) error {
	query := `UPDATE api_keys SET last_used_at = $1 WHERE id = $2`
	cmdTag, err := r.db.Exec(ctx, query, lastUsed, id)
	if err != nil {
		r.logger.Error("Failed to update api key last_used_at", zap.String("id", id.String()), zap.Error(err))
		return fmt.Errorf("db error updating last used time: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {

		r.logger.Warn("API key not found when updating last_used_at", zap.String("id", id.String()))

	}
	return nil
}
