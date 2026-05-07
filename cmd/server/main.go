package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gustavodetoni/pullsing/internal/application"
	"github.com/gustavodetoni/pullsing/internal/infrastructure/config"
	grpcinfra "github.com/gustavodetoni/pullsing/internal/infrastructure/grpc"
	postgresinfra "github.com/gustavodetoni/pullsing/internal/infrastructure/postgres"
	redisinfra "github.com/gustavodetoni/pullsing/internal/infrastructure/redis"
	grpcserver "github.com/gustavodetoni/pullsing/internal/interfaces/grpc"
	httpserver "github.com/gustavodetoni/pullsing/internal/interfaces/http"
)

func main() {
	cfg := config.Load()

	logger := log.New(cfg.LogOutput(), "", log.Ldate|log.Ltime|log.LUTC)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgresinfra.NewPool(ctx, cfg.PostgresURL)
	if err != nil {
		logger.Fatalf("postgres setup failed: %v", err)
	}
	defer pool.Close()

	if err := postgresinfra.RunMigrations(ctx, pool, migrationDir()); err != nil {
		logger.Fatalf("postgres migrations failed: %v", err)
	}

	redisClient := redisinfra.NewClient(cfg.RedisAddr)
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatalf("redis setup failed: %v", err)
	}

	store := postgresinfra.NewStore(pool)
	publisher := redisinfra.NewPublisher(redisClient, "")
	subscriber := redisinfra.NewSubscriber(redisClient, "")
	adminService := application.NewAdminService(store, publisher)
	sdkService := application.NewSDKService(store)
	hub := grpcinfra.NewHub(cfg.GRPCClientBuffer)
	relay := grpcinfra.NewRedisRelay(subscriber, hub, logger)

	httpSrv := httpserver.New(cfg, logger, adminService)
	grpcSrv, err := grpcserver.New(cfg, logger, sdkService, hub, relay)
	if err != nil {
		logger.Fatalf("grpc server setup failed: %v", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	go func() {
		errCh <- httpSrv.Run(runCtx)
	}()
	go func() {
		errCh <- grpcSrv.Run(runCtx)
	}()

	var runErr error
	for i := 0; i < 2; i++ {
		err := <-errCh
		if err == nil || err == http.ErrServerClosed || runErr != nil {
			continue
		}

		runErr = err
		cancel()
	}

	if runErr != nil {
		logger.Fatalf("server stopped with error: %v", runErr)
	}
}

func migrationDir() string {
	if dir := os.Getenv("PULLSING_MIGRATIONS_DIR"); dir != "" {
		return dir
	}

	candidates := make([]string, 0, 3)
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "migrations"))
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "migrations"))
	}
	candidates = append(candidates, "migrations")

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	return candidates[0]
}
