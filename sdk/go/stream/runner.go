package stream

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/gustavodetoni/pullsing/sdk/go/cache"
	"github.com/gustavodetoni/pullsing/sdk/go/types"
)

type Logger interface {
	Printf(format string, v ...any)
}

type State struct {
	Connected    bool
	Revision     uint64
	LastError    error
	LastSyncTime time.Time
}

type Observer interface {
	OnStateChange(State)
}

type RunnerConfig struct {
	EnvAPIKey string
	Backoff   BackoffConfig
	Logger    Logger
}

type Runner struct {
	service  Service
	store    *cache.Store
	observer Observer
	cfg      RunnerConfig
	backoff  *Backoff
}

func NewRunner(service Service, store *cache.Store, observer Observer, cfg RunnerConfig) *Runner {
	return &Runner{
		service:  service,
		store:    store,
		observer: observer,
		cfg:      cfg,
		backoff:  NewBackoff(cfg.Backoff),
	}
}

func (r *Runner) Run(ctx context.Context) error {
	for {
		err := r.runOnce(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			return err
		}

		r.notify(State{
			Connected: false,
			Revision:  r.store.Revision(),
			LastError: err,
		})
		r.logf("pullsing sdk stream: reconnecting after error: %v", err)

		delay := r.backoff.Next()
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (r *Runner) runOnce(ctx context.Context) error {
	snapshot, err := r.service.GetSnapshot(ctx, r.cfg.EnvAPIKey)
	if err != nil {
		return err
	}

	r.store.Replace(types.SnapshotFromProto(snapshot))
	now := time.Now()
	r.notify(State{
		Connected:    true,
		Revision:     snapshot.GetRevision(),
		LastSyncTime: now,
	})
	r.backoff.Reset()

	stream, err := r.service.StreamUpdates(ctx, r.cfg.EnvAPIKey, snapshot.GetRevision())
	if err != nil {
		return err
	}
	defer func() {
		_ = stream.Close()
	}()

	for {
		update, err := stream.Recv()
		if err != nil {
			if isExpectedStreamEnd(err) {
				return err
			}
			return err
		}

		r.store.Apply(update.GetRevision(), types.MutationsFromProto(update.GetMutations()))
		r.notify(State{
			Connected:    true,
			Revision:     update.GetRevision(),
			LastSyncTime: time.Now(),
		})
	}
}

func (r *Runner) notify(state State) {
	if r.observer != nil {
		r.observer.OnStateChange(state)
	}
}

func (r *Runner) logf(format string, args ...any) {
	if r.cfg.Logger != nil {
		r.cfg.Logger.Printf(format, args...)
		return
	}

	log.Printf(format, args...)
}
