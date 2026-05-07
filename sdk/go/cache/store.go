package cache

import (
	"sync/atomic"

	"github.com/gustavodetoni/pullsing/sdk/go/evaluation"
	"github.com/gustavodetoni/pullsing/sdk/go/types"
)

type Store struct {
	current atomic.Pointer[types.Snapshot]
}

func NewStore() *Store {
	store := &Store{}
	store.current.Store(&types.Snapshot{Flags: map[string]types.BoolFlag{}})
	return store
}

func (s *Store) Load() types.Snapshot {
	current := s.current.Load()
	if current == nil {
		return types.Snapshot{Flags: map[string]types.BoolFlag{}}
	}

	return *current
}

func (s *Store) Replace(snapshot types.Snapshot) {
	s.current.Store(cloneSnapshot(snapshot))
}

func (s *Store) Apply(revision uint64, mutations []types.Mutation) {
	current := s.Load()
	next := types.Snapshot{
		Revision: revision,
		Flags:    cloneFlags(current.Flags),
	}

	for _, mutation := range mutations {
		switch mutation.Type {
		case types.MutationTypeDelete:
			delete(next.Flags, mutation.Key)
		case types.MutationTypeUpsert:
			flag := mutation.Flag
			if flag.Key == "" {
				flag.Key = mutation.Key
			}
			if flag.Key == "" {
				continue
			}
			next.Flags[flag.Key] = flag
		}
	}

	s.current.Store(&next)
}

func (s *Store) Enabled(key string) bool {
	return evaluation.Enabled(s.Load(), key)
}

func (s *Store) Revision() uint64 {
	return s.Load().Revision
}

func cloneSnapshot(snapshot types.Snapshot) *types.Snapshot {
	return &types.Snapshot{
		Revision: snapshot.Revision,
		Flags:    cloneFlags(snapshot.Flags),
	}
}

func cloneFlags(in map[string]types.BoolFlag) map[string]types.BoolFlag {
	if len(in) == 0 {
		return map[string]types.BoolFlag{}
	}

	out := make(map[string]types.BoolFlag, len(in))
	for key, flag := range in {
		out[key] = flag
	}

	return out
}
