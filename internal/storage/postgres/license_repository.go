package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/makkenzo/license-service-api/internal/domain/license"
	"go.uber.org/zap"
)

type LicenseRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewLicenseRepository(db *pgxpool.Pool, logger *zap.Logger) *LicenseRepository {
	return &LicenseRepository{
		db:     db,
		logger: logger.Named("LicenseRepository"),
	}
}

var _ license.Repository = (*LicenseRepository)(nil)

func (r *LicenseRepository) Create(ctx context.Context, lic *license.License) (uuid.UUID, error) {

	query := `
        INSERT INTO licenses (
            license_key, status, type, customer_name, customer_email,
            product_name, metadata, issued_at, expires_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9
        ) RETURNING id
    `
	var insertedID uuid.UUID

	err := r.db.QueryRow(ctx, query,
		lic.LicenseKey,
		lic.Status,
		lic.Type,
		lic.CustomerName,
		lic.CustomerEmail,
		lic.ProductName,
		lic.Metadata,
		lic.IssuedAt,
		lic.ExpiresAt,
	).Scan(&insertedID)

	if err != nil {

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {

			r.logger.Warn("Attempted to create license with duplicate key",
				zap.String("license_key", lic.LicenseKey),
				zap.String("constraint", pgErr.ConstraintName),
			)

			return uuid.Nil, fmt.Errorf("license key '%s' already exists", lic.LicenseKey)
		}

		r.logger.Error("Failed to create license in database", zap.Error(err))
		return uuid.Nil, fmt.Errorf("database error on create license: %w", err)
	}

	r.logger.Info("License created successfully", zap.String("id", insertedID.String()))
	return insertedID, nil
}

func (r *LicenseRepository) FindByID(ctx context.Context, id uuid.UUID) (*license.License, error) {
	query := `
        SELECT
            id, license_key, status, type, customer_name, customer_email,
            product_name, metadata, issued_at, expires_at, created_at, updated_at
        FROM licenses
        WHERE id = $1
    `

	row := r.db.QueryRow(ctx, query, id)
	return r.scanLicense(row)
}

func (r *LicenseRepository) FindByKey(ctx context.Context, key string) (*license.License, error) {
	query := `
        SELECT
            id, license_key, status, type, customer_name, customer_email,
            product_name, metadata, issued_at, expires_at, created_at, updated_at
        FROM licenses
        WHERE license_key = $1
    `

	row := r.db.QueryRow(ctx, query, key)
	return r.scanLicense(row)
}

func (r *LicenseRepository) List(ctx context.Context /*, params ListParams*/) ([]*license.License, error) {

	query := `
        SELECT
            id, license_key, status, type, customer_name, customer_email,
            product_name, metadata, issued_at, expires_at, created_at, updated_at
        FROM licenses
        ORDER BY created_at DESC -- Сортируем по умолчанию по дате создания
        -- LIMIT $1 OFFSET $2 -- Добавить позже для пагинации
    `
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.logger.Error("Failed to query list of licenses", zap.Error(err))
		return nil, fmt.Errorf("database error on list licenses: %w", err)
	}
	defer rows.Close()

	licenses := make([]*license.License, 0)

	for rows.Next() {
		var lic license.License

		err := rows.Scan(
			&lic.ID,
			&lic.LicenseKey,
			&lic.Status,
			&lic.Type,
			&lic.CustomerName,
			&lic.CustomerEmail,
			&lic.ProductName,
			&lic.Metadata,
			&lic.IssuedAt,
			&lic.ExpiresAt,
			&lic.CreatedAt,
			&lic.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan license row during list", zap.Error(err))

			return nil, fmt.Errorf("database scan error during list: %w", err)
		}
		licenses = append(licenses, &lic)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating license rows", zap.Error(err))
		return nil, fmt.Errorf("database iteration error on list licenses: %w", err)
	}

	return licenses, nil
}

func (r *LicenseRepository) Update(ctx context.Context, lic *license.License) error {

	query := `
        UPDATE licenses SET
            status = $1,
            type = $2,
            customer_name = $3,
            customer_email = $4,
            product_name = $5,
            metadata = $6,
            issued_at = $7,
            expires_at = $8
            -- updated_at обновляется триггером
        WHERE id = $9
    `

	cmdTag, err := r.db.Exec(ctx, query,
		lic.Status,
		lic.Type,
		lic.CustomerName,
		lic.CustomerEmail,
		lic.ProductName,
		lic.Metadata,
		lic.IssuedAt,
		lic.ExpiresAt,
		lic.ID,
	)

	if err != nil {
		r.logger.Error("Failed to update license in database", zap.String("id", lic.ID.String()), zap.Error(err))

		return fmt.Errorf("database error on update license: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		r.logger.Warn("Attempted to update license, but no rows were affected (likely not found)", zap.String("id", lic.ID.String()))

		return fmt.Errorf("license with ID %s not found for update", lic.ID)
	}

	r.logger.Info("License updated successfully", zap.String("id", lic.ID.String()))
	return nil
}

func (r *LicenseRepository) scanLicense(row pgx.Row) (*license.License, error) {
	var lic license.License
	err := row.Scan(
		&lic.ID,
		&lic.LicenseKey,
		&lic.Status,
		&lic.Type,
		&lic.CustomerName,
		&lic.CustomerEmail,
		&lic.ProductName,
		&lic.Metadata,
		&lic.IssuedAt,
		&lic.ExpiresAt,
		&lic.CreatedAt,
		&lic.UpdatedAt,
	)

	if err != nil {

		if errors.Is(err, pgx.ErrNoRows) {

			return nil, err
		}

		r.logger.Error("Failed to scan license row", zap.Error(err))
		return nil, fmt.Errorf("database scan error: %w", err)
	}

	return &lic, nil
}
