package application

import (
	"context"
	"errors"
	"testing"

	"github.com/gustavodetoni/pullsing/internal/domain"
)

func TestAdminServiceCreateFlagPublishesRedisEvent(t *testing.T) {
	t.Parallel()

	repo := &fakeAdminRepository{
		createFlagFn: func(_ context.Context, flag domain.Flag) (domain.Flag, error) {
			flag.ID = 17
			flag.Revision = 9
			return flag, nil
		},
	}
	publisher := &fakePublisher{}

	service := NewAdminService(repo, publisher)

	flag, err := service.CreateFlag(context.Background(), CreateFlagInput{
		EnvironmentID: 42,
		Key:           "checkout-redesign",
		Name:          "Checkout redesign",
		Description:   "Enable the new checkout",
		Enabled:       true,
		BoolValue:     true,
	})
	if err != nil {
		t.Fatalf("CreateFlag() error = %v", err)
	}

	if flag.Revision != 9 {
		t.Fatalf("CreateFlag() revision = %d, want 9", flag.Revision)
	}

	if publisher.lastEvent.EnvironmentID != 42 {
		t.Fatalf("PublishFlagChange() environment_id = %d, want 42", publisher.lastEvent.EnvironmentID)
	}

	if publisher.lastEvent.Revision != 9 {
		t.Fatalf("PublishFlagChange() revision = %d, want 9", publisher.lastEvent.Revision)
	}

	if len(publisher.lastEvent.ChangedKeys) != 1 || publisher.lastEvent.ChangedKeys[0] != "checkout-redesign" {
		t.Fatalf("PublishFlagChange() changed_keys = %#v, want [checkout-redesign]", publisher.lastEvent.ChangedKeys)
	}
}

func TestAdminServiceCreateFlagReturnsPublisherError(t *testing.T) {
	t.Parallel()

	repo := &fakeAdminRepository{
		createFlagFn: func(_ context.Context, flag domain.Flag) (domain.Flag, error) {
			flag.Revision = 3
			return flag, nil
		},
	}
	publisher := &fakePublisher{err: errors.New("redis unavailable")}

	service := NewAdminService(repo, publisher)

	_, err := service.CreateFlag(context.Background(), CreateFlagInput{
		EnvironmentID: 11,
		Key:           "new-nav",
		Name:          "New navigation",
	})
	if err == nil {
		t.Fatal("CreateFlag() error = nil, want publisher error")
	}
}

type fakeAdminRepository struct {
	createProjectFn               func(context.Context, domain.Project) (domain.Project, error)
	createEnvironmentWithAPIKeyFn func(context.Context, domain.Environment, string) (domain.Environment, error)
	rotateAPIKeyFn                func(context.Context, int64, string) error
	createFlagFn                  func(context.Context, domain.Flag) (domain.Flag, error)
}

func (f *fakeAdminRepository) CreateProject(ctx context.Context, project domain.Project) (domain.Project, error) {
	if f.createProjectFn == nil {
		return domain.Project{}, nil
	}
	return f.createProjectFn(ctx, project)
}

func (f *fakeAdminRepository) CreateEnvironmentWithAPIKey(ctx context.Context, environment domain.Environment, tokenHash string) (domain.Environment, error) {
	if f.createEnvironmentWithAPIKeyFn == nil {
		return environment, nil
	}
	return f.createEnvironmentWithAPIKeyFn(ctx, environment, tokenHash)
}

func (f *fakeAdminRepository) RotateAPIKey(ctx context.Context, environmentID int64, tokenHash string) error {
	if f.rotateAPIKeyFn == nil {
		return nil
	}
	return f.rotateAPIKeyFn(ctx, environmentID, tokenHash)
}

func (f *fakeAdminRepository) CreateFlag(ctx context.Context, flag domain.Flag) (domain.Flag, error) {
	if f.createFlagFn == nil {
		return flag, nil
	}
	return f.createFlagFn(ctx, flag)
}

type fakePublisher struct {
	lastEvent FlagChangeEvent
	err       error
}

func (f *fakePublisher) PublishFlagChange(_ context.Context, event FlagChangeEvent) error {
	f.lastEvent = event
	return f.err
}
