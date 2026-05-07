package grpc

import "testing"

func TestHubDisconnectsSlowSubscriber(t *testing.T) {
	t.Parallel()

	hub := NewHub(1)
	subscription := hub.Subscribe(42)

	hub.Publish(EnvironmentEvent{EnvironmentID: 42, Revision: 1})
	hub.Publish(EnvironmentEvent{EnvironmentID: 42, Revision: 2})

	if !subscription.Slow() {
		t.Fatal("expected subscription to be marked as slow")
	}

	for range subscription.Events() {
	}
}
