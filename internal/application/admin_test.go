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

func TestAdminServiceUpdateFlagPublishesRedisEvent(t *testing.T) {
	t.Parallel()

	repo := &fakeAdminRepository{
		getFlagFn: func(_ context.Context, environmentID, flagID int64) (domain.Flag, error) {
			return domain.Flag{
				ID:            flagID,
				EnvironmentID: environmentID,
				Key:           "checkout-redesign",
				Name:          "Checkout redesign",
				Description:   "Enable the new checkout",
				Type:          domain.FlagTypeBool,
				Enabled:       true,
				BoolValue:     true,
			}, nil
		},
		updateFlagFn: func(_ context.Context, flag domain.Flag) (domain.Flag, error) {
			flag.Revision = 10
			return flag, nil
		},
	}
	publisher := &fakePublisher{}

	service := NewAdminService(repo, publisher)

	name := "Checkout V2"
	enabled := false
	flag, err := service.UpdateFlag(context.Background(), UpdateFlagInput{
		EnvironmentID: 42,
		FlagID:        17,
		Name:          &name,
		Enabled:       &enabled,
	})
	if err != nil {
		t.Fatalf("UpdateFlag() error = %v", err)
	}

	if flag.Name != name {
		t.Fatalf("UpdateFlag() name = %q, want %q", flag.Name, name)
	}

	if flag.Revision != 10 {
		t.Fatalf("UpdateFlag() revision = %d, want 10", flag.Revision)
	}

	if len(publisher.lastEvent.ChangedKeys) != 1 || publisher.lastEvent.ChangedKeys[0] != "checkout-redesign" {
		t.Fatalf("PublishFlagChange() changed_keys = %#v, want [checkout-redesign]", publisher.lastEvent.ChangedKeys)
	}
}

func TestAdminServiceUpdateFlagRejectsEmptyPatch(t *testing.T) {
	t.Parallel()

	service := NewAdminService(&fakeAdminRepository{}, &fakePublisher{})

	_, err := service.UpdateFlag(context.Background(), UpdateFlagInput{
		EnvironmentID: 42,
		FlagID:        17,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("UpdateFlag() error = %v, want ErrInvalidInput", err)
	}
}

func TestAdminServiceArchiveFlagPublishesRedisEvent(t *testing.T) {
	t.Parallel()

	repo := &fakeAdminRepository{
		getFlagFn: func(_ context.Context, environmentID, flagID int64) (domain.Flag, error) {
			return domain.Flag{
				ID:            flagID,
				EnvironmentID: environmentID,
				Key:           "checkout-redesign",
				Name:          "Checkout redesign",
				Type:          domain.FlagTypeBool,
			}, nil
		},
		archiveFlagFn: func(_ context.Context, environmentID, flagID int64) (domain.Flag, error) {
			return domain.Flag{
				ID:            flagID,
				EnvironmentID: environmentID,
				Key:           "checkout-redesign",
				Name:          "Checkout redesign",
				Type:          domain.FlagTypeBool,
				Revision:      11,
			}, nil
		},
	}
	publisher := &fakePublisher{}

	service := NewAdminService(repo, publisher)

	flag, err := service.ArchiveFlag(context.Background(), 42, 17)
	if err != nil {
		t.Fatalf("ArchiveFlag() error = %v", err)
	}

	if flag.Revision != 11 {
		t.Fatalf("ArchiveFlag() revision = %d, want 11", flag.Revision)
	}

	if publisher.lastEvent.Revision != 11 {
		t.Fatalf("PublishFlagChange() revision = %d, want 11", publisher.lastEvent.Revision)
	}
}

type fakeAdminRepository struct {
	createProjectFn               func(context.Context, domain.Project) (domain.Project, error)
	createEnvironmentWithAPIKeyFn func(context.Context, domain.Environment, string) (domain.Environment, error)
	rotateAPIKeyFn                func(context.Context, int64, string) error
	listFlagsFn                   func(context.Context, int64, bool) ([]domain.Flag, error)
	getFlagFn                     func(context.Context, int64, int64) (domain.Flag, error)
	createFlagFn                  func(context.Context, domain.Flag) (domain.Flag, error)
	updateFlagFn                  func(context.Context, domain.Flag) (domain.Flag, error)
	archiveFlagFn                 func(context.Context, int64, int64) (domain.Flag, error)
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

func (f *fakeAdminRepository) ListFlags(ctx context.Context, environmentID int64, includeArchived bool) ([]domain.Flag, error) {
	if f.listFlagsFn == nil {
		return nil, nil
	}
	return f.listFlagsFn(ctx, environmentID, includeArchived)
}

func (f *fakeAdminRepository) GetFlag(ctx context.Context, environmentID, flagID int64) (domain.Flag, error) {
	if f.getFlagFn == nil {
		return domain.Flag{}, nil
	}
	return f.getFlagFn(ctx, environmentID, flagID)
}

func (f *fakeAdminRepository) CreateFlag(ctx context.Context, flag domain.Flag) (domain.Flag, error) {
	if f.createFlagFn == nil {
		return flag, nil
	}
	return f.createFlagFn(ctx, flag)
}

func (f *fakeAdminRepository) UpdateFlag(ctx context.Context, flag domain.Flag) (domain.Flag, error) {
	if f.updateFlagFn == nil {
		return flag, nil
	}
	return f.updateFlagFn(ctx, flag)
}

func (f *fakeAdminRepository) ArchiveFlag(ctx context.Context, environmentID, flagID int64) (domain.Flag, error) {
	if f.archiveFlagFn == nil {
		return domain.Flag{}, nil
	}
	return f.archiveFlagFn(ctx, environmentID, flagID)
}

type fakePublisher struct {
	lastEvent FlagChangeEvent
	err       error
}

func (f *fakePublisher) PublishFlagChange(_ context.Context, event FlagChangeEvent) error {
	f.lastEvent = event
	return f.err
}
