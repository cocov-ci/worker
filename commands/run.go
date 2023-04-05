package commands

import (
	"context"
	"fmt"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/cocov-ci/worker/api"
	"github.com/cocov-ci/worker/docker"
	"github.com/cocov-ci/worker/redis"
	"github.com/cocov-ci/worker/runner"
	"github.com/cocov-ci/worker/storage"
)

const attempts = 10

func Backoff(log *zap.Logger, operation string, fn func() error) (err error) {
	for i := 0; i < attempts; i++ {
		time.Sleep(time.Duration(i*4) * time.Second)
		if err = fn(); err != nil {
			log.Error("Backoff "+operation,
				zap.Int("delay_secs", (i+1)*4),
				zap.String("attempt", fmt.Sprintf("%d/%d", i+1, attempts)),
				zap.Error(err))
			continue
		}
		return nil
	}
	return err
}

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
	debugPlugins := ctx.Bool("debug-plugins")

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

	if cacheServerURL != "" {
		var parsedUrl *url.URL
		parsedUrl, err = url.Parse(cacheServerURL)
		if err != nil {
			logger.Error("Failed trying to parse cache server URL", zap.Error(err))
			return err
		}

		host := parsedUrl.Hostname()
		err = Backoff(logger, "resolving Cache server host", func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil
			}
			logger.Info("Resolved cache server address",
				zap.String("cache_server_url", cacheServerURL),
				zap.Int("ips_length", len(ips)))
			return nil
		})
		if err != nil {
			logger.Warn("Unable to resolve cache server address. Disabling cache.",
				zap.String("url", cacheServerURL),
				zap.Error(err))
			cacheServerURL = ""
		}
	}

	var redisClient redis.Client
	err = Backoff(logger, "initializing Redis client", func() error {
		redisClient, err = redis.New(redisURL)
		return err
	})
	if err != nil {
		logger.Error("Gave up trying to initialize Redis client.")
		return err
	}

	go redisClient.Start()
	logger.Info("Redis client initialized")

	store, err := storage.Initialize(storageMode, ctx)
	if err != nil {
		logger.Error("Failed initializing storage", zap.Error(err))
		return err
	}
	logger.Info("Storage abstraction initialized")

	var dockerClient docker.Client
	err = Backoff(logger, "initializing Docker client", func() error {
		dockerClient, err = docker.New(docker.ClientOpts{
			Socket:         dockerSocket,
			CacheServerURL: cacheServerURL,
			TLSCAPath:      dockerTLSCAPath,
			TLSKeyPath:     dockerTLSKeyPath,
			TLSCertPath:    dockerTLSCertPath,
		})
		return err
	})
	if err != nil {
		logger.Error("Gave up trying to initialize Redis client.")
		return err
	}
	logger.Info("Docker client initialized")

	logger.Info("Preparing system images...")
	thence := time.Now()
	err = Backoff(logger, "preparing system images", func() error {
		return dockerClient.PullImage("alpine")
	})
	if err != nil {
		logger.Error("Gave up trying to prepare system images")
		return err
	}
	logger.Info("Prepared system images", zap.String("elapsed", time.Since(thence).String()))

	var apiClient api.Client
	err = Backoff(logger, "initializing API client", func() error {
		apiClient, err = api.New(apiURL, serviceToken)
		return err
	})
	if err != nil {
		logger.Error("Gave up trying to initialize API client")
		return err
	}
	logger.Info("Initialized API client")

	jobRunner := runner.New(runner.SchedulerOpts{
		MaxJobs:      maxJobs,
		API:          apiClient,
		Docker:       dockerClient,
		RedisClient:  redisClient,
		Storage:      store,
		DebugPlugins: debugPlugins,
	})

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
