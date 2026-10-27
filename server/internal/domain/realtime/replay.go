package realtime

import (
	"fmt"
	"sync"
)

// ReplayLog is a bounded retention buffer of the newest Messages a
// stream published — the server half of Last-Event-ID resume. When a
// browser reconnects with the id of the last event it processed, the
// stream handler replays everything AFTER that id from this log before
// continuing live, so a brief disconnect costs the client missed frames
// rather than a gap it can never detect. The log is a ring: capacity is
// fixed at construction, the oldest entries are overwritten when it
// fills, and a client that asks for more history than was retained gets
// what exists — with sequence numbers, it can TELL how much it missed.
type ReplayLog struct {
	mu       sync.Mutex
	entries  []Message
	capacity int
	start    int
	size     int
}

// NewReplayLog builds an empty log retaining up to capacity messages.
// capacity must be positive — a log that retains nothing is a bug, not
// a policy.
func NewReplayLog(capacity int) (*ReplayLog, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("replay log capacity must be positive, got %d", capacity)
	}
	return &ReplayLog{
		entries:  make([]Message, capacity),
		capacity: capacity,
	}, nil
}

// Append records a message, overwriting the oldest entry when the log
// is full.
func (l *ReplayLog) Append(msg Message) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries[(l.start+l.size)%l.capacity] = msg
	if l.size < l.capacity {
		l.size++
	} else {
		l.start = (l.start + 1) % l.capacity
	}
}

// After returns every retained message with Seq strictly greater than
// seq, oldest first — exactly the frames a client that last processed
// seq has missed. The result is never nil so handlers can range it
// unconditionally; a client that missed more than capacity frames gets
// the retained tail, and the first returned Seq being greater than
// seq+1 is the honest signal of the gap.
func (l *ReplayLog) After(seq int64) []Message {
	l.mu.Lock()
	defer l.mu.Unlock()
	match := make([]Message, 0, l.size)
	for i := 0; i < l.size; i++ {
		msg := l.entries[(l.start+i)%l.capacity]
		if msg.Seq > seq {
			match = append(match, msg)
		}
	}
	return match
}

// Len reports how many messages the log currently retains.
func (l *ReplayLog) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.size
}
