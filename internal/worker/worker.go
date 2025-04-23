package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/makkenzo/license-service-api/internal/config"
	"github.com/makkenzo/license-service-api/internal/domain/license"
	"github.com/makkenzo/license-service-api/internal/tasks"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func RunWorkers(ctx context.Context, cfg *config.Config, repo license.Repository, logger *zap.Logger) error {
	redisConnOpts := asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}
	logServer := logger.Named("AsynqServer")
	logScheduler := logger.Named("AsynqScheduler")

	srv := asynq.NewServer(
		redisConnOpts,
		asynq.Config{
			Concurrency: 10,
			Queues:      map[string]int{"critical": 6, "default": 3, "low": 1},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				logServer.Error("Asynq task processing failed",
					zap.String("task_id", task.Type()),
					zap.ByteString("payload", task.Payload()),
					zap.Error(err),
				)
			}),
			Logger: NewAsynqLoggerAdapter(logServer),

			ShutdownTimeout: 30 * time.Second,
		},
	)
	mux := asynq.NewServeMux()
	expireHandler := tasks.NewLicenseExpireHandler(repo, logger)
	mux.HandleFunc(tasks.TypeLicenseExpire, expireHandler.ProcessTask)

	scheduler := asynq.NewScheduler(
		redisConnOpts,
		&asynq.SchedulerOpts{
			Logger: NewAsynqLoggerAdapter(logScheduler),
		},
	)

	licenseExpireTask, err := tasks.NewLicenseExpireTask()
	if err != nil {
		return fmt.Errorf("scheduler task creation error: %w", err)
	}
	entryID, err := scheduler.Register("@every 1h", licenseExpireTask)
	if err != nil {
		return fmt.Errorf("scheduler registration error: %w", err)
	}
	logger.Info("Registered periodic license expiration check", zap.String("entry_id", entryID), zap.String("schedule", "@every 1h"))

	g, workerCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		logServer.Info("Starting Asynq Server...")

		if err := srv.Run(mux); err != nil {
			logServer.Error("Asynq Server run failed", zap.Error(err))
			return fmt.Errorf("asynq server run error: %w", err)
		}
		logServer.Info("Asynq Server Run gracefully finished.")
		return nil
	})

	g.Go(func() error {
		logScheduler.Info("Starting Asynq Scheduler...")

		if err := scheduler.Run(); err != nil {
			logScheduler.Error("Asynq Scheduler run failed", zap.Error(err))
			return fmt.Errorf("asynq scheduler run error: %w", err)
		}
		logScheduler.Info("Asynq Scheduler Run gracefully finished.")
		return nil
	})

	go func() {
		<-workerCtx.Done()
		logScheduler.Info("Shutdown signal received by worker, initiating Asynq shutdown...")

		scheduler.Shutdown()
		logScheduler.Info("Asynq Scheduler shutdown initiated.")

		srv.Shutdown()
		logServer.Info("Asynq Server shutdown initiated.")
	}()

	logger.Info("Asynq workers running...")

	runErr := g.Wait()
	logger.Info("Asynq workers stopped.")
	return runErr
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
