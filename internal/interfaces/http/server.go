package http

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gustavodetoni/pullsing/internal/application"
	"github.com/gustavodetoni/pullsing/internal/domain"
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

type adminService interface {
	CreateProject(ctx context.Context, input application.CreateProjectInput) (domain.Project, error)
	CreateEnvironment(ctx context.Context, input application.CreateEnvironmentInput) (application.EnvironmentWithAPIKey, error)
	RotateAPIKey(ctx context.Context, environmentID int64) (string, error)
	CreateFlag(ctx context.Context, input application.CreateFlagInput) (domain.Flag, error)
}

type createProjectRequest struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type createEnvironmentRequest struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type createFlagRequest struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	BoolValue   bool   `json:"bool_value"`
}

type rotateAPIKeyResponse struct {
	APIKey string `json:"api_key"`
}

func New(cfg config.Config, logger *log.Logger, admin adminService) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler(cfg))
	mux.HandleFunc("/readyz", healthHandler(cfg))
	mux.HandleFunc("/v1/projects", createProjectHandler(admin))
	mux.HandleFunc("/v1/projects/", projectSubresourceHandler(admin))
	mux.HandleFunc("/v1/environments/", environmentSubresourceHandler(admin))

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

func createProjectHandler(admin adminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}

		var request createProjectRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		project, err := admin.CreateProject(r.Context(), application.CreateProjectInput{
			Key:  request.Key,
			Name: request.Name,
		})
		if err != nil {
			writeApplicationError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, project)
	}
}

func projectSubresourceHandler(admin adminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, suffix, ok := parseResourcePath(r.URL.Path, "/v1/projects/")
		if !ok || suffix != "/environments" {
			http.NotFound(w, r)
			return
		}

		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}

		var request createEnvironmentRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		environment, err := admin.CreateEnvironment(r.Context(), application.CreateEnvironmentInput{
			ProjectID: projectID,
			Key:       request.Key,
			Name:      request.Name,
		})
		if err != nil {
			writeApplicationError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, environment)
	}
}

func environmentSubresourceHandler(admin adminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		environmentID, suffix, ok := parseResourcePath(r.URL.Path, "/v1/environments/")
		if !ok {
			http.NotFound(w, r)
			return
		}

		switch {
		case suffix == "/flags":
			if r.Method != http.MethodPost {
				writeMethodNotAllowed(w, http.MethodPost)
				return
			}

			var request createFlagRequest
			if err := decodeJSON(r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			flag, err := admin.CreateFlag(r.Context(), application.CreateFlagInput{
				EnvironmentID: environmentID,
				Key:           request.Key,
				Name:          request.Name,
				Description:   request.Description,
				Enabled:       request.Enabled,
				BoolValue:     request.BoolValue,
			})
			if err != nil {
				writeApplicationError(w, err)
				return
			}

			writeJSON(w, http.StatusCreated, flag)
		case suffix == "/api-keys:rotate":
			if r.Method != http.MethodPost {
				writeMethodNotAllowed(w, http.MethodPost)
				return
			}

			token, err := admin.RotateAPIKey(r.Context(), environmentID)
			if err != nil {
				writeApplicationError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, rotateAPIKeyResponse{APIKey: token})
		default:
			http.NotFound(w, r)
		}
	}
}

func parseResourcePath(path, prefix string) (int64, string, bool) {
	if !strings.HasPrefix(path, prefix) {
		return 0, "", false
	}

	trimmed := strings.TrimPrefix(path, prefix)
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 {
		return 0, "", false
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, "", false
	}

	return id, "/" + parts[1], true
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return err
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeMethodNotAllowed(w http.ResponseWriter, method string) {
	w.Header().Set("Allow", method)
	writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
}

func writeApplicationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, application.ErrConflict):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, domain.ErrInvalidKey),
		errors.Is(err, domain.ErrInvalidName),
		errors.Is(err, domain.ErrUnsupportedFlagType):
		writeError(w, http.StatusBadRequest, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}
