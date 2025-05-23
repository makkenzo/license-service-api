package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/makkenzo/license-service-api/internal/handler/dto"
	"github.com/makkenzo/license-service-api/internal/ierr"
	"github.com/makkenzo/license-service-api/internal/service"
	"go.uber.org/zap"
)

type LicenseHandler struct {
	service *service.LicenseService
	logger  *zap.Logger
}

func NewLicenseHandler(service *service.LicenseService, logger *zap.Logger) *LicenseHandler {
	return &LicenseHandler{
		service: service,
		logger:  logger.Named("LicenseHandler"),
	}
}

func (h *LicenseHandler) Create(c *gin.Context) {
	h.logger.Debug("Received request to create license")
	var req dto.CreateLicenseRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Failed to bind or validate request body", zap.Error(err))

		_ = c.Error(err)
		return
	}

	createdLicense, err := h.service.CreateLicense(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Service failed to create license", zap.Error(err))

		_ = c.Error(err)
		return
	}

	h.logger.Info("License created successfully via handler", zap.String("id", createdLicense.ID.String()))
	responseDTO := dto.NewLicenseResponse(createdLicense)
	c.JSON(http.StatusCreated, responseDTO)
}

func (h *LicenseHandler) List(c *gin.Context) {
	h.logger.Debug("Received request to list licenses")
	var req dto.ListLicensesRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		h.logger.Warn("Failed to bind or validate query parameters", zap.Error(err))
		_ = c.Error(err)
		return
	}

	licenses, totalCount, err := h.service.ListLicenses(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Service failed to list licenses", zap.Error(err))
		_ = c.Error(err)
		return
	}

	licenseResponses := make([]*dto.LicenseResponse, len(licenses))
	for i, lic := range licenses {
		licenseResponses[i] = dto.NewLicenseResponse(lic)
	}

	paginatedResponse := dto.PaginatedLicenseResponse{
		Licenses:   licenseResponses,
		TotalCount: totalCount,
		Limit:      req.Limit,
		Offset:     req.Offset,
	}

	c.JSON(http.StatusOK, paginatedResponse)
}

func (h *LicenseHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	h.logger.Debug("Received request to get license by ID", zap.String("id_param", idStr))

	id, err := uuid.Parse(idStr)
	if err != nil {
		h.logger.Warn("Invalid UUID format received", zap.String("id_param", idStr), zap.Error(err))
		_ = c.Error(err)
		return
	}

	lic, err := h.service.GetLicenseByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ierr.ErrNotFound) {
			h.logger.Info("License not found by handler", zap.String("id", idStr))
			_ = c.Error(err)
			return
		}

		h.logger.Error("Service failed to get license by ID", zap.String("id", idStr), zap.Error(err))
		_ = c.Error(err)
		return
	}

	h.logger.Info("License retrieved successfully via handler", zap.String("id", idStr))
	responseDTO := dto.NewLicenseResponse(lic)
	c.JSON(http.StatusOK, responseDTO)
}

func (h *LicenseHandler) UpdateStatus(c *gin.Context) {
	idStr := c.Param("id")
	h.logger.Debug("Received request to update license status", zap.String("id_param", idStr))

	id, err := uuid.Parse(idStr)
	if err != nil {
		h.logger.Warn("Invalid UUID format for status update", zap.String("id_param", idStr), zap.Error(err))
		_ = c.Error(err)
		return
	}

	var req dto.UpdateLicenseStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Failed to bind or validate status update request body", zap.String("id", idStr), zap.Error(err))
		_ = c.Error(err)
		return
	}

	err = h.service.UpdateLicenseStatus(c.Request.Context(), id, *req.Status)
	if err != nil {

		if errors.Is(err, ierr.ErrNotFound) {
			h.logger.Info("License not found for status update", zap.String("id", idStr))
			_ = c.Error(err)
			return
		}
		if errors.Is(err, ierr.ErrUpdateFailed) {
			h.logger.Error("Repository failed to update license status", zap.String("id", idStr), zap.Error(err))
			_ = c.Error(err)
			return
		}

		h.logger.Error("Service failed to update license status", zap.String("id", idStr), zap.Error(err))
		_ = c.Error(err)
		return
	}

	h.logger.Info("License status updated successfully via handler", zap.String("id", idStr), zap.String("new_status", string(*req.Status)))

	c.JSON(http.StatusOK, gin.H{"message": "License status updated successfully"})

}

func (h *LicenseHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	h.logger.Debug("Received request to update license", zap.String("id_param", idStr))

	id, err := uuid.Parse(idStr)
	if err != nil {
		h.logger.Warn("Invalid UUID format for update", zap.String("id_param", idStr), zap.Error(err))
		_ = c.Error(err)
		return
	}

	var req dto.UpdateLicenseRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Failed to bind or validate update request body", zap.String("id", idStr), zap.Error(err))
		_ = c.Error(err)
		return
	}

	updatedLicense, err := h.service.UpdateLicense(c.Request.Context(), id, &req)
	if err != nil {

		if errors.Is(err, ierr.ErrNotFound) {
			h.logger.Info("License not found for update by handler", zap.String("id", idStr))
			_ = c.Error(err)
			return
		}

		if errors.Is(err, ierr.ErrUpdateFailed) {
			h.logger.Error("Repository failed to update license", zap.String("id", idStr), zap.Error(err))
			_ = c.Error(err)
			return
		}

		h.logger.Error("Service failed to update license", zap.String("id", idStr), zap.Error(err))
		_ = c.Error(err)
		return
	}

	h.logger.Info("License updated successfully via handler", zap.String("id", idStr))
	responseDTO := dto.NewLicenseResponse(updatedLicense)
	c.JSON(http.StatusOK, responseDTO)
}

func (h *LicenseHandler) Validate(c *gin.Context) {
	h.logger.Debug("Received request to validate license")
	var req dto.ValidateLicenseRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Failed to bind or validate validation request body", zap.Error(err))
		_ = c.Error(err)
		return
	}

	validationResult, err := h.service.ValidateLicense(c.Request.Context(), &req)
	if err != nil {

		h.logger.Error("Service failed during license validation", zap.String("license_key", req.LicenseKey), zap.Error(err))
		_ = c.Error(err)
		return
	}

	resp := dto.ValidateLicenseResponse{
		IsValid:     validationResult.IsValid,
		Reason:      validationResult.Reason,
		AllowedData: validationResult.ResponseData,
	}

	if validationResult.License != nil {
		resp.Status = &validationResult.License.Status
		if validationResult.License.ExpiresAt.Valid {
			resp.ExpiresAt = &validationResult.License.ExpiresAt.Time
		}
	}

	h.logger.Info("License validation processed",
		zap.String("license_key", req.LicenseKey),
		zap.Bool("is_valid", resp.IsValid),
		zap.String("reason", resp.Reason),
	)
	c.JSON(http.StatusOK, resp)
}
