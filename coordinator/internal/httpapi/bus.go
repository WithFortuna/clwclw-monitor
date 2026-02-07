package httpapi

import (
	"sync"
	"time"
)

type busEvent struct {
	Type string    `json:"type"`
	Time time.Time `json:"time"`
}

type eventBus struct {
	mu   sync.Mutex
	subs map[chan busEvent]struct{}
}

func newEventBus() *eventBus {
	return &eventBus{subs: make(map[chan busEvent]struct{})}
}

func (b *eventBus) Subscribe() chan busEvent {
	ch := make(chan busEvent, 32)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *eventBus) Unsubscribe(ch chan busEvent) {
	if ch == nil {
		return
	}
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *eventBus) Publish(typ string) {
	if typ == "" {
		typ = "update"
	}
	ev := busEvent{Type: typ, Time: time.Now().UTC()}

	b.mu.Lock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
			// drop if subscriber is slow
		}
	}
	b.mu.Unlock()
}

