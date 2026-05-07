package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/gustavodetoni/pullsing/internal/infrastructure/config"
	httpserver "github.com/gustavodetoni/pullsing/internal/interfaces/http"
)

func main() {
	cfg := config.Load()

	logger := log.New(cfg.LogOutput(), "", log.Ldate|log.Ltime|log.LUTC)

	server := httpserver.New(cfg, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("server stopped with error: %v", err)
	}
}
