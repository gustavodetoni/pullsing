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

func TestListFlagsHandler(t *testing.T) {
	t.Parallel()

	admin := &fakeAdminService{
		listFlagsFn: func(_ context.Context, environmentID int64, includeArchived bool) ([]domain.Flag, error) {
			if environmentID != 12 {
				t.Fatalf("ListFlags() environmentID = %d, want 12", environmentID)
			}
			if !includeArchived {
				t.Fatal("ListFlags() includeArchived = false, want true")
			}

			return []domain.Flag{
				{
					ID:            5,
					EnvironmentID: environmentID,
					Key:           "new-checkout",
					Name:          "New checkout",
					Type:          domain.FlagTypeBool,
				},
			}, nil
		},
	}

	server := New(config.Config{HTTPAddr: ":0", AppName: "pullsing", Environment: "test"}, log.Default(), admin)
	request := httptest.NewRequest(http.MethodGet, "/v1/environments/12/flags?include_archived=true", nil)
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response []domain.Flag
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(response) != 1 || response[0].Key != "new-checkout" {
		t.Fatalf("response = %#v, want one new-checkout flag", response)
	}
}

func TestGetFlagHandler(t *testing.T) {
	t.Parallel()

	admin := &fakeAdminService{
		getFlagFn: func(_ context.Context, environmentID, flagID int64) (domain.Flag, error) {
			return domain.Flag{
				ID:            flagID,
				EnvironmentID: environmentID,
				Key:           "new-checkout",
				Name:          "New checkout",
				Type:          domain.FlagTypeBool,
			}, nil
		},
	}

	server := New(config.Config{HTTPAddr: ":0", AppName: "pullsing", Environment: "test"}, log.Default(), admin)
	request := httptest.NewRequest(http.MethodGet, "/v1/environments/12/flags/5", nil)
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response domain.Flag
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if response.ID != 5 {
		t.Fatalf("response id = %d, want 5", response.ID)
	}
}

func TestUpdateFlagHandler(t *testing.T) {
	t.Parallel()

	admin := &fakeAdminService{
		updateFlagFn: func(_ context.Context, input application.UpdateFlagInput) (domain.Flag, error) {
			if input.FlagID != 5 {
				t.Fatalf("UpdateFlag() flagID = %d, want 5", input.FlagID)
			}
			if input.Name == nil || *input.Name != "Checkout V2" {
				t.Fatalf("UpdateFlag() name = %#v, want Checkout V2", input.Name)
			}

			return domain.Flag{
				ID:            input.FlagID,
				EnvironmentID: input.EnvironmentID,
				Key:           "new-checkout",
				Name:          *input.Name,
				Type:          domain.FlagTypeBool,
				Revision:      8,
			}, nil
		},
	}

	server := New(config.Config{HTTPAddr: ":0", AppName: "pullsing", Environment: "test"}, log.Default(), admin)

	body := bytes.NewBufferString(`{"name":"Checkout V2"}`)
	request := httptest.NewRequest(http.MethodPatch, "/v1/environments/12/flags/5", body)
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response domain.Flag
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if response.Revision != 8 {
		t.Fatalf("response revision = %d, want 8", response.Revision)
	}
}

func TestArchiveFlagHandler(t *testing.T) {
	t.Parallel()

	admin := &fakeAdminService{
		archiveFlagFn: func(_ context.Context, environmentID, flagID int64) (domain.Flag, error) {
			return domain.Flag{
				ID:            flagID,
				EnvironmentID: environmentID,
				Key:           "new-checkout",
				Type:          domain.FlagTypeBool,
				Revision:      9,
			}, nil
		},
	}

	server := New(config.Config{HTTPAddr: ":0", AppName: "pullsing", Environment: "test"}, log.Default(), admin)
	request := httptest.NewRequest(http.MethodDelete, "/v1/environments/12/flags/5", nil)
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

type fakeAdminService struct {
	createProjectFn     func(context.Context, application.CreateProjectInput) (domain.Project, error)
	createEnvironmentFn func(context.Context, application.CreateEnvironmentInput) (application.EnvironmentWithAPIKey, error)
	rotateAPIKeyFn      func(context.Context, int64) (string, error)
	listFlagsFn         func(context.Context, int64, bool) ([]domain.Flag, error)
	getFlagFn           func(context.Context, int64, int64) (domain.Flag, error)
	createFlagFn        func(context.Context, application.CreateFlagInput) (domain.Flag, error)
	updateFlagFn        func(context.Context, application.UpdateFlagInput) (domain.Flag, error)
	archiveFlagFn       func(context.Context, int64, int64) (domain.Flag, error)
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

func (f *fakeAdminService) ListFlags(ctx context.Context, environmentID int64, includeArchived bool) ([]domain.Flag, error) {
	if f.listFlagsFn == nil {
		return nil, nil
	}
	return f.listFlagsFn(ctx, environmentID, includeArchived)
}

func (f *fakeAdminService) GetFlag(ctx context.Context, environmentID, flagID int64) (domain.Flag, error) {
	if f.getFlagFn == nil {
		return domain.Flag{}, nil
	}
	return f.getFlagFn(ctx, environmentID, flagID)
}

func (f *fakeAdminService) CreateFlag(ctx context.Context, input application.CreateFlagInput) (domain.Flag, error) {
	if f.createFlagFn == nil {
		return domain.Flag{}, nil
	}
	return f.createFlagFn(ctx, input)
}

func (f *fakeAdminService) UpdateFlag(ctx context.Context, input application.UpdateFlagInput) (domain.Flag, error) {
	if f.updateFlagFn == nil {
		return domain.Flag{}, nil
	}
	return f.updateFlagFn(ctx, input)
}

func (f *fakeAdminService) ArchiveFlag(ctx context.Context, environmentID, flagID int64) (domain.Flag, error) {
	if f.archiveFlagFn == nil {
		return domain.Flag{}, nil
	}
	return f.archiveFlagFn(ctx, environmentID, flagID)
}
