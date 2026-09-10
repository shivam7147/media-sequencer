// Package sse is a minimal publish/subscribe broker for the
// GET /api/events stream. It knows nothing about HTTP; internal/handlers
// owns the actual SSE wire format.
package sse

import "sync"

// subscriberBuffer is how many pending events a subscriber can queue
// before events start getting dropped for it. Small on purpose: a
// subscriber only ever needs to know "something changed, go refetch
// /api/state" — it does not need every event that has ever fired, just
// the fact that at least one fired since it last looked.
const subscriberBuffer = 4

// Broker fans a broadcast event out to every subscribed channel.
type Broker struct {
	mu          sync.Mutex
	subscribers map[chan string]struct{}
}

// NewBroker returns an empty, ready-to-use Broker.
func NewBroker() *Broker {
	return &Broker{subscribers: make(map[chan string]struct{})}
}

// Subscribe registers a new subscriber and returns its event channel plus
// an unsubscribe function. Callers must call unsubscribe exactly once,
// typically in a defer, when they stop reading from the channel.
func (b *Broker) Subscribe() (<-chan string, func()) {
	ch := make(chan string, subscriberBuffer)

	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		if _, ok := b.subscribers[ch]; ok {
			delete(b.subscribers, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
	return ch, unsubscribe
}

// Broadcast sends event to every current subscriber.
//
// The send is non-blocking: a subscriber whose buffer is already full
// (a slow reader, or one whose HTTP handler goroutine is stuck) simply
// misses this event rather than being waited on. That's safe because
// every event is just a signal to refetch /api/state — dropping one is
// harmless, the client is still correct the moment it refetches. The
// alternative, a blocking send, would let one slow or dead client freeze
// Broadcast itself, and every write handler in the process calls
// Broadcast synchronously after committing — so a stuck subscriber would
// stall every future write for every other client too.
func (b *Broker) Broadcast(event string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
