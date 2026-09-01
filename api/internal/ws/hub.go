// Package ws is the WebSocket hub: it keeps a registry of connected
// clients per user and fans out job event notifications to them. It never
// decides what counts as an event — internal/store's Postgres LISTEN
// forwards those in (see Listener in listener.go).
package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Event is the message shape pushed to clients over the WebSocket, one
// per job_events row written by either the API or a worker.
type Event struct {
	JobID     uuid.UUID       `json:"job_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

// Client is one connected WebSocket, registered under the user it
// authenticated as. Send is buffered so one slow reader cannot block the
// listener goroutine broadcasting to every other client.
type Client struct {
	UserID uuid.UUID
	Send   chan []byte
}

func newClient(userID uuid.UUID) *Client {
	return &Client{UserID: userID, Send: make(chan []byte, 16)}
}

type Hub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[uuid.UUID]map[*Client]struct{})}
}

// Register creates and returns a new client for userID.
func (h *Hub) Register(userID uuid.UUID) *Client {
	c := newClient(userID)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[*Client]struct{})
	}
	h.clients[userID][c] = struct{}{}
	return c
}

// Unregister removes c from the hub and closes its send channel. Safe to
// call more than once for the same client.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.clients[c.UserID]
	if !ok {
		return
	}
	if _, ok := set[c]; !ok {
		return
	}
	delete(set, c)
	if len(set) == 0 {
		delete(h.clients, c.UserID)
	}
	close(c.Send)
}

// BroadcastToUser sends msg to every client currently connected as
// userID. A client whose send buffer is full is dropped rather than
// allowed to stall the broadcast for everyone else — that client's next
// GET /jobs/{id} or the 5s polling fallback will catch it up.
func (h *Hub) BroadcastToUser(userID uuid.UUID, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[userID] {
		select {
		case c.Send <- msg:
		default:
		}
	}
}
