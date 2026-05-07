package client

import (
	"context"
	"testing"
	"time"

	pullsingv1 "github.com/gustavodetoni/pullsing/proto/gen/go/pullsing/v1"
	"github.com/gustavodetoni/pullsing/sdk/go/stream"
)

func TestClientEnabledUsesLatestSnapshot(t *testing.T) {
	service := &stubService{
		snapshot: &pullsingv1.Snapshot{
			Revision: 1,
			Flags: []*pullsingv1.Flag{
				{Key: "new_button", Enabled: true, BoolValue: true},
			},
		},
	}

	client := NewClientWithService(service, Config{
		EnvAPIKey: "env-api-key",
		Backoff: stream.BackoffConfig{
			Min: time.Millisecond,
			Max: time.Millisecond,
		},
	}, nil)
	defer func() {
		_ = client.Close()
	}()

	waitFor(t, time.Second, func() bool {
		return client.Enabled("new_button")
	})

	health := client.Health()
	if !health.Connected {
		t.Fatalf("expected client to report connected")
	}
	if health.LastRevision != 1 {
		t.Fatalf("expected revision 1, got %d", health.LastRevision)
	}
}

type stubService struct {
	snapshot *pullsingv1.Snapshot
}

func (s *stubService) GetSnapshot(context.Context, string) (*pullsingv1.Snapshot, error) {
	return s.snapshot, nil
}

func (s *stubService) StreamUpdates(context.Context, string, uint64) (stream.UpdateStream, error) {
	return &stubUpdateStream{}, nil
}

type stubUpdateStream struct{}

func (s *stubUpdateStream) Recv() (*pullsingv1.Update, error) {
	return nil, context.Canceled
}

func (s *stubUpdateStream) Close() error {
	return nil
}

func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("condition was not met within %s", timeout)
}
