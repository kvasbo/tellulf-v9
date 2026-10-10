package web

import "sync"

// Event is one server-sent event: an HTML fragment for htmx to swap in.
type Event struct {
	Name string
	Data string
}

// Hub fans rendered fragments out to every connected browser. It remembers
// the latest version of each fragment so new subscribers start complete and
// unchanged fragments are never resent.
type Hub struct {
	mu    sync.Mutex
	order []string // fragment names in first-published order
	last  map[string]string
	subs  map[chan Event]struct{}
}

func NewHub() *Hub {
	return &Hub{last: map[string]string{}, subs: map[chan Event]struct{}{}}
}

// Publish sends a fragment to all subscribers if it differs from the last
// one published under that name.
func (h *Hub) Publish(name, data string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	prev, seen := h.last[name]
	if seen && prev == data {
		return
	}
	if !seen {
		h.order = append(h.order, name)
	}
	h.last[name] = data
	h.fanOut(Event{name, data})
}

// Broadcast sends an event to all subscribers without remembering it.
func (h *Hub) Broadcast(name, data string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fanOut(Event{name, data})
}

// fanOut must be called with h.mu held. A subscriber whose buffer is full is
// too slow to keep up; it gets disconnected; the browser's EventSource
// reconnects and starts again from a fresh snapshot.
func (h *Hub) fanOut(e Event) {
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
			delete(h.subs, ch)
			close(ch)
		}
	}
}

// Subscribe returns the current fragments plus a channel of future updates.
// Call cancel when done. The channel is closed if the subscriber falls
// behind.
func (h *Hub) Subscribe() (snapshot []Event, updates <-chan Event, cancel func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, name := range h.order {
		snapshot = append(snapshot, Event{name, h.last[name]})
	}
	ch := make(chan Event, 64)
	h.subs[ch] = struct{}{}
	return snapshot, ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
	}
}
