package httpapi

import (
	"sync"
	"time"
)

const (
	EventAgents   = "agents"
	EventTasks    = "tasks"
	EventChannels = "channels"
	EventChains   = "chains"
	EventInputs   = "inputs"
	EventEvents   = "events"
	EventUpdate   = "update"
)

type busEvent struct {
	Type string    `json:"type"`
	Time time.Time `json:"time"`
}

type subscriber struct {
	ch     chan busEvent
	userID string
}

type eventBus struct {
	mu   sync.Mutex
	subs map[chan busEvent]subscriber
}

func newEventBus() *eventBus {
	return &eventBus{subs: make(map[chan busEvent]subscriber)}
}

func (b *eventBus) Subscribe(userID string) chan busEvent {
	ch := make(chan busEvent, 32)
	b.mu.Lock()
	b.subs[ch] = subscriber{ch: ch, userID: userID}
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

func (b *eventBus) Publish(typ string, userID string) {
	if typ == "" {
		typ = EventUpdate
	}
	ev := busEvent{Type: typ, Time: time.Now().UTC()}

	b.mu.Lock()
	for _, sub := range b.subs {
		// Send to subscriber if: no userID filter on event, or subscriber has no userID, or they match
		if userID == "" || sub.userID == "" || sub.userID == userID {
			select {
			case sub.ch <- ev:
			default:
				// drop if subscriber is slow
			}
		}
	}
	b.mu.Unlock()
}
