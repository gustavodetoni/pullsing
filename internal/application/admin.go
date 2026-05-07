package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/gustavodetoni/pullsing/internal/domain"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type AdminRepository interface {
	CreateProject(ctx context.Context, project domain.Project) (domain.Project, error)
	CreateEnvironmentWithAPIKey(ctx context.Context, environment domain.Environment, tokenHash string) (domain.Environment, error)
	RotateAPIKey(ctx context.Context, environmentID int64, tokenHash string) error
	CreateFlag(ctx context.Context, flag domain.Flag) (domain.Flag, error)
}

type EventPublisher interface {
	PublishFlagChange(ctx context.Context, event FlagChangeEvent) error
}

type FlagChangeEvent struct {
	EnvironmentID int64    `json:"environment_id"`
	Revision      int64    `json:"revision"`
	ChangedKeys   []string `json:"changed_keys"`
}

type AdminService struct {
	repo      AdminRepository
	publisher EventPublisher
}

type CreateProjectInput struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type CreateEnvironmentInput struct {
	ProjectID int64  `json:"project_id"`
	Key       string `json:"key"`
	Name      string `json:"name"`
}

type EnvironmentWithAPIKey struct {
	Environment domain.Environment `json:"environment"`
	APIKey      string             `json:"api_key"`
}

type CreateFlagInput struct {
	EnvironmentID int64  `json:"environment_id"`
	Key           string `json:"key"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Enabled       bool   `json:"enabled"`
	BoolValue     bool   `json:"bool_value"`
}

func NewAdminService(repo AdminRepository, publisher EventPublisher) *AdminService {
	return &AdminService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *AdminService) CreateProject(ctx context.Context, input CreateProjectInput) (domain.Project, error) {
	project, err := domain.NewProject(input.Key, input.Name)
	if err != nil {
		return domain.Project{}, err
	}

	return s.repo.CreateProject(ctx, project)
}

func (s *AdminService) CreateEnvironment(ctx context.Context, input CreateEnvironmentInput) (EnvironmentWithAPIKey, error) {
	environment, err := domain.NewEnvironment(input.ProjectID, input.Key, input.Name)
	if err != nil {
		return EnvironmentWithAPIKey{}, err
	}

	token, tokenHash, err := newAPIKey()
	if err != nil {
		return EnvironmentWithAPIKey{}, err
	}

	environment, err = s.repo.CreateEnvironmentWithAPIKey(ctx, environment, tokenHash)
	if err != nil {
		return EnvironmentWithAPIKey{}, err
	}

	return EnvironmentWithAPIKey{
		Environment: environment,
		APIKey:      token,
	}, nil
}

func (s *AdminService) RotateAPIKey(ctx context.Context, environmentID int64) (string, error) {
	token, tokenHash, err := newAPIKey()
	if err != nil {
		return "", err
	}

	if err := s.repo.RotateAPIKey(ctx, environmentID, tokenHash); err != nil {
		return "", err
	}

	return token, nil
}

func (s *AdminService) CreateFlag(ctx context.Context, input CreateFlagInput) (domain.Flag, error) {
	flag, err := domain.NewBoolFlag(input.EnvironmentID, input.Key, input.Name, input.Description, input.Enabled, input.BoolValue)
	if err != nil {
		return domain.Flag{}, err
	}

	flag, err = s.repo.CreateFlag(ctx, flag)
	if err != nil {
		return domain.Flag{}, err
	}

	if err := s.publisher.PublishFlagChange(ctx, FlagChangeEvent{
		EnvironmentID: flag.EnvironmentID,
		Revision:      flag.Revision,
		ChangedKeys:   []string{flag.Key},
	}); err != nil {
		return domain.Flag{}, err
	}

	return flag, nil
}

func newAPIKey() (string, string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", "", fmt.Errorf("generate api key: %w", err)
	}

	token := "psk_" + base64.RawURLEncoding.EncodeToString(raw[:])
	sum := sha256.Sum256([]byte(token))

	return token, hex.EncodeToString(sum[:]), nil
}
