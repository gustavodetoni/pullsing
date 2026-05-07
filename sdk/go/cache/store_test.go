package cache

import (
	"testing"

	"github.com/gustavodetoni/pullsing/sdk/go/types"
)

func TestStoreReplaceAndEnabled(t *testing.T) {
	store := NewStore()
	store.Replace(types.Snapshot{
		Revision: 3,
		Flags: map[string]types.BoolFlag{
			"new_button": {Key: "new_button", Enabled: true, Value: true},
		},
	})

	if !store.Enabled("new_button") {
		t.Fatalf("expected flag to be enabled")
	}
	if store.Revision() != 3 {
		t.Fatalf("expected revision 3, got %d", store.Revision())
	}
}

func TestStoreApplyKeepsPreviousSnapshotImmutable(t *testing.T) {
	store := NewStore()
	store.Replace(types.Snapshot{
		Revision: 1,
		Flags: map[string]types.BoolFlag{
			"new_button": {Key: "new_button", Enabled: true, Value: true},
		},
	})

	before := store.Load()
	store.Apply(2, []types.Mutation{
		{
			Type: types.MutationTypeUpsert,
			Key:  "old_button",
			Flag: types.BoolFlag{Key: "old_button", Enabled: true, Value: true},
		},
	})

	if _, ok := before.Flags["old_button"]; ok {
		t.Fatalf("expected previous snapshot map to remain immutable")
	}
	if !store.Enabled("old_button") {
		t.Fatalf("expected new mutation to be visible")
	}
}

func TestStoreApplyDelete(t *testing.T) {
	store := NewStore()
	store.Replace(types.Snapshot{
		Revision: 1,
		Flags: map[string]types.BoolFlag{
			"new_button": {Key: "new_button", Enabled: true, Value: true},
		},
	})

	store.Apply(2, []types.Mutation{
		{Type: types.MutationTypeDelete, Key: "new_button"},
	})

	if store.Enabled("new_button") {
		t.Fatalf("expected flag to be deleted")
	}
}
