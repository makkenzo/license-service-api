package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/makkenzo/license-service-api/internal/handler/dto"
	"github.com/makkenzo/license-service-api/internal/ierr"
	"github.com/makkenzo/license-service-api/internal/service"
	"go.uber.org/zap"
)

type APIKeyHandler struct {
	service *service.APIKeyService
	logger  *zap.Logger
}

func NewAPIKeyHandler(service *service.APIKeyService, logger *zap.Logger) *APIKeyHandler {
	return &APIKeyHandler{
		service: service,
		logger:  logger.Named("APIKeyHandler"),
	}
}

func (h *APIKeyHandler) Create(c *gin.Context) {
	var req dto.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Failed to bind create api key request", zap.Error(err))
		_ = c.Error(fmt.Errorf("%w: %v", ierr.ErrValidation, err))
		return
	}

	var productIDPtr *uuid.UUID
	if req.ProductID != uuid.Nil {
		productIDPtr = &req.ProductID
	}

	respDTO, _, err := h.service.CreateAPIKey(c.Request.Context(), req.Description, productIDPtr)
	if err != nil {
		h.logger.Error("Service failed to create api key", zap.Error(err))
		_ = c.Error(err)
		return
	}

	h.logger.Info("API Key created via handler", zap.String("id", respDTO.ID.String()))
	c.JSON(http.StatusCreated, respDTO)
}

func (h *APIKeyHandler) List(c *gin.Context) {
	keys, err := h.service.ListAPIKeys(c.Request.Context())
	if err != nil {
		h.logger.Error("Service failed to list api keys", zap.Error(err))
		_ = c.Error(err)
		return
	}

	h.logger.Debug("API Keys listed successfully via handler", zap.Int("count", len(keys)))
	c.JSON(http.StatusOK, keys)
}

func (h *APIKeyHandler) Revoke(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.logger.Warn("Invalid UUID format for revoke api key", zap.String("id_param", idStr), zap.Error(err))
		_ = c.Error(fmt.Errorf("%w: invalid api key id format", ierr.ErrValidation))
		return
	}

	err = h.service.RevokeAPIKey(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Service failed to revoke api key", zap.String("id", id.String()), zap.Error(err))

		_ = c.Error(err)
		return
	}

	h.logger.Info("API Key revoked successfully via handler", zap.String("id", id.String()))
	c.Status(http.StatusNoContent)
}
