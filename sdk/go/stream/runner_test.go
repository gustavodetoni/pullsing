package stream

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	pullsingv1 "github.com/gustavodetoni/pullsing/proto/gen/go/pullsing/v1"
	"github.com/gustavodetoni/pullsing/sdk/go/cache"
)

func TestRunnerLoadsSnapshotAndAppliesUpdates(t *testing.T) {
	service := &fakeService{
		snapshots: []*pullsingv1.Snapshot{
			{
				Revision: 1,
				Flags: []*pullsingv1.Flag{
					{Key: "new_button", Enabled: true, BoolValue: true},
				},
			},
		},
		streams: []*fakeUpdateStream{
			{
				updates: []*pullsingv1.Update{
					{
						Revision: 2,
						Mutations: []*pullsingv1.FlagMutation{
							{
								Type: pullsingv1.MutationType_MUTATION_TYPE_UPSERT,
								Key:  "other_button",
								Flag: &pullsingv1.Flag{Key: "other_button", Enabled: true, BoolValue: true},
							},
						},
					},
				},
				err: context.Canceled,
			},
		},
	}

	store := cache.NewStore()
	observer := &fakeObserver{}
	runner := NewRunner(service, store, observer, RunnerConfig{EnvAPIKey: "env-api-key"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := runner.Run(ctx)
	if err != context.Canceled {
		t.Fatalf("expected context canceled, got %v", err)
	}

	if !store.Enabled("new_button") {
		t.Fatalf("expected snapshot flag to be available")
	}
	if !store.Enabled("other_button") {
		t.Fatalf("expected update flag to be available")
	}
	if store.Revision() != 2 {
		t.Fatalf("expected revision 2, got %d", store.Revision())
	}
	if len(observer.states) < 2 {
		t.Fatalf("expected observer to receive state transitions")
	}
}

func TestRunnerReconnectsAndKeepsLastSnapshot(t *testing.T) {
	service := &fakeService{
		snapshots: []*pullsingv1.Snapshot{
			{
				Revision: 1,
				Flags: []*pullsingv1.Flag{
					{Key: "new_button", Enabled: true, BoolValue: true},
				},
			},
			{
				Revision: 2,
				Flags: []*pullsingv1.Flag{
					{Key: "new_button", Enabled: true, BoolValue: true},
					{Key: "secondary", Enabled: true, BoolValue: true},
				},
			},
		},
		streams: []*fakeUpdateStream{
			{err: io.EOF},
			{err: context.Canceled},
		},
	}

	store := cache.NewStore()
	runner := NewRunner(service, store, nil, RunnerConfig{
		EnvAPIKey: "env-api-key",
		Backoff: BackoffConfig{
			Min:    time.Millisecond,
			Max:    time.Millisecond,
			Jitter: 0,
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := runner.Run(ctx)
	if err != context.Canceled {
		t.Fatalf("expected context canceled, got %v", err)
	}

	if !store.Enabled("new_button") {
		t.Fatalf("expected previous snapshot to remain available")
	}
	if !store.Enabled("secondary") {
		t.Fatalf("expected second snapshot to be applied after reconnect")
	}
	if service.snapshotCalls != 2 {
		t.Fatalf("expected 2 snapshot calls, got %d", service.snapshotCalls)
	}
}

type fakeService struct {
	mu            sync.Mutex
	snapshotCalls int
	streamCalls   int
	snapshots     []*pullsingv1.Snapshot
	streams       []*fakeUpdateStream
}

func (f *fakeService) GetSnapshot(_ context.Context, _ string) (*pullsingv1.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	index := f.snapshotCalls
	f.snapshotCalls++
	if index >= len(f.snapshots) {
		return f.snapshots[len(f.snapshots)-1], nil
	}
	return f.snapshots[index], nil
}

func (f *fakeService) StreamUpdates(_ context.Context, _ string, _ uint64) (UpdateStream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	index := f.streamCalls
	f.streamCalls++
	if index >= len(f.streams) {
		return &fakeUpdateStream{err: context.Canceled}, nil
	}
	return f.streams[index], nil
}

type fakeUpdateStream struct {
	updates []*pullsingv1.Update
	index   int
	err     error
}

func (f *fakeUpdateStream) Recv() (*pullsingv1.Update, error) {
	if f.index >= len(f.updates) {
		return nil, f.err
	}

	update := f.updates[f.index]
	f.index++
	return update, nil
}

func (f *fakeUpdateStream) Close() error {
	return nil
}

type fakeObserver struct {
	states []State
}

func (f *fakeObserver) OnStateChange(state State) {
	f.states = append(f.states, state)
}
