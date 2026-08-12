// SPDX-License-Identifier: AGPL-3.0-only

package devserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const eventsPath = "/__himesan/events"

// Diagnostic is a compiler/build diagnostic suitable for the development
// browser overlay. The CLI may map its compiler's native diagnostics through
// Options.MapDiagnostics without coupling this package to the compiler.
type Diagnostic struct {
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
}

// Event is delivered both to Options.OnEvent and to connected browser clients.
// Type is currently one of "ready", "reload", or "diagnostic".
type Event struct {
	Type        string       `json:"type"`
	Phase       string       `json:"phase,omitempty"`
	Message     string       `json:"message,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	At          time.Time    `json:"at"`
}

type eventHub struct {
	mu          sync.Mutex
	subscribers map[chan Event]struct{}
	latest      *Event
	closed      bool
}

func newEventHub() *eventHub {
	return &eventHub{subscribers: make(map[chan Event]struct{})}
}

func (h *eventHub) publish(event Event) {
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	if event.Type == "diagnostic" {
		copy := event
		h.latest = &copy
	} else if event.Type == "reload" {
		h.latest = nil
	}
	for subscriber := range h.subscribers {
		select {
		case subscriber <- event:
		default:
			// Reload and diagnostic events are snapshots, not a log. Replace the
			// oldest queued snapshot so a slow browser still receives the newest
			// state transition.
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- event:
			default:
			}
		}
	}
}

func (h *eventHub) subscribe() (<-chan Event, func()) {
	updates := make(chan Event, 8)
	h.mu.Lock()
	if h.closed {
		close(updates)
		h.mu.Unlock()
		return updates, func() {}
	}
	h.subscribers[updates] = struct{}{}
	if h.latest != nil {
		updates <- *h.latest
	}
	h.mu.Unlock()

	var once sync.Once
	return updates, func() {
		once.Do(func() {
			h.mu.Lock()
			if _, ok := h.subscribers[updates]; ok {
				delete(h.subscribers, updates)
				close(updates)
			}
			h.mu.Unlock()
		})
	}
}

func (h *eventHub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for subscriber := range h.subscribers {
		close(subscriber)
		delete(h.subscribers, subscriber)
	}
}

func (h *eventHub) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if err := writeSSE(w, Event{Type: "ready", At: time.Now().UTC()}); err != nil {
		return
	}
	flusher.Flush()

	updates, unsubscribe := h.subscribe()
	defer unsubscribe()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-updates:
			if !ok {
				return
			}
			if err := writeSSE(w, event); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event.Type); err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", payload)
	return err
}
