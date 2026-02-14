package httpapi

import (
	"sync"
	"time"
)

const notificationCooldown = 5 * time.Minute

// Notification represents a stored notification visible to a user.
type Notification struct {
	Key       string         `json:"key"` // unique: "agentID:type"
	UserID    string         `json:"user_id"`
	AgentID   string         `json:"agent_id"`
	AgentName string         `json:"agent_name"`
	Type      string         `json:"type"`    // e.g. "setup_waiting"
	Channel   string         `json:"channel"` // first subscribed channel (may be empty)
	Message   string         `json:"message"`
	CreatedAt time.Time      `json:"created_at"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// notificationTracker stores active notifications per user and prevents duplicate sends.
type notificationTracker struct {
	mu    sync.Mutex
	sent  map[string]time.Time      // cooldown tracker, key: "agentID:type"
	items map[string][]Notification // stored notifications, key: userID
}

func newNotificationTracker() *notificationTracker {
	return &notificationTracker{
		sent:  make(map[string]time.Time),
		items: make(map[string][]Notification),
	}
}

func (n *notificationTracker) cooldownKey(agentID, typ string) string {
	return agentID + ":" + typ
}

// ShouldNotify returns true if this agent+type hasn't been notified within the cooldown.
func (n *notificationTracker) ShouldNotify(agentID, typ string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	k := n.cooldownKey(agentID, typ)
	last, ok := n.sent[k]
	if ok && time.Since(last) < notificationCooldown {
		return false
	}
	n.sent[k] = time.Now()
	return true
}

// Add stores a notification. Replaces any existing notification with the same key for this user.
func (n *notificationTracker) Add(notif Notification) {
	n.mu.Lock()
	defer n.mu.Unlock()

	list := n.items[notif.UserID]
	// Replace if exists
	for i, existing := range list {
		if existing.Key == notif.Key {
			list[i] = notif
			return
		}
	}
	n.items[notif.UserID] = append(list, notif)
}

// List returns all stored notifications for a user.
func (n *notificationTracker) List(userID string) []Notification {
	n.mu.Lock()
	defer n.mu.Unlock()

	list := n.items[userID]
	if list == nil {
		return []Notification{}
	}
	// Return a copy
	out := make([]Notification, len(list))
	copy(out, list)
	return out
}

// Dismiss removes a notification by agentID+type for a user and clears the cooldown.
func (n *notificationTracker) Dismiss(userID, agentID, typ string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	key := n.cooldownKey(agentID, typ)
	delete(n.sent, key)

	list := n.items[userID]
	for i, item := range list {
		if item.Key == key {
			n.items[userID] = append(list[:i], list[i+1:]...)
			return
		}
	}
}

// ClearByAgent removes all notifications for a specific agent+type across all users,
// and clears the cooldown. Used when agent exits setup_waiting.
func (n *notificationTracker) ClearByAgent(agentID, typ string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	key := n.cooldownKey(agentID, typ)
	delete(n.sent, key)

	for userID, list := range n.items {
		for i, item := range list {
			if item.Key == key {
				n.items[userID] = append(list[:i], list[i+1:]...)
				break
			}
		}
	}
}
