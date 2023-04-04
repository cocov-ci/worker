package commands

import (
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"os/signal"

	"github.com/cocov-ci/worker/api"
	"github.com/cocov-ci/worker/docker"
	"github.com/cocov-ci/worker/redis"
	"github.com/cocov-ci/worker/runner"
	"github.com/cocov-ci/worker/storage"
)

func Run(ctx *cli.Context) error {
	redisURL := ctx.String("redis-url")
	dockerSocket := ctx.String("docker-socket")
	maxJobs := ctx.Int("max-parallel-jobs")
	apiURL := ctx.String("api-url")
	serviceToken := ctx.String("service-token")
	storageMode := ctx.String("gs-storage-mode")
	cacheServerURL := ctx.String("cache-server-url")
	dockerTLSCAPath := ctx.String("docker-tls-ca-path")
	dockerTLSCertPath := ctx.String("docker-tls-cert-path")
	dockerTLSKeyPath := ctx.String("docker-tls-key-path")
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

	defer func() { _ = logger.Sync() }()
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

	dockerClient, err := docker.New(docker.ClientOpts{
		Socket:         dockerSocket,
		CacheServerURL: cacheServerURL,
		TLSCAPath:      dockerTLSCAPath,
		TLSKeyPath:     dockerTLSKeyPath,
		TLSCertPath:    dockerTLSCertPath,
	})
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

	signalChan := make(chan os.Signal, 10)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		stopping := false
		for {
			<-signalChan
			if !stopping {
				stopping = true
				logger.Info("Received interrupt signal. Will exit once jobs in progress are finished")
				jobRunner.Shutdown()
			} else {
				logger.Info("If you insist... Received second interrupt signal. Forcefully exiting.")
				logger.Warn("Forcefully exiting worker. Jobs in execution may be left dangling on API, and temporary files won't be removed.")
				os.Exit(0)
			}
		}
	}()

	jobRunner.Run()

	logger.Info("See you later!")

	return nil
}
