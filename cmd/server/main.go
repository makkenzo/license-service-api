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
	"github.com/makkenzo/license-service-api/internal/handler/middleware"
	"github.com/makkenzo/license-service-api/internal/ierr"
	"github.com/makkenzo/license-service-api/internal/service"
	"github.com/makkenzo/license-service-api/internal/storage/memstorage"
	"github.com/makkenzo/license-service-api/internal/storage/postgres"
	apikeyRepoImpl "github.com/makkenzo/license-service-api/internal/storage/postgres"
	"github.com/makkenzo/license-service-api/internal/storage/redis"
	"github.com/makkenzo/license-service-api/internal/worker"
	"github.com/makkenzo/license-service-api/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
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

	licenseRepo := postgres.NewLicenseRepository(dbPool, appLogger)
	userRepoMock := memstorage.NewUserRepositoryMock()
	apiKeyRepo := apikeyRepoImpl.NewAPIKeyRepository(dbPool, appLogger)

	licenseService := service.NewLicenseService(licenseRepo, appLogger)
	authService := service.NewAuthService(userRepoMock, &cfg.JWT, appLogger)

	authMiddleware := middleware.AuthMiddleware(authService, appLogger)
	apiKeyAuthMiddleware := middleware.APIKeyAuthMiddleware(apiKeyRepo, appLogger)
	errorMiddleware := middleware.ErrorHandlerMiddleware(appLogger)

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
		logMsg := "Panic recovered"
		if err, ok := recovered.(string); ok {
			logMsg = fmt.Sprintf("%s: %s", logMsg, err)
		} else if err, ok := recovered.(error); ok {
			logMsg = fmt.Sprintf("%s: %v", logMsg, err)
		}
		appLogger.Error(logMsg, zap.Stack("stack"))

		_ = c.Error(ierr.ErrInternalServer)
		c.Abort()
	}))

	router.Use(errorMiddleware)

	healthHandler := handler.NewHealthHandler(dbPool, redisClient, appLogger)
	licenseHandler := handler.NewLicenseHandler(licenseService, appLogger)
	authHandler := handler.NewAuthHandler(authService, appLogger)
	dashboardHandler := handler.NewDashboardHandler(licenseService, appLogger)

	router.GET("/healthz", healthHandler.Check)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	authRoutes := router.Group("/api/v1/auth")
	{
		authRoutes.POST("/login", authHandler.Login)
	}

	apiV1 := router.Group("/api/v1")
	{
		licenseRoutes := apiV1.Group("/licenses")
		{
			licenseRoutes.POST("/validate", apiKeyAuthMiddleware, licenseHandler.Validate)

			licenseRoutes.Use(authMiddleware)

			licenseRoutes.POST("", licenseHandler.Create)
			licenseRoutes.GET("", licenseHandler.List)
			licenseRoutes.GET("/:id", licenseHandler.GetByID)
			licenseRoutes.PATCH("/:id", licenseHandler.Update)
			licenseRoutes.PATCH("/:id/status", licenseHandler.UpdateStatus)
		}
		dashboardRoutes := apiV1.Group("/dashboard")
		dashboardRoutes.Use(authMiddleware)
		{
			dashboardRoutes.GET("/summary", dashboardHandler.GetSummary)
		}
	}

	eg, appCtx := errgroup.WithContext(context.Background())

	httpServer := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	eg.Go(func() error {
		sugarLogger.Infof("HTTP server listening on port %s", cfg.Server.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			sugarLogger.Errorf("Failed to start HTTP server: %v", err)
			return fmt.Errorf("http server error: %w", err)
		}
		sugarLogger.Info("HTTP server stopped.")
		return nil
	})

	workerErrChan, workerShutdown := worker.RunWorkers(cfg, licenseRepo, appLogger)
	eg.Go(func() error {
		select {
		case err := <-workerErrChan:
			sugarLogger.Error("Asynq worker failed", zap.Error(err))
			return fmt.Errorf("asynq worker error: %w", err)
		case <-appCtx.Done():
			sugarLogger.Info("Asynq worker context cancelled")
			return nil
		}
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		sugarLogger.Infof("Received signal %s, shutting down gracefully...", sig)
	case <-appCtx.Done():
		sugarLogger.Warn("Context done, shutting down due to error...")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.Server.ShutdownPeriod)
	defer cancelShutdown()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		sugarLogger.Errorf("HTTP server shutdown failed: %v", err)
	} else {
		sugarLogger.Info("HTTP server shutdown complete.")
	}

	workerShutdown(shutdownCtx)
	sugarLogger.Info("Asynq workers shutdown complete.")

	if err := eg.Wait(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		sugarLogger.Errorf("Application shutdown finished with error: %v", err)
		os.Exit(1)
	}

	sugarLogger.Info("Application shutdown complete.")
}
