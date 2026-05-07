package http

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gustavodetoni/pullsing/internal/application"
	"github.com/gustavodetoni/pullsing/internal/domain"
	"github.com/gustavodetoni/pullsing/internal/infrastructure/config"
)

func TestCreateFlagHandler(t *testing.T) {
	t.Parallel()

	admin := &fakeAdminService{
		createFlagFn: func(_ context.Context, input application.CreateFlagInput) (domain.Flag, error) {
			return domain.Flag{
				ID:            5,
				EnvironmentID: input.EnvironmentID,
				Key:           input.Key,
				Name:          input.Name,
				Description:   input.Description,
				Type:          domain.FlagTypeBool,
				Enabled:       input.Enabled,
				BoolValue:     input.BoolValue,
				Revision:      7,
			}, nil
		},
	}

	server := New(config.Config{HTTPAddr: ":0", AppName: "pullsing", Environment: "test"}, log.Default(), admin)

	body := bytes.NewBufferString(`{"key":"new-checkout","name":"New checkout","description":"release gate","enabled":true,"bool_value":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/environments/12/flags", body)
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusCreated)
	}

	var response domain.Flag
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if response.EnvironmentID != 12 {
		t.Fatalf("response environment_id = %d, want 12", response.EnvironmentID)
	}

	if response.Revision != 7 {
		t.Fatalf("response revision = %d, want 7", response.Revision)
	}
}

type fakeAdminService struct {
	createProjectFn     func(context.Context, application.CreateProjectInput) (domain.Project, error)
	createEnvironmentFn func(context.Context, application.CreateEnvironmentInput) (application.EnvironmentWithAPIKey, error)
	rotateAPIKeyFn      func(context.Context, int64) (string, error)
	createFlagFn        func(context.Context, application.CreateFlagInput) (domain.Flag, error)
}

func (f *fakeAdminService) CreateProject(ctx context.Context, input application.CreateProjectInput) (domain.Project, error) {
	if f.createProjectFn == nil {
		return domain.Project{}, nil
	}
	return f.createProjectFn(ctx, input)
}

func (f *fakeAdminService) CreateEnvironment(ctx context.Context, input application.CreateEnvironmentInput) (application.EnvironmentWithAPIKey, error) {
	if f.createEnvironmentFn == nil {
		return application.EnvironmentWithAPIKey{}, nil
	}
	return f.createEnvironmentFn(ctx, input)
}

func (f *fakeAdminService) RotateAPIKey(ctx context.Context, environmentID int64) (string, error) {
	if f.rotateAPIKeyFn == nil {
		return "", nil
	}
	return f.rotateAPIKeyFn(ctx, environmentID)
}

func (f *fakeAdminService) CreateFlag(ctx context.Context, input application.CreateFlagInput) (domain.Flag, error) {
	if f.createFlagFn == nil {
		return domain.Flag{}, nil
	}
	return f.createFlagFn(ctx, input)
}
