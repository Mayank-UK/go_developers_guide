package realtime

import (
	"context"
	"sync"
	"testing"
	"time"
)

// ReplayLog off-by-ones and the ticker feed's deterministic core — the
// resume math the SSE story payload quotes is proven here first.

func appendSeq(t *testing.T, log *ReplayLog, seqs ...int64) {
	t.Helper()
	for _, seq := range seqs {
		log.Append(Message{Seq: seq, Kind: "tick", Body: "tick"})
	}
}

func TestReplayAfterReturnsStrictlyNewerMessagesOldestFirst(t *testing.T) {
	log, err := NewReplayLog(8)
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	appendSeq(t, log, 41, 42, 43, 44, 45)
	got := log.After(42)
	if len(got) != 3 || got[0].Seq != 43 || got[1].Seq != 44 || got[2].Seq != 45 {
		t.Errorf("After(42) = %v, want 43 44 45", seqsOf(got))
	}
	// The boundary that matters: a client that processed 42 missed
	// exactly 43 onward — never 42 again (no duplicate), never a skip.
	if len(log.After(45)) != 0 {
		t.Errorf("After(45) = %v, want empty", seqsOf(log.After(45)))
	}
	if len(log.After(0)) != 5 {
		t.Errorf("After(0) = %v, want everything retained", seqsOf(log.After(0)))
	}
	if log.After(-1) == nil {
		t.Error("After must return a non-nil slice")
	}
}

func TestReplayLogOverwritesOldestWhenFullAndReportsTheGap(t *testing.T) {
	log, err := NewReplayLog(4)
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	appendSeq(t, log, 1, 2, 3, 4, 5, 6) // 1 and 2 overwritten
	if log.Len() != 4 {
		t.Fatalf("len = %d, want 4", log.Len())
	}
	got := log.After(0)
	if len(got) != 4 || got[0].Seq != 3 || got[3].Seq != 6 {
		t.Errorf("retained = %v, want 3 4 5 6 — the client sees the gap from its own seq", seqsOf(got))
	}
}

func TestReplayLogRejectsNonPositiveCapacity(t *testing.T) {
	if _, err := NewReplayLog(0); err == nil {
		t.Error("capacity 0 must be refused")
	}
}

func TestReplayLogIsSafeForConcurrentAppendAndReaders(t *testing.T) {
	log, err := NewReplayLog(64)
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(base int64) {
			defer wg.Done()
			for i := int64(0); i < 200; i++ {
				log.Append(Message{Seq: base*1000 + i, Kind: "tick"})
			}
		}(int64(w))
	}
	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_ = log.After(0)
			}
		}()
	}
	wg.Wait()
}

func seqsOf(messages []Message) []int64 {
	seqs := make([]int64, 0, len(messages))
	for _, msg := range messages {
		seqs = append(seqs, msg.Seq)
	}
	return seqs
}

// ── the ticker feed ──────────────────────────────────────────────────────

func newTestFeed(t *testing.T, interval time.Duration) *TickerFeed {
	t.Helper()
	hub := newTestHub(t, DefaultHubPolicy())
	feed, err := NewTickerFeed(hub, "ticker", 16, FeedOptions{
		Interval: interval,
		Now:      func() time.Time { return testNow },
	})
	if err != nil {
		t.Fatalf("feed: %v", err)
	}
	return feed
}

func TestTickerFeedAssignsConsecutiveSeqsAndRetainsTicks(t *testing.T) {
	feed := newTestFeed(t, time.Hour)
	for i := 1; i <= 5; i++ {
		msg := feed.TickOnce()
		if msg.Seq != int64(i) || msg.Kind != "tick" || msg.At != testNow {
			t.Fatalf("tick %d = %+v", i, msg)
		}
	}
	if feed.Retained() != 5 {
		t.Errorf("retained = %d, want 5", feed.Retained())
	}
	if got := feed.ReplayAfter(3); len(got) != 2 || got[0].Seq != 4 {
		t.Errorf("ReplayAfter(3) = %v, want seqs 4 5", seqsOf(got))
	}
}

func TestTickerFeedPublishesToSubscribedStreams(t *testing.T) {
	feed := newTestFeed(t, time.Hour)
	sub := &Subscriber{ID: "sse-1"}
	feed.Hub().Join(feed.Room(), sub)
	msg := feed.TickOnce()
	got := <-sub.Messages()
	if got.Seq != msg.Seq {
		t.Errorf("subscriber seq = %d, want %d", got.Seq, msg.Seq)
	}
}

func TestTickerFeedRunTicksUntilCanceled(t *testing.T) {
	feed := newTestFeed(t, time.Millisecond)
	sub := &Subscriber{ID: "sse-1"}
	feed.Hub().Join(feed.Room(), sub)
	ctx, cancel := context.WithCancel(context.Background())
	go feed.Run(ctx)
	// A few ticks must arrive; then cancellation must stop the feed
	// promptly (Run returns, no goroutine leak, no more ticks).
	for i := 0; i < 3; i++ {
		select {
		case <-sub.Messages():
		case <-time.After(2 * time.Second):
			t.Fatalf("tick %d never arrived", i+1)
		}
	}
	cancel()
	drain := 0
	for {
		select {
		case <-sub.Messages():
			drain++
			if drain > 64 {
				t.Fatal("the feed keeps publishing after cancellation")
			}
		case <-time.After(150 * time.Millisecond):
			return // quiet: Run stopped, nothing new arrives
		}
	}
}

func TestTickerFeedRejectsUnusableOptions(t *testing.T) {
	hub := newTestHub(t, DefaultHubPolicy())
	if _, err := NewTickerFeed(hub, "ticker", 16, FeedOptions{Interval: 0}); err == nil {
		t.Error("zero interval must be refused")
	}
	if _, err := NewTickerFeed(hub, "ticker", 0, FeedOptions{Interval: time.Second}); err == nil {
		t.Error("zero replay capacity must be refused")
	}
}
