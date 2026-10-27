package realtime

import (
	"context"
	"fmt"
	"time"
)

// FeedOptions shapes a TickerFeed: how often a tick fires and what
// clock it reads. Now is injectable so tests get deterministic
// timestamps without waiting.
type FeedOptions struct {
	Interval time.Duration
	Now      func() time.Time
}

// TickerFeed is the SSE demo's single source of ticks: one shared
// sequence, published into a hub room (so every connected stream sees
// the same ids) and retained in a bounded ReplayLog (so a reconnecting
// client resumes by Last-Event-ID instead of restarting the count).
// TickOnce is the deterministic core — one tick, one publish, one log
// append; Run is only the wall-clock shell around it.
type TickerFeed struct {
	hub  *Hub
	room string
	log  *ReplayLog
	opts FeedOptions
}

// NewTickerFeed builds a feed publishing into the hub's room, retaining
// the newest replayCapacity messages for Last-Event-ID resume. The
// interval must be positive — a feed that cannot tick is a bug.
func NewTickerFeed(hub *Hub, room string, replayCapacity int, opts FeedOptions) (*TickerFeed, error) {
	if opts.Interval <= 0 {
		return nil, fmt.Errorf("ticker feed interval must be positive, got %s", opts.Interval)
	}
	log, err := NewReplayLog(replayCapacity)
	if err != nil {
		return nil, fmt.Errorf("ticker feed replay log: %w", err)
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &TickerFeed{hub: hub, room: room, log: log, opts: opts}, nil
}

// TickOnce advances the feed by one tick: the hub assigns the room's
// next sequence number and fans the message out to every subscribed
// stream, and the replay log retains it. Exposed so tests — and the
// story payload — can drive the feed deterministically.
func (f *TickerFeed) TickOnce() Message {
	msg, _ := f.hub.Publish(f.room, "tick", "feed", "tick", f.opts.Now())
	f.log.Append(msg)
	return msg
}

// Run ticks forever at the feed's interval until the context is
// canceled — the composition root starts it once per process in its own
// goroutine and cancels it on shutdown.
func (f *TickerFeed) Run(ctx context.Context) {
	ticker := time.NewTicker(f.opts.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.TickOnce()
		}
	}
}

// ReplayAfter returns the retained ticks a client that last processed
// seq has missed — the resume path the stream handler serves on a
// reconnection carrying Last-Event-ID.
func (f *TickerFeed) ReplayAfter(seq int64) []Message {
	return f.log.After(seq)
}

// Retained reports how many ticks the replay log currently holds (the
// story payload's resume-math beat quotes it).
func (f *TickerFeed) Retained() int {
	return f.log.Len()
}

// Hub exposes the hub the feed publishes through (the stream handler
// subscribes through the same hub).
func (f *TickerFeed) Hub() *Hub { return f.hub }

// Room names the hub room the feed publishes to.
func (f *TickerFeed) Room() string { return f.room }
