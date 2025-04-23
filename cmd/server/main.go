package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
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

	appCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dbPool, err := postgres.NewPgxPool(appCtx, &cfg.Database, appLogger)
	if err != nil {
		sugarLogger.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer dbPool.Close()

	redisClient, err := redis.NewRedisClient(appCtx, &cfg.Redis, appLogger)
	if err != nil {
		sugarLogger.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	licenseRepo := postgres.NewLicenseRepository(dbPool, appLogger)
	userRepoMock := memstorage.NewUserRepositoryMock()
	apiKeyRepo := apikeyRepoImpl.NewAPIKeyRepository(dbPool, appLogger)

	licenseService := service.NewLicenseService(licenseRepo, appLogger)
	authService := service.NewAuthService(userRepoMock, &cfg.JWT, appLogger)
	apiKeyService := service.NewAPIKeyService(apiKeyRepo, appLogger)

	healthHandler := handler.NewHealthHandler(dbPool, redisClient, appLogger)
	licenseHandler := handler.NewLicenseHandler(licenseService, appLogger)
	authHandler := handler.NewAuthHandler(authService, appLogger)
	dashboardHandler := handler.NewDashboardHandler(licenseService, appLogger)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyService, appLogger)

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

	corsConfig := cors.Config{
		AllowOrigins: []string{"http://localhost:3000", "http://marchenzo:3000"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-API-Key",
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	router.Use(cors.New(corsConfig))
	router.Use(errorMiddleware)

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
		apiKeyRoutes := apiV1.Group("/apikeys")
		apiKeyRoutes.Use(authMiddleware)
		{
			apiKeyRoutes.POST("", apiKeyHandler.Create)
			apiKeyRoutes.GET("", apiKeyHandler.List)
			apiKeyRoutes.DELETE("/:id", apiKeyHandler.Revoke)
		}
	}

	g, groupCtx := errgroup.WithContext(appCtx)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	g.Go(func() error {
		sugarLogger.Infof("HTTP server listening on port %s", cfg.Server.Port)

		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			sugarLogger.Errorf("HTTP server ListenAndServe error: %v", err)
			return fmt.Errorf("http server failed: %w", err)
		}
		sugarLogger.Info("HTTP server stopped listening.")
		return nil
	})

	g.Go(func() error {
		<-groupCtx.Done()
		sugarLogger.Info("Shutting down HTTP server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownPeriod)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			sugarLogger.Errorf("HTTP server graceful shutdown failed: %v", err)
			return fmt.Errorf("http server shutdown error: %w", err)
		}
		sugarLogger.Info("HTTP server shutdown complete.")
		return nil
	})

	g.Go(func() error {
		if err := worker.RunWorkers(groupCtx, cfg, licenseRepo, appLogger); err != nil {
			sugarLogger.Error("Asynq worker failed", zap.Error(err))
			return fmt.Errorf("asynq worker error: %w", err)
		}
		sugarLogger.Info("Asynq workers finished gracefully.")
		return nil
	})

	sugarLogger.Info("Application started. Waiting for interrupt signal (Ctrl+C) or component error...")

	waitErr := g.Wait()

	sugarLogger.Info("Shutdown sequence finished.")

	if waitErr != nil {

		if errors.Is(waitErr, context.Canceled) {
			sugarLogger.Info("Shutdown reason: Context canceled (likely due to OS signal).")
		} else if errors.Is(waitErr, http.ErrServerClosed) {
			sugarLogger.Info("Shutdown reason: HTTP server closed normally.")
		} else {
			sugarLogger.Errorf("Application shutdown finished with unexpected error: %v", waitErr)
		}
	} else {
		sugarLogger.Info("Application shutdown successfully (all components finished without errors).")
	}

	sugarLogger.Info("Application exiting now.")
}
