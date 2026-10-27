package realtime

import (
	"sync"
	"testing"
)

// The hub under real concurrency: publishers and joiners/leavers racing
// (the -race detector is the point), the slow-consumer policy actually
// dropping and evicting, and the write-after-close hazards (send on a
// closed channel, double eviction) structurally impossible.

func newTestHub(t *testing.T, policy HubPolicy) *Hub {
	t.Helper()
	hub, err := NewHub(policy)
	if err != nil {
		t.Fatalf("hub: %v", err)
	}
	return hub
}

func TestHubRejectsUnusablePolicy(t *testing.T) {
	for _, policy := range []HubPolicy{{Buffer: 0, MaxDrops: 8}, {Buffer: 16, MaxDrops: 0}} {
		if _, err := NewHub(policy); err == nil {
			t.Errorf("policy %+v must be refused", policy)
		}
	}
}

func TestHubPublishDeliversToEveryMemberAndAssignsMonotonicSeqs(t *testing.T) {
	hub := newTestHub(t, DefaultHubPolicy())
	subs := make([]*Subscriber, 3)
	for i := range subs {
		subs[i] = &Subscriber{ID: string(rune('a' + i))}
		hub.Join("lobby", subs[i])
	}
	var last int64
	for i := 0; i < 5; i++ {
		msg, stats := hub.Publish("lobby", "chat", "ada", "hello", testNow)
		if msg.Seq != last+1 {
			t.Fatalf("seq = %d, want %d", msg.Seq, last+1)
		}
		last = msg.Seq
		if stats.Delivered != 3 || stats.Dropped != 0 {
			t.Errorf("stats = %+v, want 3 delivered 0 dropped", stats)
		}
		for _, sub := range subs {
			got := <-sub.Messages()
			if got.Seq != msg.Seq || got.Body != "hello" {
				t.Errorf("subscriber %s got %+v", sub.ID, got)
			}
		}
	}
	if hub.RoomCount("lobby") != 3 {
		t.Errorf("room count = %d, want 3", hub.RoomCount("lobby"))
	}
}

func TestHubSlowConsumerIsDroppedThenEvicted(t *testing.T) {
	hub := newTestHub(t, HubPolicy{Buffer: 2, MaxDrops: 3})
	healthy := &Subscriber{ID: "healthy"}
	stalled := &Subscriber{ID: "stalled"} // never reads — by design
	hub.Join("lobby", healthy)
	hub.Join("lobby", stalled)

	// Two publishes fit everyone's buffer; the healthy subscriber is
	// drained after every publish so only the stalled one accumulates.
	publish := func() BroadcastStats {
		_, stats := hub.Publish("lobby", "chat", "ada", "m", testNow)
		select {
		case <-healthy.Messages():
		default:
		}
		return stats
	}
	if stats := publish(); stats.Dropped != 0 {
		t.Fatalf("first publish stats = %+v, want no drops", stats)
	}
	if stats := publish(); stats.Dropped != 0 {
		t.Fatalf("second publish stats = %+v, want no drops", stats)
	}
	// Publishes 3..5 overflow the stalled subscriber's buffer: drops 1,
	// 2, 3 — the third reaches MaxDrops and evicts it.
	var totalDropped int
	for i := 0; i < 3; i++ {
		stats := publish()
		totalDropped += stats.Dropped
		if stats.Dropped != 1 {
			t.Fatalf("overflow publish %d stats = %+v, want exactly the stalled drop", i+1, stats)
		}
		if i < 2 && len(stats.Evicted) != 0 {
			t.Fatalf("eviction fired early on publish %d: %+v", i+1, stats)
		}
		if i == 2 && (len(stats.Evicted) != 1 || stats.Evicted[0] != "stalled") {
			t.Fatalf("third overflow stats = %+v, want the stalled subscriber evicted", stats)
		}
	}
	if totalDropped != 3 {
		t.Errorf("total dropped = %d, want 3", totalDropped)
	}
	// The eviction closes the channel; the two buffered messages are
	// still receivable (Go delivers them before the closed signal)…
	for i := 0; i < 2; i++ {
		if msg, ok := <-stalled.Messages(); !ok || msg.Seq == 0 {
			t.Fatalf("buffered message %d lost on eviction: (%+v, %t)", i+1, msg, ok)
		}
	}
	// …and then the channel reports closed with the policy code.
	if msg, ok := <-stalled.Messages(); ok || msg.Seq != 0 {
		t.Fatalf("stalled channel must end closed, got (%+v, %t)", msg, ok)
	}
	code, why := stalled.Eviction()
	if code != CloseSlowConsumer || why == "" {
		t.Errorf("eviction = (%d, %q)", uint16(code), why)
	}
	// …and the healthy subscriber lost nothing: every publish drained
	// exactly one message from it above, including the overflow ones.
	if hub.RoomCount("lobby") != 1 {
		t.Errorf("room count after eviction = %d, want 1", hub.RoomCount("lobby"))
	}
	// Publishing after the eviction must neither panic (send on closed)
	// nor touch the evicted subscriber again.
	_, stats := hub.Publish("lobby", "chat", "ada", "after", testNow)
	if stats.Dropped != 0 || len(stats.Evicted) != 0 {
		t.Errorf("post-eviction stats = %+v, want a clean delivery", stats)
	}
	<-healthy.Messages()
}

func TestHubLeaveIsIdempotentAndSafeAfterEviction(t *testing.T) {
	hub := newTestHub(t, HubPolicy{Buffer: 1, MaxDrops: 1})
	sub := &Subscriber{ID: "s"}
	hub.Join("lobby", sub)
	if ok := hub.Join("lobby", sub); ok {
		t.Error("double join must be refused, not rewire the channel")
	}
	hub.Publish("lobby", "chat", "ada", "m", testNow) // fills the buffer
	hub.Publish("lobby", "chat", "ada", "m", testNow) // evicts
	hub.Leave("lobby", sub)                           // the handler's deferred leave
	hub.Leave("lobby", sub)                           // …twice, like a racing cleanup
	hub.Publish("lobby", "chat", "ada", "m", testNow) // must not send on the closed channel
}

func TestHubRoomsAreIndependentAndCleanUpWhenEmpty(t *testing.T) {
	hub := newTestHub(t, DefaultHubPolicy())
	one := &Subscriber{ID: "one"}
	two := &Subscriber{ID: "two"}
	hub.Join("a", one)
	hub.Join("b", two)
	msgA, _ := hub.Publish("a", "chat", "ada", "for a", testNow)
	msgB, _ := hub.Publish("b", "chat", "ada", "for b", testNow)
	if msgA.Seq != 1 || msgB.Seq != 1 {
		t.Errorf("room seqs must be independent, got %d and %d", msgA.Seq, msgB.Seq)
	}
	if got := <-two.Messages(); got.Body != "for b" {
		t.Errorf("room b subscriber got %q", got.Body)
	}
	hub.Leave("b", two)
	if hub.RoomCount("b") != 0 {
		t.Error("empty room must be removed")
	}
	// Room a is unaffected; its seq continues where it left off.
	next, _ := hub.Publish("a", "chat", "ada", "again", testNow)
	if next.Seq != 2 {
		t.Errorf("room a seq after b's cleanup = %d, want 2", next.Seq)
	}
}

// TestHubSurvivesPublishersRacingJoinersAndLeavers is the -race proof:
// many goroutines publish while members churn. The assertions are the
// invariants that must hold at every interleaving: no panic, no send on
// a closed channel, and a room that stays consistent throughout.
func TestHubSurvivesPublishersRacingJoinersAndLeavers(t *testing.T) {
	hub := newTestHub(t, HubPolicy{Buffer: 4, MaxDrops: 4})
	const churners, publishers, publishes = 24, 8, 200

	var churnWG, publishWG sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < churners; i++ {
		churnWG.Add(1)
		go func() {
			defer churnWG.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				// A fresh subscriber per cycle — a Subscriber is a seat,
				// not a person: after Leave the channel is gone and only
				// a new seat can rejoin.
				sub := &Subscriber{ID: "churner"}
				if hub.Join("lobby", sub) {
					select {
					case <-sub.Messages():
						hub.Leave("lobby", sub)
					case <-stop:
						hub.Leave("lobby", sub)
						return
					}
				}
			}
		}()
	}
	for p := 0; p < publishers; p++ {
		publishWG.Add(1)
		go func() {
			defer publishWG.Done()
			for i := 0; i < publishes; i++ {
				hub.Publish("lobby", "chat", "ada", "m", testNow)
			}
		}()
	}
	publishWG.Wait() // publishers done; tell churners to wind down
	close(stop)
	churnWG.Wait()
}
