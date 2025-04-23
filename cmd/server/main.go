package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/makkenzo/license-service-api/internal/config"
	"github.com/makkenzo/license-service-api/internal/handler"
	"github.com/makkenzo/license-service-api/internal/storage/postgres"
	"github.com/makkenzo/license-service-api/internal/storage/redis"
	"github.com/makkenzo/license-service-api/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	configPath := flag.String("config", "./configs/config.dev.yaml", "Path to configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	appLogger, err := logger.NewZapLogger(cfg.Log.Level)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer appLogger.Sync()

	sugarLogger := appLogger.Sugar()

	sugarLogger.Info("Starting application...")
	sugarLogger.Infof("Log level set to: %s", cfg.Log.Level)

	ctx := context.Background()

	dbPool, err := postgres.NewPgxPool(ctx, &cfg.Database, appLogger)
	if err != nil {
		sugarLogger.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer dbPool.Close()

	redisClient, err := redis.NewRedisClient(ctx, &cfg.Redis, appLogger)
	if err != nil {
		sugarLogger.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	router := gin.New()

	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	}))

	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			sugarLogger.Errorf("Panic recovered: %s", err)
		}
		c.AbortWithStatus(http.StatusInternalServerError)
	}))

	healthHandler := handler.NewHealthHandler(dbPool, redisClient, appLogger)
	router.GET("/healthz", healthHandler.Check)

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	httpServer := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		sugarLogger.Infof("HTTP server listening on port %s", cfg.Server.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			sugarLogger.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	sugarLogger.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownPeriod)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		sugarLogger.Fatalf("Server forced to shutdown: %v", err)
	}

	sugarLogger.Info("Server exiting")
}
