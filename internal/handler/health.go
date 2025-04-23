package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type HealthHandler struct {
	db     *pgxpool.Pool
	redis  *redis.Client
	logger *zap.Logger
}

func NewHealthHandler(db *pgxpool.Pool, redis *redis.Client, logger *zap.Logger) *HealthHandler {
	return &HealthHandler{
		db:     db,
		redis:  redis,
		logger: logger,
	}
}

func (h *HealthHandler) Check(c *gin.Context) {
	dbStatus := "ok"
	if err := h.db.Ping(c.Request.Context()); err != nil {
		dbStatus = "error"
		h.logger.Error("Health check: PostgreSQL ping failed", zap.Error(err))
	}

	redisStatus := "ok"
	if _, err := h.redis.Ping(c.Request.Context()).Result(); err != nil {
		redisStatus = "error"
		h.logger.Error("Health check: Redis ping failed", zap.Error(err))
	}

	if dbStatus == "error" || redisStatus == "error" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"dependencies": gin.H{
				"database": dbStatus,
				"redis":    redisStatus,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"dependencies": gin.H{
			"database": dbStatus,
			"redis":    redisStatus,
		},
	})
}
