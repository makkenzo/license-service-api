package middleware

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/makkenzo/license-service-api/internal/domain/apikey"
	apikeyRepo "github.com/makkenzo/license-service-api/internal/domain/apikey"
	"github.com/makkenzo/license-service-api/internal/util"
)

const (
	apiKeyHeader = "X-API-Key"
)

func APIKeyAuthMiddleware(apiKeyRepo apikeyRepo.Repository, logger *zap.Logger) gin.HandlerFunc {
	log := logger.Named("APIKeyAuthMiddleware")
	return func(c *gin.Context) {
		apiKeyFromHeader := c.GetHeader(apiKeyHeader)
		if apiKeyFromHeader == "" {
			log.Debug("API Key header is missing", zap.String("header", apiKeyHeader))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
			return
		}

		parts := strings.SplitN(apiKeyFromHeader, "_", 3)
		if len(parts) < 3 || parts[0] != "lm" {
			log.Warn("Invalid API key format received", zap.String("key_received", apiKeyFromHeader))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key format"})
			return
		}
		prefix := parts[1]

		keyRecord, err := apiKeyRepo.FindByPrefix(c.Request.Context(), prefix)
		if err != nil {
			if errors.Is(err, apikey.ErrAPIKeyNotFound) {
				log.Warn("API key not found or disabled", zap.String("prefix", prefix))
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid or disabled API key"})
				return
			}

			log.Error("Failed to query API key repository", zap.String("prefix", prefix), zap.Error(err))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error during API key validation"})
			return
		}

		receivedKeyHash := util.HashAPIKey(apiKeyFromHeader)

		if subtle.ConstantTimeCompare([]byte(receivedKeyHash), []byte(keyRecord.KeyHash)) != 1 {
			log.Warn("API key hash mismatch", zap.String("prefix", prefix), zap.String("key_id", keyRecord.ID.String()))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid or disabled API key"})
			return
		}

		go func(id uuid.UUID, repo apikeyRepo.Repository, l *zap.Logger) {
			ctxAsync, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			errUpdate := repo.UpdateLastUsed(ctxAsync, id, time.Now().UTC())
			if errUpdate != nil {
				l.Error("Failed to update API key last used time asynchronously", zap.String("key_id", id.String()), zap.Error(errUpdate))
			}
		}(keyRecord.ID, apiKeyRepo, log)

		log.Info("API key validated successfully", zap.String("prefix", prefix), zap.String("key_id", keyRecord.ID.String()))
		c.Next()
	}
}
