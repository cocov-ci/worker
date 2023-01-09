package commands

import (
	"github.com/cocov-ci/worker/api"
	"github.com/cocov-ci/worker/docker"
	"github.com/cocov-ci/worker/redis"
	"github.com/cocov-ci/worker/runner"
	"github.com/cocov-ci/worker/storage"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

func Run(ctx *cli.Context) error {
	redisURL := ctx.String("redis-url")
	dockerSocket := ctx.String("docker-socket")
	maxJobs := ctx.Int("max-parallel-jobs")
	apiURL := ctx.String("api-url")
	serviceToken := ctx.String("service-token")
	storageMode := ctx.String("gs-storage-mode")
	isDevelopment := os.Getenv("COCOV_WORKER_DEV") == "true"

	var logger *zap.Logger
	var err error
	if isDevelopment {
		config := zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.Development = true
		logger, err = config.Build()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		return err
	}

	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	redisClient, err := redis.New(redisURL)
	if err != nil {
		logger.Error("Failed initializing Redis client", zap.Error(err))
		return err
	}
	go redisClient.Start()

	store, err := storage.Initialize(storageMode, ctx)
	if err != nil {
		logger.Error("Failed initializing storage", zap.Error(err))
		return err
	}

	dockerClient, err := docker.New(dockerSocket)
	if err != nil {
		logger.Error("Failed initializing Docker client", zap.Error(err))
		return err
	}

	logger.Info("Preparing system images...")
	if err = dockerClient.PullImage("alpine"); err != nil {
		logger.Error("Failed downloading image", zap.Error(err))
		return err
	}

	apiClient, err := api.New(apiURL, serviceToken)
	if err != nil {
		logger.Error("Failed initializing API client", zap.Error(err))
		return err
	}

	jobRunner := runner.New(maxJobs, apiClient, dockerClient, redisClient, store)
	jobRunner.Run()

	return nil
}
