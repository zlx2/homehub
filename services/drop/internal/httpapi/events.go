package httpapi

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type eventHub struct {
	mu      sync.Mutex
	clients map[chan struct{}]struct{}
}

func newEventHub() *eventHub { return &eventHub{clients: make(map[chan struct{}]struct{})} }

func (hub *eventHub) subscribe() (chan struct{}, func()) {
	channel := make(chan struct{}, 1)
	hub.mu.Lock()
	hub.clients[channel] = struct{}{}
	hub.mu.Unlock()
	return channel, func() {
		hub.mu.Lock()
		delete(hub.clients, channel)
		hub.mu.Unlock()
	}
}

func (hub *eventHub) publish() {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	for channel := range hub.clients {
		select {
		case channel <- struct{}{}:
		default:
		}
	}
}

func (api *API) events(response http.ResponseWriter, request *http.Request) {
	flusher, ok := response.(http.Flusher)
	if !ok {
		writeError(response, http.StatusNotImplemented, "streaming_unavailable")
		return
	}
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("X-Accel-Buffering", "no")
	_, _ = fmt.Fprint(response, "event: ready\ndata: {}\n\n")
	flusher.Flush()
	changes, unsubscribe := api.eventsHub.subscribe()
	defer unsubscribe()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case <-changes:
			_, _ = fmt.Fprint(response, "event: items_changed\ndata: {}\n\n")
			flusher.Flush()
		case <-ticker.C:
			_, _ = fmt.Fprint(response, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
