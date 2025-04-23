package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/makkenzo/license-service-api/internal/service"
	"go.uber.org/zap"
)

const (
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "
	userContextKey      = "userClaims"
)

func AuthMiddleware(authService *service.AuthService, logger *zap.Logger) gin.HandlerFunc {
	log := logger.Named("AuthMiddleware")
	return func(c *gin.Context) {
		authHeader := c.GetHeader(authorizationHeader)
		if authHeader == "" {
			log.Debug("Authorization header is missing")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}

		if !strings.HasPrefix(authHeader, bearerPrefix) {
			log.Debug("Authorization header format is invalid", zap.String("header", authHeader))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, bearerPrefix)
		if tokenString == "" {
			log.Debug("Token is missing after Bearer prefix")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token missing"})
			return
		}

		claims, err := authService.ValidateToken(c.Request.Context(), tokenString)
		if err != nil {
			log.Warn("Token validation failed", zap.Error(err))

			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		log.Debug("Token validated, setting user claims in context", zap.String("user_id", claims.UserID.String()), zap.String("role", claims.Role))
		c.Set(userContextKey, claims)

		c.Next()
	}
}

func GetUserClaims(c *gin.Context) *service.Claims {
	claims, exists := c.Get(userContextKey)
	if !exists {
		return nil
	}
	userClaims, ok := claims.(*service.Claims)
	if !ok {
		return nil
	}
	return userClaims
}
