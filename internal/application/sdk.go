package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/gustavodetoni/pullsing/internal/domain"
)

var (
	ErrUnauthorized  = errors.New("unauthorized")
	ErrInvalidCursor = errors.New("invalid cursor")
)

type SDKRepository interface {
	GetEnvironmentByAPIKeyHash(ctx context.Context, tokenHash string) (domain.Environment, error)
	GetSnapshot(ctx context.Context, environmentID int64) (EnvironmentSnapshot, error)
	ListFlagStatesSince(ctx context.Context, environmentID int64, sinceRevision uint64) (EnvironmentFlagChanges, error)
}

type FlagState struct {
	Key       string
	Enabled   bool
	BoolValue bool
	Archived  bool
	Revision  uint64
}

type EnvironmentSnapshot struct {
	Revision uint64
	Flags    []FlagState
}

type EnvironmentFlagChanges struct {
	CurrentRevision uint64
	Flags           []FlagState
}

type SnapshotFlag struct {
	Key       string
	Enabled   bool
	BoolValue bool
}

type Snapshot struct {
	EnvironmentID int64
	Revision      uint64
	Flags         []SnapshotFlag
}

type MutationType int

const (
	MutationTypeUnspecified MutationType = iota
	MutationTypeUpsert
	MutationTypeDelete
)

type Mutation struct {
	Type MutationType
	Key  string
	Flag SnapshotFlag
}

type Update struct {
	Revision  uint64
	Mutations []Mutation
}

type SDKService struct {
	repo SDKRepository
}

func NewSDKService(repo SDKRepository) *SDKService {
	return &SDKService{repo: repo}
}

func (s *SDKService) AuthenticateEnvironment(ctx context.Context, envKey string) (domain.Environment, error) {
	if envKey == "" {
		return domain.Environment{}, ErrUnauthorized
	}

	sum := sha256.Sum256([]byte(envKey))
	environment, err := s.repo.GetEnvironmentByAPIKeyHash(ctx, hex.EncodeToString(sum[:]))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Environment{}, ErrUnauthorized
		}
		return domain.Environment{}, err
	}

	return environment, nil
}

func (s *SDKService) GetSnapshot(ctx context.Context, envKey string) (Snapshot, error) {
	environment, err := s.AuthenticateEnvironment(ctx, envKey)
	if err != nil {
		return Snapshot{}, err
	}

	snapshot, err := s.repo.GetSnapshot(ctx, environment.ID)
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		EnvironmentID: environment.ID,
		Revision:      snapshot.Revision,
		Flags:         mapSnapshotFlags(snapshot.Flags),
	}, nil
}

func (s *SDKService) ListUpdatesSince(ctx context.Context, environmentID int64, sinceRevision uint64) ([]Update, error) {
	changes, err := s.repo.ListFlagStatesSince(ctx, environmentID, sinceRevision)
	if err != nil {
		return nil, err
	}

	if sinceRevision > changes.CurrentRevision {
		return nil, ErrInvalidCursor
	}

	if len(changes.Flags) == 0 {
		if changes.CurrentRevision == sinceRevision {
			return nil, nil
		}

		return []Update{{Revision: changes.CurrentRevision}}, nil
	}

	updates := make([]Update, 0, len(changes.Flags))
	for _, state := range changes.Flags {
		mutation := Mutation{
			Type: MutationTypeUpsert,
			Key:  state.Key,
			Flag: SnapshotFlag{
				Key:       state.Key,
				Enabled:   state.Enabled,
				BoolValue: state.BoolValue,
			},
		}
		if state.Archived {
			mutation.Type = MutationTypeDelete
		}

		if len(updates) == 0 || updates[len(updates)-1].Revision != state.Revision {
			updates = append(updates, Update{
				Revision:  state.Revision,
				Mutations: []Mutation{mutation},
			})
			continue
		}

		updates[len(updates)-1].Mutations = append(updates[len(updates)-1].Mutations, mutation)
	}

	if lastRevision := updates[len(updates)-1].Revision; lastRevision < changes.CurrentRevision {
		updates = append(updates, Update{Revision: changes.CurrentRevision})
	}

	return updates, nil
}

func mapSnapshotFlags(states []FlagState) []SnapshotFlag {
	if len(states) == 0 {
		return nil
	}

	flags := make([]SnapshotFlag, 0, len(states))
	for _, state := range states {
		if state.Archived {
			continue
		}

		flags = append(flags, SnapshotFlag{
			Key:       state.Key,
			Enabled:   state.Enabled,
			BoolValue: state.BoolValue,
		})
	}

	return flags
}
