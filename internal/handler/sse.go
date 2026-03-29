package handler

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// SSEHub manages Server-Sent Events subscribers per match.
type SSEHub struct {
	mu       sync.RWMutex
	clients  map[string][]chan string
	push     *PushManager
	// presence tracks online unique_ids per channel: channel -> uid -> connection count
	presence map[string]map[string]int
}

func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients:  make(map[string][]chan string),
		presence: make(map[string]map[string]int),
	}
}

// SetPushManager conecta o PushManager ao SSEHub.
// Quando um Broadcast é feito, o push também é disparado para dispositivos
// que estão em background.
func (h *SSEHub) SetPushManager(pm *PushManager) {
	h.push = pm
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
// Também envia push notification para dispositivos em background.
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

	// Dispara push notification para dispositivos em background
	if h.push != nil {
		h.push.NotifyMatch(matchID, event)
	}
}

// PresenceJoin registers a unique_id as online for a channel.
func (h *SSEHub) PresenceJoin(channel, uniqueID string) {
	if uniqueID == "" {
		return
	}
	h.mu.Lock()
	if h.presence[channel] == nil {
		h.presence[channel] = make(map[string]int)
	}
	h.presence[channel][uniqueID]++
	h.mu.Unlock()
}

// PresenceLeave unregisters a unique_id from a channel.
func (h *SSEHub) PresenceLeave(channel, uniqueID string) {
	if uniqueID == "" {
		return
	}
	h.mu.Lock()
	if m := h.presence[channel]; m != nil {
		m[uniqueID]--
		if m[uniqueID] <= 0 {
			delete(m, uniqueID)
		}
		if len(m) == 0 {
			delete(h.presence, channel)
		}
	}
	h.mu.Unlock()
}

// OnlineUIDs returns the set of unique_ids currently connected to a channel.
func (h *SSEHub) OnlineUIDs(channel string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	m := h.presence[channel]
	if len(m) == 0 {
		return nil
	}
	uids := make([]string, 0, len(m))
	for uid := range m {
		uids = append(uids, uid)
	}
	return uids
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
		_, _ = fmt.Fprintf(w, "event: ping\ndata: connected\n\n")
		flusher.Flush()

		ctx := r.Context()
		// Keep-alive ticker: envia comentário vazio a cada 20s para manter
		// conexões SSE ativas em redes móveis (4G/5G) que encerram conexões ociosas.
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Comentário SSE (linhas começando com ":") não geram eventos no cliente
				_, _ = fmt.Fprintf(w, ": keep-alive\n\n")
				flusher.Flush()
			case event, ok := <-ch:
				if !ok {
					return
				}
				_, _ = fmt.Fprintf(w, "event: update\ndata: %s\n\n", event)
				flusher.Flush()
			}
		}
	}
}
