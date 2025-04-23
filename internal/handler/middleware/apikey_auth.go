package middleware

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	apikeyRepo "github.com/makkenzo/license-service-api/internal/domain/apikey"
	"github.com/makkenzo/license-service-api/internal/ierr"
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
			_ = c.Error(fmt.Errorf("%w: API key required in %s header", ierr.ErrUnauthorized, apiKeyHeader))
			c.Abort()
			return
		}

		parts := strings.SplitN(apiKeyFromHeader, "_", 3)
		if len(parts) < 3 || parts[0] != "lm" {
			log.Warn("Invalid API key format received", zap.String("key_received", apiKeyFromHeader))
			_ = c.Error(fmt.Errorf("%w: invalid API key format", ierr.ErrUnauthorized))
			c.Abort()
			return
		}
		prefix := parts[1]

		keyRecord, err := apiKeyRepo.FindByPrefix(c.Request.Context(), prefix)
		if err != nil {
			if errors.Is(err, ierr.ErrAPIKeyNotFound) {
				log.Warn("API key not found or disabled", zap.String("prefix", prefix))
				_ = c.Error(fmt.Errorf("%w: invalid or disabled api key", ierr.ErrForbidden))
				c.Abort()
				return
			}

			log.Error("Failed to query API key repository", zap.String("prefix", prefix), zap.Error(err))
			_ = c.Error(fmt.Errorf("%w: checking api key: %v", ierr.ErrInternalServer, err))
			c.Abort()
			return
		}

		receivedKeyHash := util.HashAPIKey(apiKeyFromHeader)

		if subtle.ConstantTimeCompare([]byte(receivedKeyHash), []byte(keyRecord.KeyHash)) != 1 {
			log.Warn("API key hash mismatch", zap.String("prefix", prefix), zap.String("key_id", keyRecord.ID.String()))
			_ = c.Error(fmt.Errorf("%w: invalid or disabled api key", ierr.ErrForbidden))
			c.Abort()
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
