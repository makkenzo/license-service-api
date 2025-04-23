package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/makkenzo/license-service-api/internal/handler/dto"
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

		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	createdLicense, err := h.service.CreateLicense(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Service failed to create license", zap.Error(err))

		if errors.Is(err, pgx.ErrNoRows) {

			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal error after creating license"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create license"})
		return
	}

	h.logger.Info("License created successfully via handler", zap.String("id", createdLicense.ID.String()))

	responseDTO := dto.NewLicenseResponse(createdLicense)
	c.JSON(http.StatusCreated, responseDTO)
}
