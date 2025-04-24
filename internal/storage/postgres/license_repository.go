package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/makkenzo/license-service-api/internal/domain/license"
	"github.com/makkenzo/license-service-api/internal/ierr"
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

func (r *LicenseRepository) List(ctx context.Context, params license.ListParams) ([]*license.License, int64, error) {
	var baseQuery strings.Builder
	var countQuery strings.Builder
	args := make([]interface{}, 0, 5)
	paramIndex := 1

	baseQuery.WriteString(`
        SELECT
            id, license_key, status, type, customer_name, customer_email,
            product_name, metadata, issued_at, expires_at, created_at, updated_at
        FROM licenses
    `)

	countQuery.WriteString(`SELECT COUNT(*) FROM licenses`)

	whereClause := strings.Builder{}

	addWhereCondition := func(condition string, value interface{}) {
		if whereClause.Len() == 0 {
			whereClause.WriteString(" WHERE ")
		} else {
			whereClause.WriteString(" AND ")
		}
		whereClause.WriteString(fmt.Sprintf("%s = $%d", condition, paramIndex))
		args = append(args, value)
		paramIndex++
	}

	if params.Status != nil {
		addWhereCondition("status", *params.Status)
	}
	if params.CustomerEmail != nil {
		addWhereCondition("customer_email", *params.CustomerEmail)
	}
	if params.ProductName != nil {
		addWhereCondition("product_name", *params.ProductName)
	}
	if params.Type != nil {
		addWhereCondition("type", *params.Type)
	}

	if whereClause.Len() > 0 {
		baseQuery.WriteString(whereClause.String())
		countQuery.WriteString(whereClause.String())
	}

	var totalCount int64
	countSQL := countQuery.String()
	r.logger.Debug("Executing count query", zap.String("sql", countSQL), zap.Any("args", args))
	err := r.db.QueryRow(ctx, countSQL, args...).Scan(&totalCount)
	if err != nil {
		r.logger.Error("Failed to execute count query for licenses", zap.Error(err))
		return nil, 0, fmt.Errorf("database error on count licenses: %w", err)
	}

	if totalCount == 0 {
		return []*license.License{}, 0, nil
	}

	orderByClause, err := r.buildOrderBy(params.SortBy, params.SortOrder)
	if err != nil {
		r.logger.Warn("Invalid sort parameters", zap.Error(err))

		orderByClause = " ORDER BY created_at DESC"
	}
	baseQuery.WriteString(orderByClause)

	baseQuery.WriteString(fmt.Sprintf(" LIMIT $%d", paramIndex))
	args = append(args, params.Limit)
	paramIndex++

	baseQuery.WriteString(fmt.Sprintf(" OFFSET $%d", paramIndex))
	args = append(args, params.Offset)

	listSQL := baseQuery.String()
	r.logger.Debug("Executing list query", zap.String("sql", listSQL), zap.Any("args", args))
	rows, err := r.db.Query(ctx, listSQL, args...)
	if err != nil {
		r.logger.Error("Failed to query list of licenses", zap.Error(err))
		return nil, 0, fmt.Errorf("database error on list licenses: %w", err)
	}
	defer rows.Close()

	licenses := make([]*license.License, 0, params.Limit)

	for rows.Next() {
		var lic license.License
		err := rows.Scan(
			&lic.ID, &lic.LicenseKey, &lic.Status, &lic.Type, &lic.CustomerName,
			&lic.CustomerEmail, &lic.ProductName, &lic.Metadata, &lic.IssuedAt,
			&lic.ExpiresAt, &lic.CreatedAt, &lic.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan license row during list", zap.Error(err))
			return nil, 0, fmt.Errorf("database scan error during list: %w", err)
		}
		licenses = append(licenses, &lic)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating license rows", zap.Error(err))
		return nil, 0, fmt.Errorf("database iteration error on list licenses: %w", err)
	}

	return licenses, totalCount, nil
}

func (r *LicenseRepository) buildOrderBy(sortBy, sortOrder string) (string, error) {
	allowedSortBy := map[string]string{
		"id":             "id",
		"created_at":     "created_at",
		"expires_at":     "expires_at",
		"issued_at":      "issued_at",
		"updated_at":     "updated_at",
		"customer_name":  "customer_name",
		"customer_email": "customer_email",
		"product_name":   "product_name",
		"type":           "type",
		"status":         "status",
	}

	dbColumn, ok := allowedSortBy[strings.ToLower(sortBy)]
	if !ok {
		return "", fmt.Errorf("invalid sort_by field: %s", sortBy)
	}

	order := strings.ToUpper(sortOrder)
	if order != "ASC" && order != "DESC" {
		return "", fmt.Errorf("invalid sort_order: %s", sortOrder)
	}

	nullsPlacement := ""
	if dbColumn == "expires_at" || dbColumn == "issued_at" || dbColumn == "customer_name" || dbColumn == "customer_email" {
		if order == "ASC" {
			nullsPlacement = " NULLS FIRST"
		} else {
			nullsPlacement = " NULLS LAST"
		}
	}

	return fmt.Sprintf(" ORDER BY %s %s%s", dbColumn, order, nullsPlacement), nil
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

func (r *LicenseRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status license.LicenseStatus) error {
	query := `UPDATE licenses SET status = $1 WHERE id = $2`

	cmdTag, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		r.logger.Error("Failed to update license status in database",
			zap.String("id", id.String()),
			zap.String("new_status", string(status)),
			zap.Error(err),
		)

		return fmt.Errorf("%w: error updating status for license %s: %v", ierr.ErrUpdateFailed, id, err)
	}

	if cmdTag.RowsAffected() == 0 {
		r.logger.Warn("Attempted to update status, but license was not found",
			zap.String("id", id.String()),
			zap.String("new_status", string(status)),
		)
		return ierr.ErrNotFound
	}

	r.logger.Info("License status updated successfully",
		zap.String("id", id.String()),
		zap.String("new_status", string(status)),
	)
	return nil
}

func (r *LicenseRepository) GetDashboardSummary(ctx context.Context, expiringPeriodDays int) (*license.DashboardSummaryData, error) {
	summary := &license.DashboardSummaryData{
		StatusCounts:  make(map[license.LicenseStatus]int64),
		TypeCounts:    make(map[string]int64),
		ProductCounts: make(map[string]int64),
	}
	var err error

	dbExecutor := r.db

	err = dbExecutor.QueryRow(ctx, "SELECT COUNT(*) FROM licenses").Scan(&summary.TotalCount)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		r.logger.Error("Failed to get total license count", zap.Error(err))
		return nil, fmt.Errorf("db error counting total licenses: %w", err)
	}

	rowsStatus, err := dbExecutor.Query(ctx, "SELECT status, COUNT(*) FROM licenses GROUP BY status")
	if err != nil {
		r.logger.Error("Failed to get license counts by status", zap.Error(err))
		return nil, fmt.Errorf("db error counting by status: %w", err)
	}
	for rowsStatus.Next() {
		var status license.LicenseStatus
		var count int64
		if err := rowsStatus.Scan(&status, &count); err != nil {
			rowsStatus.Close()
			r.logger.Error("Failed to scan status count row", zap.Error(err))
			return nil, fmt.Errorf("db scan error for status counts: %w", err)
		}
		summary.StatusCounts[status] = count
	}
	rowsStatus.Close()
	if err = rowsStatus.Err(); err != nil {
		r.logger.Error("Error iterating status counts", zap.Error(err))
		return nil, fmt.Errorf("db iteration error for status counts: %w", err)
	}

	rowsType, err := dbExecutor.Query(ctx, "SELECT type, COUNT(*) FROM licenses GROUP BY type")
	if err != nil {
		r.logger.Error("Failed to get license counts by type", zap.Error(err))
		return nil, fmt.Errorf("db error counting by type: %w", err)
	}
	for rowsType.Next() {
		var licType string
		var count int64
		if err := rowsType.Scan(&licType, &count); err != nil {
			rowsType.Close()
			r.logger.Error("Failed to scan type count row", zap.Error(err))
			return nil, fmt.Errorf("db scan error for type counts: %w", err)
		}
		summary.TypeCounts[licType] = count
	}
	rowsType.Close()
	if err = rowsType.Err(); err != nil {
		r.logger.Error("Error iterating type counts", zap.Error(err))
		return nil, fmt.Errorf("db iteration error for type counts: %w", err)
	}

	rowsProd, err := dbExecutor.Query(ctx, "SELECT product_name, COUNT(*) FROM licenses GROUP BY product_name")
	if err != nil {
		r.logger.Error("Failed to get license counts by product", zap.Error(err))
		return nil, fmt.Errorf("db error counting by product: %w", err)
	}
	for rowsProd.Next() {
		var prodName string
		var count int64
		if err := rowsProd.Scan(&prodName, &count); err != nil {
			rowsProd.Close()
			r.logger.Error("Failed to scan product count row", zap.Error(err))
			return nil, fmt.Errorf("db scan error for product counts: %w", err)
		}
		summary.ProductCounts[prodName] = count
	}
	rowsProd.Close()
	if err = rowsProd.Err(); err != nil {
		r.logger.Error("Error iterating product counts", zap.Error(err))
		return nil, fmt.Errorf("db iteration error for product counts: %w", err)
	}

	now := time.Now().UTC()
	expiresSoonDate := now.AddDate(0, 0, expiringPeriodDays)

	queryExpiringCount := `
		SELECT COUNT(*) FROM licenses
		WHERE status = $1 AND expires_at IS NOT NULL AND expires_at > $2 AND expires_at <= $3
	`
	err = dbExecutor.QueryRow(ctx, queryExpiringCount, license.StatusActive, now, expiresSoonDate).Scan(&summary.ExpiringSoonCount)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		r.logger.Error("Failed to get expiring soon count", zap.Error(err))
		return nil, fmt.Errorf("db error counting expiring licenses: %w", err)
	}

	queryNextToExpire := `
		SELECT license_key, expires_at, product_name FROM licenses
		WHERE status = $1 AND expires_at IS NOT NULL AND expires_at > $2
		ORDER BY expires_at ASC
		LIMIT 1
	`
	var nextKey sql.NullString
	var nextDate sql.NullTime
	var nextProd sql.NullString
	err = dbExecutor.QueryRow(ctx, queryNextToExpire, license.StatusActive, now).Scan(&nextKey, &nextDate, &nextProd)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		r.logger.Error("Failed to get next expiring license", zap.Error(err))
		return nil, fmt.Errorf("db error finding next expiring license: %w", err)
	}

	if nextKey.Valid {
		summary.NextToExpireKey = &nextKey.String
	}
	if nextDate.Valid {
		summary.NextToExpireDate = &nextDate.Time
	}
	if nextProd.Valid {
		summary.NextToExpireProd = &nextProd.String
	}

	r.logger.Info("Dashboard summary data retrieved successfully")
	return summary, nil
}
