package messaging

import (
	"context"
	"strings"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"guide/internal/domain/demos"
)

// demosRun is the run type the stream story answers, aliased so the
// steps helper reads plainly.
type demosRun = demos.MessagingStreamRun

// The story runners' tests walk BOTH full demos against the running
// stack stores (Redis + PostgreSQL via a throwaway database) — the same
// walks the HTTP handlers serve — and pin the stories' own guarantees:
// ordering, the empty second consumer, pending/claim recovery, the
// outbox's all-or-nothing pair, the relay's idempotent second pass, and
// the consumer's refusal of a redelivered duplicate. The no-residue
// belts re-derive what the closing teardowns claim: no keys under the
// namespace, no rows for the run.

func TestMessagingStreamStoryWalksTheWholeLifecycle(t *testing.T) {
	client, namespace := newTestRedisClient(t)
	runner := NewStreamStoryRunner(client)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	run, err := runner.RunStreamStory(ctx, namespace)
	if err != nil {
		t.Fatalf("the stream story must walk against the running Redis: %v", err)
	}

	if run.Stream != namespace+":orders" || run.Group != "workers" {
		t.Errorf("stream/group = %q/%q", run.Stream, run.Group)
	}
	if len(run.AppendedIDs) != 3 || len(run.ConsumerARead) != 3 {
		t.Fatalf("appended %d, consumer-a read %d, want 3/3", len(run.AppendedIDs), len(run.ConsumerARead))
	}
	for i := range run.AppendedIDs {
		if run.ConsumerARead[i].ID != run.AppendedIDs[i] {
			t.Errorf("entry %d drifted from append order", i)
		}
	}
	if want := []string{"created", "paid", "shipped"}; strings.Join(stepsOfRun(run), ",") != strings.Join(want, ",") {
		t.Errorf("steps = %v, want %v", stepsOfRun(run), want)
	}
	if len(run.ConsumerBRead) != 0 {
		t.Errorf("consumer-b read %d entries, want 0", len(run.ConsumerBRead))
	}
	if len(run.AckedByA) != 2 {
		t.Errorf("consumer-a acked %d entries, want 2", len(run.AckedByA))
	}
	if len(run.PendingBefore) != 1 || run.PendingBefore[0].ID != run.StalledID || run.PendingBefore[0].Consumer != "consumer-a" {
		t.Errorf("pending before = %+v, want the stalled entry owned by consumer-a", run.PendingBefore)
	}
	if len(run.ClaimedByB) != 1 || run.ClaimedByB[0].ID != run.StalledID || run.ClaimedByB[0].Step != "paid" {
		t.Errorf("claimed = %+v, want exactly the stalled entry with its values", run.ClaimedByB)
	}
	if run.PendingAfterCount != 0 {
		t.Errorf("pending after = %d, want 0", run.PendingAfterCount)
	}

	// The no-residue belt: the stream key and its group are gone.
	exists, err := client.Exists(ctx, run.Stream).Result()
	if err != nil {
		t.Fatalf("check stream residue: %v", err)
	}
	if exists != 0 {
		t.Errorf("stream %s still exists after teardown", run.Stream)
	}
}

func TestMessagingOutboxStoryWalksTheWholePattern(t *testing.T) {
	pool := newThrowawayOutboxPool(t)
	client, namespace := newTestRedisClient(t)
	runner := NewOutboxStoryRunner(pool, client)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	run, err := runner.RunOutboxStory(ctx, namespace)
	if err != nil {
		t.Fatalf("the outbox story must walk against the running stores: %v", err)
	}

	if run.Stream != namespace+":outbox" {
		t.Errorf("stream = %q", run.Stream)
	}
	if len(run.Recorded) != 2 {
		t.Fatalf("committed events = %d, want 2", len(run.Recorded))
	}
	for _, event := range run.Recorded {
		if !event.Committed {
			t.Errorf("event %q recorded as not committed", event.ID)
		}
	}
	if run.RolledBackPresent {
		t.Error("the rolled-back pair survived — the transaction is not all-or-nothing")
	}
	if len(run.Relay.Published) != 2 {
		t.Errorf("relay published %d, want 2", len(run.Relay.Published))
	}
	if run.RelayAgainCount != 0 {
		t.Errorf("second relay pass published %d, want 0", run.RelayAgainCount)
	}
	if len(run.Consumed) != 2 {
		t.Fatalf("consumed %d, want 2", len(run.Consumed))
	}
	for _, delivery := range run.Consumed {
		if !delivery.FirstTime {
			t.Errorf("delivery %s was not a first-time processing", delivery.EntryID)
		}
	}
	if len(run.Redelivered) != 1 || run.Redelivered[0].FirstTime {
		t.Fatalf("redelivered = %+v, want exactly one refused duplicate", run.Redelivered)
	}
	if run.DuplicatesRefused != 1 {
		t.Errorf("duplicates refused = %d, want 1", run.DuplicatesRefused)
	}
	if run.EventsDeleted != 2 || run.OutboxDeleted != 2 {
		t.Errorf("cleanup deleted %d events / %d outbox rows, want 2/2", run.EventsDeleted, run.OutboxDeleted)
	}

	// The no-residue belts: stream gone, dedup marks gone, rows gone.
	exists, err := client.Exists(ctx, run.Stream).Result()
	if err != nil {
		t.Fatalf("check stream residue: %v", err)
	}
	if exists != 0 {
		t.Errorf("stream %s still exists after teardown", run.Stream)
	}
	var dedupResidue int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM msgdemo_outbox WHERE run_id = $1`, namespace).Scan(&dedupResidue); err != nil {
		t.Fatalf("check outbox residue: %v", err)
	}
	if dedupResidue != 0 {
		t.Errorf("%d outbox rows survived the cleanup", dedupResidue)
	}
}

func TestMessagingStoriesDegradeFastWhenRedisIsDown(t *testing.T) {
	deadClient := newDeadRedisClient(t)
	runner := NewStreamStoryRunner(deadClient)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	if _, err := runner.RunStreamStory(ctx, "gdg:demo:msg:dead"); err == nil {
		t.Fatal("the stream story against a dead Redis must fail honestly")
	}
	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Errorf("degradation took %v — the story must fail inside its bounds", elapsed)
	}
}

// stepsOfRun renders a stream run's steps for comparison.
func stepsOfRun(run demosRun) []string {
	steps := make([]string, 0, len(run.ConsumerARead))
	for _, row := range run.ConsumerARead {
		steps = append(steps, row.Step)
	}
	return steps
}

// newDeadRedisClient builds a client pointed at an address nothing
// listens on, with half-second timeouts so degradation is fast.
func newDeadRedisClient(t *testing.T) *goredis.Client {
	t.Helper()
	client := goredis.NewClient(&goredis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  500 * time.Millisecond,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	})
	t.Cleanup(func() { _ = client.Close() })
	return client
}
