package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/makkenzo/license-service-api/internal/ierr"
	"github.com/makkenzo/license-service-api/internal/service"
	"go.uber.org/zap"
)

const (
	authorizationHeader     = "Authorization"
	bearerPrefix            = "Bearer "
	zitadelClaimsContextKey = "zitadelClaims"
)

func AuthMiddleware(authService *service.AuthService, logger *zap.Logger) gin.HandlerFunc {
	log := logger.Named("AuthMiddleware")
	return func(c *gin.Context) {
		authHeader := c.GetHeader(authorizationHeader)
		if authHeader == "" {
			log.Debug("Authorization header is missing")
			_ = c.Error(fmt.Errorf("%w: authorization header required", ierr.ErrUnauthorized))
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, bearerPrefix) {
			log.Debug("Authorization header format is invalid", zap.String("header", authHeader))
			_ = c.Error(fmt.Errorf("%w: invalid authorization header format", ierr.ErrUnauthorized))
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, bearerPrefix)
		if tokenString == "" {
			log.Debug("Token is missing after Bearer prefix")
			_ = c.Error(fmt.Errorf("%w: token missing", ierr.ErrUnauthorized))
			c.Abort()
			return
		}

		claims, err := authService.ValidateToken(c.Request.Context(), tokenString)
		if err != nil {
			log.Warn("Token validation failed", zap.Error(err))
			_ = c.Error(err)
			c.Abort()
			return
		}

		log.Debug("Access Token validated, setting claims in context", zap.String("subject", claims.Subject))
		c.Set(zitadelClaimsContextKey, claims)

		c.Next()
	}
}

func GetUserClaims(c *gin.Context) *service.ZitadelClaims {
	value, exists := c.Get(zitadelClaimsContextKey)
	if !exists {
		return nil
	}
	claims, ok := value.(*service.ZitadelClaims)
	if !ok {

		return nil
	}
	return claims
}
