package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/makkenzo/license-service-api/internal/domain/license"
	"github.com/makkenzo/license-service-api/internal/handler/dto"
	"go.uber.org/zap"
)

type LicenseService struct {
	repo   license.Repository
	logger *zap.Logger
}

func NewLicenseService(repo license.Repository, logger *zap.Logger) *LicenseService {
	return &LicenseService{
		repo:   repo,
		logger: logger.Named("LicenseService"),
	}
}

func (s *LicenseService) CreateLicense(ctx context.Context, req *dto.CreateLicenseRequest) (*license.License, error) {
	s.logger.Info("Attempting to create a new license", zap.String("product", req.ProductName), zap.Any("type", req.Type))

	licenseKey := uuid.NewString()

	newLicense := &license.License{
		LicenseKey:  licenseKey,
		Type:        req.Type,
		ProductName: req.ProductName,
		Metadata:    req.Metadata,
	}

	if req.InitialStatus != nil {

		newLicense.Status = *req.InitialStatus
	} else {

		newLicense.Status = license.StatusActive
	}

	if newLicense.Status == license.StatusActive {
		now := time.Now()
		newLicense.IssuedAt = sql.NullTime{Time: now, Valid: true}
	}

	if req.CustomerName != nil {
		newLicense.CustomerName = sql.NullString{String: *req.CustomerName, Valid: true}
	}
	if req.CustomerEmail != nil {
		newLicense.CustomerEmail = sql.NullString{String: *req.CustomerEmail, Valid: true}
	}
	if req.ExpiresAt != nil {
		newLicense.ExpiresAt = sql.NullTime{Time: *req.ExpiresAt, Valid: true}
	}

	insertedID, err := s.repo.Create(ctx, newLicense)
	if err != nil {

		s.logger.Error("Failed to create license via repository", zap.Error(err))

		return nil, fmt.Errorf("repository error during license creation: %w", err)
	}

	createdLicense, err := s.repo.FindByID(ctx, insertedID)
	if err != nil {
		s.logger.Error("Failed to find newly created license by ID", zap.String("id", insertedID.String()), zap.Error(err))

		return nil, fmt.Errorf("failed to retrieve created license (id: %s): %w", insertedID, err)
	}

	s.logger.Info("License created successfully", zap.String("id", createdLicense.ID.String()), zap.String("key", createdLicense.LicenseKey))
	return createdLicense, nil
}

func (s *LicenseService) ListLicenses(ctx context.Context, req *dto.ListLicensesRequest) ([]*license.License, int64, error) {
	params := license.ListParams{
		Status:        req.Status,
		CustomerEmail: req.CustomerEmail,
		ProductName:   req.ProductName,
		Type:          req.Type,
		Limit:         req.Limit,
		Offset:        req.Offset,
		SortBy:        req.SortBy,
		SortOrder:     req.SortOrder,
	}

	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 20
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	s.logger.Debug("Listing licenses with params", zap.Any("params", params))

	licenses, totalCount, err := s.repo.List(ctx, params)
	if err != nil {
		s.logger.Error("Failed to list licenses via repository", zap.Error(err))
		return nil, 0, fmt.Errorf("repository error during license listing: %w", err)
	}

	s.logger.Info("Licenses listed successfully", zap.Int("count", len(licenses)), zap.Int64("total", totalCount))
	return licenses, totalCount, nil
}
