package worker

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/makkenzo/license-service-api/internal/config"
	"github.com/makkenzo/license-service-api/internal/domain/license"
	"github.com/makkenzo/license-service-api/internal/tasks"
	"go.uber.org/zap"
)

func RunWorkers(cfg *config.Config, repo license.Repository, logger *zap.Logger) (<-chan error, func(context.Context)) {
	errChan := make(chan error, 2)

	redisConnOpts := asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	srv := asynq.NewServer(
		redisConnOpts,
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log := logger.Named("AsynqServerErrorHandler")
				log.Error("Asynq task processing failed",
					zap.String("task_id", task.Type()),
					zap.ByteString("payload", task.Payload()),
					zap.Error(err),
				)

			}),
			Logger: NewAsynqLoggerAdapter(logger.Named("AsynqServer")),
		},
	)

	mux := asynq.NewServeMux()

	expireHandler := tasks.NewLicenseExpireHandler(repo, logger)
	mux.HandleFunc(tasks.TypeLicenseExpire, expireHandler.ProcessTask)

	go func() {
		logger.Info("Starting Asynq Server...")
		if err := srv.Run(mux); err != nil {
			logger.Error("Asynq Server run failed", zap.Error(err))
			errChan <- fmt.Errorf("asynq server error: %w", err)
		}
		logger.Info("Asynq Server stopped.")
	}()

	scheduler := asynq.NewScheduler(
		redisConnOpts,
		&asynq.SchedulerOpts{
			Logger: NewAsynqLoggerAdapter(logger.Named("AsynqScheduler")),
		},
	)

	licenseExpireTask, err := tasks.NewLicenseExpireTask()
	if err != nil {
		logger.Error("Failed to create license expire task for scheduler", zap.Error(err))
		errChan <- fmt.Errorf("scheduler task creation error: %w", err)

	} else {

		entryID, err := scheduler.Register("@every 1h", licenseExpireTask)

		if err != nil {
			logger.Error("Could not register periodic task for license expiration", zap.Error(err))
			errChan <- fmt.Errorf("scheduler registration error: %w", err)
		} else {
			logger.Info("Registered periodic license expiration check", zap.String("entry_id", entryID), zap.String("schedule", "@every 1h"))
		}
	}

	go func() {
		logger.Info("Starting Asynq Scheduler...")
		if err := scheduler.Run(); err != nil {
			logger.Error("Asynq Scheduler run failed", zap.Error(err))
			errChan <- fmt.Errorf("asynq scheduler error: %w", err)
		}
		logger.Info("Asynq Scheduler stopped.")
	}()

	shutdownFunc := func(ctx context.Context) {
		logger.Info("Shutting down Asynq Scheduler...")
		scheduler.Shutdown()
		logger.Info("Asynq Scheduler stopped.")

		logger.Info("Shutting down Asynq Server...")
		srv.Shutdown()
		logger.Info("Asynq Server stopped.")
	}

	return errChan, shutdownFunc
}

type asynqLoggerAdapter struct {
	logger *zap.Logger
}

func NewAsynqLoggerAdapter(logger *zap.Logger) *asynqLoggerAdapter {
	return &asynqLoggerAdapter{logger: logger.WithOptions(zap.AddCallerSkip(1))}
}

func (l *asynqLoggerAdapter) Debug(args ...interface{}) {
	l.logger.Debug(fmt.Sprint(args...))
}
func (l *asynqLoggerAdapter) Info(args ...interface{}) {
	l.logger.Info(fmt.Sprint(args...))
}
func (l *asynqLoggerAdapter) Warn(args ...interface{}) {
	l.logger.Warn(fmt.Sprint(args...))
}
func (l *asynqLoggerAdapter) Error(args ...interface{}) {
	l.logger.Error(fmt.Sprint(args...))
}
func (l *asynqLoggerAdapter) Fatal(args ...interface{}) {
	l.logger.Fatal(fmt.Sprint(args...))
}
