package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/gustavodetoni/pullsing/internal/domain"
)

func TestSDKServiceGetSnapshotAuthenticatesAndReturnsFlags(t *testing.T) {
	t.Parallel()

	const envKey = "psk_example"

	var receivedHash string
	repo := &fakeSDKRepository{
		getEnvironmentByAPIKeyHashFn: func(_ context.Context, tokenHash string) (domain.Environment, error) {
			receivedHash = tokenHash
			return domain.Environment{ID: 42}, nil
		},
		getSnapshotFn: func(_ context.Context, environmentID int64) (EnvironmentSnapshot, error) {
			if environmentID != 42 {
				t.Fatalf("GetSnapshot() environmentID = %d, want 42", environmentID)
			}

			return EnvironmentSnapshot{
				Revision: 7,
				Flags: []FlagState{
					{Key: "checkout-redesign", Enabled: true, BoolValue: true},
				},
			}, nil
		},
	}

	service := NewSDKService(repo)

	snapshot, err := service.GetSnapshot(context.Background(), envKey)
	if err != nil {
		t.Fatalf("GetSnapshot() error = %v", err)
	}

	expectedSum := sha256.Sum256([]byte(envKey))
	expectedHash := hex.EncodeToString(expectedSum[:])
	if receivedHash != expectedHash {
		t.Fatalf("AuthenticateEnvironment() hash = %q, want %q", receivedHash, expectedHash)
	}

	if snapshot.Revision != 7 {
		t.Fatalf("GetSnapshot() revision = %d, want 7", snapshot.Revision)
	}

	if len(snapshot.Flags) != 1 || snapshot.Flags[0].Key != "checkout-redesign" {
		t.Fatalf("GetSnapshot() flags = %#v, want checkout-redesign", snapshot.Flags)
	}
}

func TestSDKServiceListUpdatesSinceGroupsByRevision(t *testing.T) {
	t.Parallel()

	repo := &fakeSDKRepository{
		listFlagStatesSinceFn: func(_ context.Context, environmentID int64, sinceRevision uint64) (EnvironmentFlagChanges, error) {
			if environmentID != 42 {
				t.Fatalf("ListFlagStatesSince() environmentID = %d, want 42", environmentID)
			}
			if sinceRevision != 4 {
				t.Fatalf("ListFlagStatesSince() sinceRevision = %d, want 4", sinceRevision)
			}

			return EnvironmentFlagChanges{
				CurrentRevision: 7,
				Flags: []FlagState{
					{Key: "checkout-redesign", Enabled: true, BoolValue: true, Revision: 5},
					{Key: "legacy-banner", Archived: true, Revision: 6},
				},
			}, nil
		},
	}

	service := NewSDKService(repo)

	updates, err := service.ListUpdatesSince(context.Background(), 42, 4)
	if err != nil {
		t.Fatalf("ListUpdatesSince() error = %v", err)
	}

	if len(updates) != 3 {
		t.Fatalf("ListUpdatesSince() updates len = %d, want 3", len(updates))
	}

	if updates[0].Revision != 5 || len(updates[0].Mutations) != 1 || updates[0].Mutations[0].Type != MutationTypeUpsert {
		t.Fatalf("ListUpdatesSince() first update = %#v", updates[0])
	}

	if updates[1].Revision != 6 || len(updates[1].Mutations) != 1 || updates[1].Mutations[0].Type != MutationTypeDelete {
		t.Fatalf("ListUpdatesSince() second update = %#v", updates[1])
	}

	if updates[2].Revision != 7 || len(updates[2].Mutations) != 0 {
		t.Fatalf("ListUpdatesSince() trailing update = %#v, want no-op revision 7", updates[2])
	}
}

func TestSDKServiceListUpdatesSinceRejectsFutureRevision(t *testing.T) {
	t.Parallel()

	repo := &fakeSDKRepository{
		listFlagStatesSinceFn: func(_ context.Context, _ int64, _ uint64) (EnvironmentFlagChanges, error) {
			return EnvironmentFlagChanges{CurrentRevision: 3}, nil
		},
	}

	service := NewSDKService(repo)

	_, err := service.ListUpdatesSince(context.Background(), 42, 4)
	if err != ErrInvalidCursor {
		t.Fatalf("ListUpdatesSince() error = %v, want %v", err, ErrInvalidCursor)
	}
}

type fakeSDKRepository struct {
	getEnvironmentByAPIKeyHashFn func(context.Context, string) (domain.Environment, error)
	getSnapshotFn                func(context.Context, int64) (EnvironmentSnapshot, error)
	listFlagStatesSinceFn        func(context.Context, int64, uint64) (EnvironmentFlagChanges, error)
}

func (f *fakeSDKRepository) GetEnvironmentByAPIKeyHash(ctx context.Context, tokenHash string) (domain.Environment, error) {
	if f.getEnvironmentByAPIKeyHashFn == nil {
		return domain.Environment{}, nil
	}
	return f.getEnvironmentByAPIKeyHashFn(ctx, tokenHash)
}

func (f *fakeSDKRepository) GetSnapshot(ctx context.Context, environmentID int64) (EnvironmentSnapshot, error) {
	if f.getSnapshotFn == nil {
		return EnvironmentSnapshot{}, nil
	}
	return f.getSnapshotFn(ctx, environmentID)
}

func (f *fakeSDKRepository) ListFlagStatesSince(ctx context.Context, environmentID int64, sinceRevision uint64) (EnvironmentFlagChanges, error) {
	if f.listFlagStatesSinceFn == nil {
		return EnvironmentFlagChanges{}, nil
	}
	return f.listFlagStatesSinceFn(ctx, environmentID, sinceRevision)
}
