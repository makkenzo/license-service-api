package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/makkenzo/license-service-api/internal/domain/license"
	"go.uber.org/zap"
)

type LicenseExpireHandler struct {
	repo   license.Repository
	logger *zap.Logger
}

func NewLicenseExpireHandler(repo license.Repository, logger *zap.Logger) *LicenseExpireHandler {
	return &LicenseExpireHandler{
		repo:   repo,
		logger: logger.Named("LicenseExpireHandler"),
	}
}

func (h *LicenseExpireHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {

	if t.Type() != TypeLicenseExpire {
		return fmt.Errorf("unexpected task type: %s", t.Type())
	}

	var p ExpireLicensePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		h.logger.Error("Failed to unmarshal payload for license expiration task", zap.Error(err), zap.ByteString("payload", t.Payload()))

		return fmt.Errorf("invalid payload: %v", err)
	}

	h.logger.Info("Processing license expiration check task...")

	now := time.Now().UTC()
	params := license.ListParams{
		Status:    ptr(license.StatusActive),
		SortBy:    "expires_at",
		SortOrder: "ASC",
		Limit:     1000,
		Offset:    0,
	}

	updatedCount := 0
	processedCount := 0

	for {
		licensesToExpire, total, err := h.repo.List(ctx, params)
		if err != nil {
			h.logger.Error("Failed to list active licenses for expiration check", zap.Error(err))
			return fmt.Errorf("repository error listing active licenses: %w", err)
		}

		if len(licensesToExpire) == 0 {
			h.logger.Debug("No more active licenses found to check for expiration.")
			break
		}

		processedCount += len(licensesToExpire)

		for _, lic := range licensesToExpire {

			if lic.ExpiresAt.Valid && lic.ExpiresAt.Time.UTC().Before(now) {
				h.logger.Info("Found expired license, updating status",
					zap.String("license_id", lic.ID.String()),
					zap.String("license_key", lic.LicenseKey),
					zap.Time("expires_at", lic.ExpiresAt.Time),
				)

				errUpdate := h.repo.UpdateStatus(ctx, lic.ID, license.StatusExpired)
				if errUpdate != nil {

					h.logger.Error("Failed to update status for expired license",
						zap.String("license_id", lic.ID.String()),
						zap.Error(errUpdate),
					)

				} else {
					updatedCount++
				}
			}
		}

		if int64(len(licensesToExpire)) < int64(params.Limit) {
			break
		}

		params.Offset += params.Limit

		if params.Offset > int(total) && total > 0 {
			h.logger.Warn("Offset exceeded total count during expiration check, breaking loop", zap.Int("offset", params.Offset), zap.Int64("total", total))
			break
		}
	}

	h.logger.Info("License expiration check task finished", zap.Int("processed_licenses", processedCount), zap.Int("updated_to_expired", updatedCount))
	return nil
}

func ptr[T any](v T) *T {
	return &v
}
