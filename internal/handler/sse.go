package handler

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)

// SSEHub manages Server-Sent Events subscribers per match.
type SSEHub struct {
	mu      sync.RWMutex
	clients map[string][]chan string
}

func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[string][]chan string),
	}
}

// Subscribe creates a new channel for the given matchID and returns it.
func (h *SSEHub) Subscribe(matchID string) chan string {
	ch := make(chan string, 8)
	h.mu.Lock()
	h.clients[matchID] = append(h.clients[matchID], ch)
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes a channel from the given matchID subscribers.
func (h *SSEHub) Unsubscribe(matchID string, ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	chans := h.clients[matchID]
	filtered := chans[:0]
	for _, c := range chans {
		if c != ch {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		delete(h.clients, matchID)
	} else {
		h.clients[matchID] = filtered
	}
	close(ch)
}

// Broadcast sends an event to all subscribers of the given matchID.
func (h *SSEHub) Broadcast(matchID, event string) {
	h.mu.RLock()
	chans := make([]chan string, len(h.clients[matchID]))
	copy(chans, h.clients[matchID])
	h.mu.RUnlock()

	for _, ch := range chans {
		select {
		case ch <- event:
		default:
			log.Printf("SSE channel full for match %s, dropping event", matchID)
		}
	}
}

// SSEHandler streams events to the client for a specific match.
func SSEHandler(hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		if matchID == "" {
			http.Error(w, "missing match id", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		ch := hub.Subscribe(matchID)
		defer hub.Unsubscribe(matchID, ch)

		// Send initial ping
		fmt.Fprintf(w, "event: ping\ndata: connected\n\n")
		flusher.Flush()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "event: update\ndata: %s\n\n", event)
				flusher.Flush()
			}
		}
	}
}
