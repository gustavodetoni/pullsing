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
	postgresinfra "github.com/gustavodetoni/pullsing/internal/infrastructure/postgres"
	redisinfra "github.com/gustavodetoni/pullsing/internal/infrastructure/redis"
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
	adminService := application.NewAdminService(store, publisher)
	server := httpserver.New(cfg, logger, adminService)

	if err := server.Run(ctx); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("server stopped with error: %v", err)
	}
}

func migrationDir() string {
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, "migrations")
	}

	return "migrations"
}
