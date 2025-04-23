package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/makkenzo/license-service-api/internal/service"
	"go.uber.org/zap"
)

type AuthHandler struct {
	service *service.AuthService
	logger  *zap.Logger
}

func NewAuthHandler(service *service.AuthService, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		service: service,
		logger:  logger.Named("AuthHandler"),
	}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Failed to bind login request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password are required"})
		return
	}

	token, err := h.service.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			h.logger.Info("Invalid login attempt", zap.String("username", req.Username))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		}

		h.logger.Error("Login service failed", zap.String("username", req.Username), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Login failed"})
		return
	}

	resp := LoginResponse{
		AccessToken: token,
	}
	h.logger.Info("User logged in successfully", zap.String("username", req.Username))
	c.JSON(http.StatusOK, resp)
}
