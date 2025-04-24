package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/makkenzo/license-service-api/internal/domain/license"
	"github.com/makkenzo/license-service-api/internal/handler/dto"
	"github.com/makkenzo/license-service-api/internal/ierr"
	"go.uber.org/zap"
)

const defaultExpiringPeriodDays = 30

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

func (s *LicenseService) GetLicenseByID(ctx context.Context, id uuid.UUID) (*license.License, error) {
	s.logger.Debug("Attempting to get license by ID", zap.String("id", id.String()))

	lic, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ierr.ErrNotFound) {
			s.logger.Info("License not found by ID", zap.String("id", id.String()))
			return nil, ierr.ErrNotFound
		}
		s.logger.Error("Failed to get license by ID from repository", zap.String("id", id.String()), zap.Error(err))
		return nil, fmt.Errorf("repository error fetching license by ID %s: %w", id, err)
	}
	s.logger.Info("License retrieved successfully by ID", zap.String("id", id.String()))
	return lic, nil
}

func (s *LicenseService) UpdateLicenseStatus(ctx context.Context, id uuid.UUID, newStatus license.LicenseStatus) error {
	s.logger.Info("Attempting to update license status",
		zap.String("id", id.String()),
		zap.String("new_status", string(newStatus)),
	)

	err := s.repo.UpdateStatus(ctx, id, newStatus)
	if err != nil {

		if errors.Is(err, ierr.ErrNotFound) || errors.Is(err, ierr.ErrUpdateFailed) {
			return err
		}

		return fmt.Errorf("repository error updating status for license %s: %w", id, err)
	}

	s.logger.Info("License status update successful in service",
		zap.String("id", id.String()),
		zap.String("new_status", string(newStatus)),
	)

	return nil
}

func (s *LicenseService) UpdateLicense(ctx context.Context, id uuid.UUID, req *dto.UpdateLicenseRequest) (*license.License, error) {
	s.logger.Debug("Attempting to update license", zap.String("id", id.String()))

	currentLicense, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, ierr.ErrNotFound) || errors.Is(err, pgx.ErrNoRows) {
			s.logger.Warn("License not found for update", zap.String("id", id.String()))
			return nil, ierr.ErrNotFound
		}
		s.logger.Error("Failed to get current license for update", zap.String("id", id.String()), zap.Error(err))
		return nil, fmt.Errorf("repository error fetching license %s for update: %w", id, err)
	}

	updated := false

	if req.Type != nil && currentLicense.Type != *req.Type {
		currentLicense.Type = *req.Type
		updated = true
	}
	if req.ProductName != nil && currentLicense.ProductName != *req.ProductName {
		currentLicense.ProductName = *req.ProductName
		updated = true
	}

	if req.CustomerName != nil {

		if !currentLicense.CustomerName.Valid || currentLicense.CustomerName.String != *req.CustomerName {
			currentLicense.CustomerName = sql.NullString{String: *req.CustomerName, Valid: true}
			updated = true
		}
	}
	if req.CustomerEmail != nil {
		if !currentLicense.CustomerEmail.Valid || currentLicense.CustomerEmail.String != *req.CustomerEmail {
			currentLicense.CustomerEmail = sql.NullString{String: *req.CustomerEmail, Valid: true}
			updated = true
		}
	}
	if req.ExpiresAt != nil {
		if !currentLicense.ExpiresAt.Valid || !currentLicense.ExpiresAt.Time.Equal(*req.ExpiresAt) {
			currentLicense.ExpiresAt = sql.NullTime{Time: *req.ExpiresAt, Valid: true}
			updated = true
		}
	}

	if req.Metadata != nil {

		currentLicense.Metadata = req.Metadata
		updated = true
	}

	if !updated {
		s.logger.Info("No fields to update for license", zap.String("id", id.String()))
		return currentLicense, nil
	}

	err = s.repo.Update(ctx, currentLicense)
	if err != nil {

		s.logger.Error("Repository failed to update license", zap.String("id", id.String()), zap.Error(err))
		return nil, fmt.Errorf("repository error updating license %s: %w", id, err)
	}

	s.logger.Info("License updated successfully in service", zap.String("id", id.String()))
	return currentLicense, nil
}

type ValidationResult struct {
	IsValid      bool
	Reason       string
	License      *license.License
	ResponseData json.RawMessage
}

const (
	MetaKeyDeviceID        = "device_id"
	MetaKeyUserID          = "user_id"
	MetaKeyIPAddress       = "ip_address"
	MetaKeyLastValidatedAt = "last_validated_at"
	MetaKeyFeatures        = "features"
	MetaKeyLimits          = "limits"
)

func (s *LicenseService) ValidateLicense(ctx context.Context, req *dto.ValidateLicenseRequest) (*ValidationResult, error) {
	s.logger.Info("Attempting to validate license key",
		zap.String("license_key", req.LicenseKey),
		zap.String("product_name", req.ProductName),
	)

	result := &ValidationResult{IsValid: false}

	lic, err := s.repo.FindByKey(ctx, req.LicenseKey)
	if err != nil {
		if errors.Is(err, ierr.ErrNotFound) || errors.Is(err, pgx.ErrNoRows) {
			s.logger.Info("License key not found during validation", zap.String("license_key", req.LicenseKey))
			result.Reason = "not_found"
			return result, nil
		}

		s.logger.Error("Repository error finding license by key during validation", zap.String("license_key", req.LicenseKey), zap.Error(err))
		return nil, fmt.Errorf("repository error validating key %s: %w", req.LicenseKey, err)
	}

	result.License = lic

	if lic.ProductName != req.ProductName {
		s.logger.Warn("License product mismatch during validation",
			zap.String("license_key", req.LicenseKey),
			zap.String("expected_product", req.ProductName),
			zap.String("actual_product", lic.ProductName),
		)
		result.Reason = "product_mismatch"
		return result, nil
	}

	if lic.Status != license.StatusActive {
		s.logger.Info("License has non-active status during validation",
			zap.String("license_key", req.LicenseKey),
			zap.String("status", string(lic.Status)),
		)
		result.Reason = string(lic.Status)

		if lic.Status == license.StatusExpired {
			result.Reason = "expired"
		}
		return result, nil
	}

	now := time.Now().UTC()
	if lic.ExpiresAt.Valid && now.After(lic.ExpiresAt.Time.UTC()) {
		s.logger.Info("License has expired (date check)",
			zap.String("license_key", req.LicenseKey),
			zap.Time("expires_at", lic.ExpiresAt.Time),
		)
		result.Reason = "expired"

		go func(lId uuid.UUID, r license.Repository, l *zap.Logger) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			l.Info("Attempting background status update to expired", zap.String("license_id", lId.String()))
			if err := r.UpdateStatus(bgCtx, lId, license.StatusExpired); err != nil {
				l.Error("Background status update to expired failed", zap.String("license_id", lId.String()), zap.Error(err))
			}
		}(lic.ID, s.repo, s.logger)

		return result, nil
	}

	var agentMeta map[string]interface{}
	var licenseMeta map[string]interface{}
	agentMetaValid := req.Metadata != nil && json.Unmarshal(req.Metadata, &agentMeta) == nil
	licenseMetaValid := lic.Metadata != nil && json.Unmarshal(lic.Metadata, &licenseMeta) == nil

	if licenseMetaValid {
		licenseDeviceID, hasDeviceBinding := licenseMeta[MetaKeyDeviceID].(string)
		licenseUserID, hasUserBinding := licenseMeta[MetaKeyUserID].(string)

		if hasDeviceBinding && licenseDeviceID != "" {
			if !agentMetaValid {
				s.logger.Warn("Device ID required but not provided by agent", zap.String("license_key", req.LicenseKey))
				result.Reason = "device_id_required"
				return result, nil
			}
			agentDeviceID, agentHasDeviceID := agentMeta[MetaKeyDeviceID].(string)
			if !agentHasDeviceID || agentDeviceID == "" {
				s.logger.Warn("Device ID required but empty in agent request", zap.String("license_key", req.LicenseKey))
				result.Reason = "device_id_required"
				return result, nil
			}
			if agentDeviceID != licenseDeviceID {
				s.logger.Warn("Device ID mismatch",
					zap.String("license_key", req.LicenseKey),
					zap.String("agent_device", agentDeviceID),
					zap.String("license_device", licenseDeviceID),
				)
				result.Reason = "device_id_mismatch"
				return result, nil
			}
		}

		if hasUserBinding && licenseUserID != "" {
			if !agentMetaValid {
				s.logger.Warn("User ID required but not provided by agent", zap.String("license_key", req.LicenseKey))
				result.Reason = "user_id_required"
				return result, nil
			}

			agentUserID, agentHasUserID := agentMeta[MetaKeyUserID].(string)

			if !agentHasUserID || agentUserID == "" {
				s.logger.Warn("User ID required but empty in agent request", zap.String("license_key", req.LicenseKey))
				result.Reason = "user_id_required"
				return result, nil
			}

			if agentUserID != licenseUserID {
				s.logger.Warn("User ID mismatch",
					zap.String("license_key", req.LicenseKey),
					zap.String("agent_user", agentUserID),
					zap.String("license_user", licenseUserID),
				)
				result.Reason = "user_id_mismatch"
				return result, nil
			}
		}
	}

	s.logger.Info("License validation successful", zap.String("license_key", req.LicenseKey))
	result.IsValid = true
	result.Reason = "valid"

	if licenseMetaValid {
		allowedDataMap := make(map[string]interface{})
		if features, ok := licenseMeta[MetaKeyFeatures]; ok {
			allowedDataMap[MetaKeyFeatures] = features
		}
		if limits, ok := licenseMeta[MetaKeyLimits]; ok {
			allowedDataMap[MetaKeyLimits] = limits
		}

		if len(allowedDataMap) > 0 {
			allowedBytes, errJson := json.Marshal(allowedDataMap)
			if errJson == nil {
				result.ResponseData = allowedBytes
			} else {
				s.logger.Error("Failed to marshal allowed_data", zap.String("license_key", req.LicenseKey), zap.Error(errJson))
			}
		}
	}

	updateData := make(map[string]interface{})
	updateData[MetaKeyLastValidatedAt] = now

	if agentMetaValid {
		if agentDeviceID, ok := agentMeta[MetaKeyDeviceID].(string); ok && agentDeviceID != "" {
		}
		if agentIP, ok := agentMeta[MetaKeyIPAddress].(string); ok && agentIP != "" {
			updateData["last_ip"] = agentIP
		}
	}

	if len(updateData) > 0 {
		go func(lId uuid.UUID, currentMeta []byte, dataToUpdate map[string]interface{}, r license.Repository, l *zap.Logger) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			l.Debug("Attempting background metadata update", zap.String("license_id", lId.String()))

			mergedMetaMap := make(map[string]interface{})
			if currentMeta != nil {
				_ = json.Unmarshal(currentMeta, &mergedMetaMap)
			}
			for k, v := range dataToUpdate {
				mergedMetaMap[k] = v
			}

			newMetaBytes, errMarshal := json.Marshal(mergedMetaMap)
			if errMarshal != nil {
				l.Error("Failed to marshal metadata for background update", zap.String("license_id", lId.String()), zap.Error(errMarshal))
				return
			}

			if bytes.Equal(currentMeta, newMetaBytes) {
				l.Debug("Metadata hasn't changed, skipping background update", zap.String("license_id", lId.String()))
				return
			}

			if err := r.UpdateMetadata(bgCtx, lId, newMetaBytes); err != nil {
				l.Error("Background metadata update failed", zap.String("license_id", lId.String()), zap.Error(err))
			} else {
				l.Info("Background metadata update successful", zap.String("license_id", lId.String()))
			}

		}(lic.ID, lic.Metadata, updateData, s.repo, s.logger)
	}

	return result, nil
}

func (s *LicenseService) GetDashboardSummary(ctx context.Context) (*dto.DashboardSummaryResponse, error) {
	s.logger.Info("Requesting dashboard summary data")

	summaryData, err := s.repo.GetDashboardSummary(ctx, defaultExpiringPeriodDays)
	if err != nil {
		s.logger.Error("Failed to get dashboard summary from repository", zap.Error(err))
		return nil, fmt.Errorf("repository error fetching dashboard summary: %w", err)
	}

	response := &dto.DashboardSummaryResponse{
		TotalLicenses: summaryData.TotalCount,
		StatusCounts:  summaryData.StatusCounts,
		TypeCounts:    summaryData.TypeCounts,
		ProductCounts: summaryData.ProductCounts,
		ExpiringSoon: dto.ExpiringSoonSummary{
			Count:      summaryData.ExpiringSoonCount,
			PeriodDays: defaultExpiringPeriodDays,
		},
	}

	if summaryData.NextToExpireKey != nil && summaryData.NextToExpireDate != nil && summaryData.NextToExpireProd != nil {
		response.ExpiringSoon.NextToExpire = &dto.LicenseInfo{
			LicenseKey:  *summaryData.NextToExpireKey,
			ExpiresAt:   *summaryData.NextToExpireDate,
			ProductName: *summaryData.NextToExpireProd,
		}
	}

	s.logger.Info("Dashboard summary prepared successfully")
	return response, nil
}

func CheckAndExpireLicenses(ctx context.Context, repo license.Repository, logger *zap.Logger) (int, error) {
	log := logger.Named("StartupExpireCheck")
	log.Info("Starting initial check for expired licenses...")

	now := time.Now().UTC()
	updatedCount := 0
	offset := 0
	limit := 500

	for {

		params := license.ListParams{
			Status:    ptr(license.StatusActive),
			Limit:     limit,
			Offset:    offset,
			SortBy:    "id",
			SortOrder: "ASC",
		}
		activeLicenses, _, err := repo.List(ctx, params)
		if err != nil {
			log.Error("Failed to list active licenses during startup check", zap.Error(err))
			return updatedCount, fmt.Errorf("repository error listing active licenses: %w", err)
		}

		if len(activeLicenses) == 0 {
			log.Debug("No more active licenses found to check.")
			break
		}

		foundExpiredInBatch := 0
		for _, lic := range activeLicenses {
			if lic.ExpiresAt.Valid && lic.ExpiresAt.Time.UTC().Before(now) {
				log.Info("Found expired license during startup check, updating status",
					zap.String("license_id", lic.ID.String()),
					zap.Time("expires_at", lic.ExpiresAt.Time),
				)
				errUpdate := repo.UpdateStatus(ctx, lic.ID, license.StatusExpired)
				if errUpdate != nil {
					log.Error("Failed to update status for expired license during startup",
						zap.String("license_id", lic.ID.String()),
						zap.Error(errUpdate),
					)
				} else {
					updatedCount++
					foundExpiredInBatch++
				}
			}
		}
		log.Debug("Checked batch for expiration", zap.Int("batch_size", len(activeLicenses)), zap.Int("found_expired", foundExpiredInBatch), zap.Int("offset", offset))

		if int64(len(activeLicenses)) < int64(limit) {
			break
		}

		offset += limit
	}

	log.Info("Initial check for expired licenses finished.", zap.Int("total_updated", updatedCount))
	return updatedCount, nil
}

func ptr[T any](v T) *T {
	return &v
}
