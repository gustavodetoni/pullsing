package http

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gustavodetoni/pullsing/internal/infrastructure/config"
)

type Server struct {
	httpServer *http.Server
	config     config.Config
	logger     *log.Logger
}

type healthResponse struct {
	Status      string `json:"status"`
	Service     string `json:"service"`
	Environment string `json:"environment"`
	Timestamp   string `json:"timestamp"`
}

func New(cfg config.Config, logger *log.Logger) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler(cfg))
	mux.HandleFunc("/readyz", healthHandler(cfg))

	return &Server{
		httpServer: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           mux,
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
		config: cfg,
		logger: logger,
	}
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		s.logger.Printf("http server listening on %s", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
		defer cancel()

		s.logger.Printf("shutting down http server")
		return s.httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func healthHandler(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_ = json.NewEncoder(w).Encode(healthResponse{
			Status:      "ok",
			Service:     cfg.AppName,
			Environment: cfg.Environment,
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
		})
	}
}
