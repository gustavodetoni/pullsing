package grpc

import (
	"sync"
	"sync/atomic"
)

type EnvironmentEvent struct {
	EnvironmentID int64
	Revision      uint64
}

type Subscription struct {
	hub           *Hub
	environmentID int64
	events        chan EnvironmentEvent
	slow          atomic.Bool
}

func (s *Subscription) Events() <-chan EnvironmentEvent {
	return s.events
}

func (s *Subscription) Slow() bool {
	return s.slow.Load()
}

func (s *Subscription) Close() {
	s.hub.remove(s, false)
}

type Hub struct {
	mu         sync.Mutex
	bufferSize int
	subs       map[int64]map[*Subscription]struct{}
}

func NewHub(bufferSize int) *Hub {
	if bufferSize <= 0 {
		bufferSize = 1
	}

	return &Hub{
		bufferSize: bufferSize,
		subs:       make(map[int64]map[*Subscription]struct{}),
	}
}

func (h *Hub) Subscribe(environmentID int64) *Subscription {
	h.mu.Lock()
	defer h.mu.Unlock()

	subscription := &Subscription{
		hub:           h,
		environmentID: environmentID,
		events:        make(chan EnvironmentEvent, h.bufferSize),
	}

	if h.subs[environmentID] == nil {
		h.subs[environmentID] = make(map[*Subscription]struct{})
	}
	h.subs[environmentID][subscription] = struct{}{}

	return subscription
}

func (h *Hub) Publish(event EnvironmentEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for subscription := range h.subs[event.EnvironmentID] {
		select {
		case subscription.events <- event:
		default:
			h.removeLocked(subscription, true)
		}
	}
}

func (h *Hub) remove(subscription *Subscription, slow bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.removeLocked(subscription, slow)
}

func (h *Hub) removeLocked(subscription *Subscription, slow bool) {
	environmentSubscriptions := h.subs[subscription.environmentID]
	if _, ok := environmentSubscriptions[subscription]; !ok {
		return
	}

	if slow {
		subscription.slow.Store(true)
	}

	delete(environmentSubscriptions, subscription)
	close(subscription.events)

	if len(environmentSubscriptions) == 0 {
		delete(h.subs, subscription.environmentID)
	}
}
