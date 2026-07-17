package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Event struct {
	ID     uint64 `json:"id"`
	Type   string `json:"type"`
	Action string `json:"action,omitempty"`
	ItemID string `json:"item_id,omitempty"`
}

type Hub struct {
	sequence atomic.Uint64
	mu       sync.Mutex
	clients  map[chan Event]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[chan Event]struct{})}
}

func (h *Hub) Publish(action, itemID string) {
	event := Event{ID: h.sequence.Add(1), Type: "items_changed", Action: action, ItemID: itemID}
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients {
		select {
		case client <- event:
		default:
			// Dropping is safe: SSE is only a hint and clients reload persisted state.
		}
	}
}

func (h *Hub) Subscribe() (<-chan Event, func()) {
	channel := make(chan Event, 8)
	h.mu.Lock()
	h.clients[channel] = struct{}{}
	h.mu.Unlock()
	return channel, func() {
		h.mu.Lock()
		delete(h.clients, channel)
		h.mu.Unlock()
	}
}

func (h *Hub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

func (h *Hub) current() uint64 { return h.sequence.Load() }

func serveEvents(w http.ResponseWriter, r *http.Request, hub *Hub, heartbeat time.Duration) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return &apiError{Status: http.StatusInternalServerError, Code: "streaming_unsupported", Message: "Streaming is unavailable"}
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	events, unsubscribe := hub.Subscribe()
	defer unsubscribe()
	if err := writeSSE(w, Event{ID: hub.current(), Type: "sync"}); err != nil {
		return err
	}
	flusher.Flush()

	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return nil
		case event := <-events:
			if err := writeSSE(w, event); err != nil {
				return err
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return err
			}
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.ID, event.Type, data)
	return err
}
