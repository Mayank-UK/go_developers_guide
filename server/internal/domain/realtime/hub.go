package realtime

import (
	"fmt"
	"sync"
	"time"
)

// Hub is the fan-out core both live protocols share: named rooms of
// subscribers, one goroutine-free Publish that hands every Message to
// every subscriber through that subscriber's BOUNDED buffer — never a
// blocking send. The two invariants that make it safe under load:
//
//   - bounded fan-out: each subscriber owns a channel of Buffer slots,
//     so a publisher's cost is O(members) non-blocking sends, and no
//     slow reader can stall a broadcast;
//   - an explicit slow-consumer policy: a full buffer costs the
//     subscriber the message (counted), and a subscriber that misses
//     MaxDrops publishes in a row is EVICTED — removed from the room
//     and its channel closed with CloseSlowConsumer — so memory stays
//     bounded by engaged readers, not by everyone who ever joined.
//
// All state sits behind one mutex; Publish holds it only for
// non-blocking channel sends, so the critical section stays tiny.
type Hub struct {
	mu      sync.Mutex
	policy  HubPolicy
	rooms   map[string]map[*Subscriber]struct{}
	nextSeq map[string]int64
}

// HubPolicy sizes a hub. Both numbers must be positive: a zero buffer
// would drop every broadcast, a zero drop budget would evict on the
// first full buffer.
type HubPolicy struct {
	Buffer   int
	MaxDrops int
}

// DefaultHubPolicy is the buffer/drop pair the live demos run.
func DefaultHubPolicy() HubPolicy {
	return HubPolicy{Buffer: 16, MaxDrops: 8}
}

// NewHub builds an empty hub. The policy is validated up front — an
// unusable hub is a programmer error the constructor refuses to make.
func NewHub(policy HubPolicy) (*Hub, error) {
	if policy.Buffer <= 0 || policy.MaxDrops <= 0 {
		return nil, fmt.Errorf("hub policy requires Buffer > 0 and MaxDrops > 0, got %+v", policy)
	}
	return &Hub{
		policy:  policy,
		rooms:   make(map[string]map[*Subscriber]struct{}),
		nextSeq: make(map[string]int64),
	}, nil
}

// Subscriber is one connection's seat in a room. The hub wires the
// bounded channel on Join; Messages delivers until the subscriber
// Leaves or the hub evicts it, in which case the channel is CLOSED and
// Eviction reports why.
type Subscriber struct {
	// ID names the subscriber in stats and eviction reports (a guest
	// id in the chat, a request id for SSE streams).
	ID string

	ch           chan Message
	drops        int
	evicted      bool
	evictionCode CloseCode
	evictionWhy  string
}

// Messages is the subscriber's inbound stream of room broadcasts.
// Receivers must drain it or be evicted by policy — that is the deal.
func (s *Subscriber) Messages() <-chan Message { return s.ch }

// Eviction reports the close code and reason the hub evicted this
// subscriber with. Meaningful only after Messages() closes; the
// channel close is a memory barrier for these fields (they are written
// under the hub lock BEFORE the close, never after).
func (s *Subscriber) Eviction() (CloseCode, string) { return s.evictionCode, s.evictionWhy }

// Join seats a subscriber in a room, creating the room on first sight
// and wiring the bounded buffer exactly once. Joining while already
// seated rewires nothing and returns false.
func (h *Hub) Join(room string, sub *Subscriber) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if sub.ch != nil {
		return false
	}
	sub.ch = make(chan Message, h.policy.Buffer)
	members, ok := h.rooms[room]
	if !ok {
		members = make(map[*Subscriber]struct{})
		h.rooms[room] = members
	}
	members[sub] = struct{}{}
	return true
}

// Leave removes a subscriber from a room (voluntary exit — the channel
// is NOT closed; the caller's own pumps are already winding down).
// Idempotent: leaving a room you are not in, or were evicted from, is a
// no-op, so a handler's deferred Leave after an eviction is safe.
func (h *Hub) Leave(room string, sub *Subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if members, ok := h.rooms[room]; ok {
		delete(members, sub)
		if len(members) == 0 {
			delete(h.rooms, room)
			delete(h.nextSeq, room)
		}
	}
}

// RoomCount reports how many subscribers a room holds right now — the
// "N online" number the chat's system notices quote.
func (h *Hub) RoomCount(room string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rooms[room])
}

// BroadcastStats reports what one Publish did: the assigned sequence
// number, how many subscribers got the message immediately, how many
// drops full buffers caused, and which subscribers this publish evicted.
type BroadcastStats struct {
	Seq       int64
	Delivered int
	Dropped   int
	Evicted   []string
}

// Publish assigns the room's next sequence number, stamps the message,
// and fans it out to every member with a non-blocking send. A full
// buffer drops the message FOR THAT SUBSCRIBER ONLY (the drop is
// counted); a subscriber whose drop count reaches MaxDrops is evicted:
// removed from the room, marked, and its channel closed — its write
// pump observes the close and ends the connection with the eviction
// code. Returns the published message and what happened to it.
func (h *Hub) Publish(room, kind, from, body string, at time.Time) (Message, BroadcastStats) {
	h.mu.Lock()
	defer h.mu.Unlock()

	stats := BroadcastStats{Evicted: []string{}}
	stats.Seq = h.nextSeq[room] + 1
	h.nextSeq[room] = stats.Seq
	msg := Message{Seq: stats.Seq, Kind: kind, From: from, Body: body, At: at}

	members := h.rooms[room]
	for sub := range members {
		select {
		case sub.ch <- msg:
			stats.Delivered++
		default:
			// Full buffer: the slow-consumer policy. This subscriber
			// misses THIS message; enough misses in a row and it loses
			// its seat. No send ever blocks the publisher.
			sub.drops++
			stats.Dropped++
			if sub.drops >= h.policy.MaxDrops && !sub.evicted {
				sub.evicted = true
				sub.evictionCode = CloseSlowConsumer
				sub.evictionWhy = fmt.Sprintf(
					"%d broadcasts missed with a full buffer — evicted so the room's memory stays bounded",
					sub.drops)
				delete(members, sub)
				close(sub.ch)
				stats.Evicted = append(stats.Evicted, sub.ID)
			}
		}
	}
	return msg, stats
}

// Policy exposes the hub's sizing (the story payload quotes it).
func (h *Hub) Policy() HubPolicy {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.policy
}
