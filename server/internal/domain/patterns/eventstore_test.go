package patterns

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// The event store's guarantees, tested against the machinery directly:
// dense 1-based versions, optimistic concurrency with a typed conflict,
// batch atomicity, defensive copies (the structural immutability),
// version-order replay, the zero-event fold, and race-clean concurrent
// appends. The fixed occurrence times are DATA — the store never reads
// a clock, so every test is deterministic without mocking time.

// esClock is a fixed occurrence time for test events.
var esClock = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

// esEvent builds one append with a fixed clock.
func esEvent(eventType string, payload string) EventAppend {
	return EventAppend{Type: eventType, Payload: []byte(payload), RecordedAt: esClock}
}

func TestEventStoreAppendAssignsDenseOneBasedVersions(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	first, err := store.Append(ctx, "inventory-tee", EventStreamVersionEmpty, esEvent("stocked", `{"qty":50}`))
	if err != nil {
		t.Fatalf("first append: %v", err)
	}
	second, err := store.Append(ctx, "inventory-tee", 1,
		esEvent("stocked", `{"qty":10}`), esEvent("reserved", `{"qty":5}`))
	if err != nil {
		t.Fatalf("second append: %v", err)
	}

	for i, record := range append(first, second...) {
		if want := int64(i + 1); record.Version != want {
			t.Errorf("record %d version = %d, want %d (dense, 1-based, batch-continuous)", i, record.Version, want)
		}
		if record.StreamID != "inventory-tee" {
			t.Errorf("record %d stream = %q, want the appended stream", i, record.StreamID)
		}
	}
	// The payload crossed the []byte→string freeze intact.
	if second[1].Payload != `{"qty":5}` {
		t.Errorf("payload = %q, want the exact appended bytes", second[1].Payload)
	}
	if second[1].RecordedAt != esClock {
		t.Errorf("recordedAt = %v, want the caller-supplied instant preserved verbatim", second[1].RecordedAt)
	}

	version, err := store.Version(ctx, "inventory-tee")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if version != 3 {
		t.Errorf("stream version = %d, want 3 (the count of stored events)", version)
	}
}

func TestEventStoreAppendEnforcesOptimisticConcurrency(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()
	if _, err := store.Append(ctx, "cart-7", EventStreamVersionEmpty, esEvent("opened", `{}`)); err != nil {
		t.Fatalf("create: %v", err)
	}

	cases := []struct {
		name            string
		expectedVersion int64
		wantExpected    int64
		wantActual      int64
	}{
		{"a stale writer is behind the head", 0, 0, 1},
		{"an ahead-of-history writer", 5, 5, 1},
	}
	for _, tc := range cases {
		_, err := store.Append(ctx, "cart-7", tc.expectedVersion, esEvent("item-added", `{}`))
		var conflict *EventStoreConflict
		if !errors.As(err, &conflict) {
			t.Fatalf("%s: error %v must be a *EventStoreConflict", tc.name, err)
		}
		if conflict.ExpectedVersion != tc.wantExpected || conflict.ActualVersion != tc.wantActual {
			t.Errorf("%s: conflict = expected %d / actual %d, want %d / %d",
				tc.name, conflict.ExpectedVersion, conflict.ActualVersion, tc.wantExpected, tc.wantActual)
		}
	}

	// The stream is untouched by every refused append.
	records, err := store.Stream(ctx, "cart-7")
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(records) != 1 || records[0].Version != 1 {
		t.Errorf("after conflicts, stream holds %d records (first version %d), want exactly 1 at version 1",
			len(records), records[0].Version)
	}

	// The matching version is accepted — the reload-retry loop closes.
	if _, err := store.Append(ctx, "cart-7", 1, esEvent("item-added", `{}`)); err != nil {
		t.Errorf("append at the fresh head must succeed: %v", err)
	}
}

func TestEventStoreAnyVersionSkipsTheCheck(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	if _, err := store.Append(ctx, "audit-log", EventStreamVersionAny, esEvent("signed-in", `{}`)); err != nil {
		t.Fatalf("any-version append on an absent stream: %v", err)
	}
	if _, err := store.Append(ctx, "audit-log", EventStreamVersionAny, esEvent("signed-out", `{}`)); err != nil {
		t.Fatalf("any-version append without knowing the head: %v", err)
	}
	version, err := store.Version(ctx, "audit-log")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if version != 2 {
		t.Errorf("version = %d, want 2 — Any skips the check, never the versioning", version)
	}
}

func TestEventStoreAppendValidatesItsInput(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	cases := []struct {
		name  string
		drive func() error
	}{
		{"blank stream id", func() error {
			_, err := store.Append(ctx, "  ", EventStreamVersionEmpty, esEvent("typed", `{}`))
			return err
		}},
		{"empty batch", func() error {
			_, err := store.Append(ctx, "stream", EventStreamVersionEmpty)
			return err
		}},
		{"blank event type", func() error {
			_, err := store.Append(ctx, "stream", EventStreamVersionEmpty, esEvent("", `{}`))
			return err
		}},
		{"zero recorded-at", func() error {
			_, err := store.Append(ctx, "stream", EventStreamVersionEmpty,
				EventAppend{Type: "typed", Payload: []byte(`{}`)})
			return err
		}},
		{"impossible expected version", func() error {
			_, err := store.Append(ctx, "stream", -2, esEvent("typed", `{}`))
			return err
		}},
	}
	for _, tc := range cases {
		err := tc.drive()
		if err == nil {
			t.Errorf("%s: must be refused", tc.name)
			continue
		}
		var conflict *EventStoreConflict
		if errors.As(err, &conflict) {
			t.Errorf("%s: refusal %v is a conflict, want a validation error — the input never reached the check", tc.name, err)
		}
	}

	// Cancelled context: refused before anything is touched.
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Append(cancelled, "stream", EventStreamVersionEmpty, esEvent("typed", `{}`)); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled ctx: %v, want the ctx error wrapped", err)
	}

	// Nothing from any refused call landed anywhere.
	if version, err := store.Version(ctx, "stream"); err != nil || version != 0 {
		t.Errorf("refused calls left version %d (err %v), want a pristine 0", version, err)
	}
}

func TestEventStoreAppendBatchIsAtomic(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()

	batch := []EventAppend{esEvent("stocked", `{"qty":1}`), {Type: "", Payload: []byte(`{}`), RecordedAt: esClock}}
	if _, err := store.Append(ctx, "inventory-tee", EventStreamVersionEmpty, batch...); err == nil {
		t.Fatal("a batch with a malformed event must be refused whole")
	}
	if version, err := store.Version(ctx, "inventory-tee"); err != nil || version != 0 {
		t.Fatalf("the refused batch left version %d (err %v), want 0 — all or nothing", version, err)
	}

	if _, err := store.Append(ctx, "inventory-tee", EventStreamVersionEmpty,
		esEvent("stocked", `{"qty":1}`), esEvent("stocked", `{"qty":2}`)); err != nil {
		t.Fatalf("clean batch after the refused one: %v", err)
	}
	records, err := store.Stream(ctx, "inventory-tee")
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(records) != 2 || records[0].Version != 1 || records[1].Version != 2 {
		t.Errorf("records = %+v, want exactly the second batch at versions 1,2", records)
	}
}

func TestEventStoreStreamReturnsDefensiveCopies(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()
	if _, err := store.Append(ctx, "ledger-1", EventStreamVersionEmpty, esEvent("opened", `{"qty":9}`)); err != nil {
		t.Fatalf("append: %v", err)
	}

	got, err := store.Stream(ctx, "ledger-1")
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	// Forge the copies every way a caller can: edit a record, extend the
	// slice. The stored history must not move.
	got[0].Type = "FORGED"
	got[0].Payload = `{"qty":999}`
	got = append(got, EventRecord{StreamID: "ledger-1", Version: 2, Type: "FORGED"})
	if len(got) != 2 {
		t.Fatalf("forged copy length = %d, want 2 (the caller's copy did extend)", len(got))
	}

	again, err := store.Stream(ctx, "ledger-1")
	if err != nil {
		t.Fatalf("stream again: %v", err)
	}
	if len(again) != 1 || again[0].Type != "opened" || again[0].Payload != `{"qty":9}` {
		t.Errorf("stored history was reachable through the returned copies: %+v", again)
	}

	// An absent stream answers an EMPTY slice — never nil, never an error.
	absent, err := store.Stream(ctx, "never-written")
	if err != nil {
		t.Fatalf("absent stream: %v", err)
	}
	if absent == nil || len(absent) != 0 {
		t.Errorf("absent stream = %v (nil=%t), want an empty non-nil slice", absent, absent == nil)
	}
}

func TestEventStoreProjectFoldsInVersionOrderAndCarriesVersion(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()
	if _, err := store.Append(ctx, "inventory-tee", EventStreamVersionEmpty,
		esEvent("stocked", `{"qty":50}`), esEvent("reserved", `{"qty":20}`)); err != nil {
		t.Fatalf("append: %v", err)
	}

	concatenate := func(state string, event EventRecord) string { return state + event.Type + "|" }
	projection, err := EventProject(ctx, store, "inventory-tee", "", concatenate)
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if projection.State != "stocked|reserved|" {
		t.Errorf("folded state = %q, want the events applied in version order", projection.State)
	}
	if projection.Version != 2 {
		t.Errorf("projection version = %d, want 2 — the state provably reflects the head", projection.Version)
	}

	// The zero-event fold: an absent stream returns the initial state at 0.
	absent, err := EventProject(ctx, store, "ghost", "initial|", concatenate)
	if err != nil {
		t.Fatalf("absent projection: %v", err)
	}
	if absent.State != "initial|" || absent.Version != 0 {
		t.Errorf("absent projection = (%q, %d), want the untouched initial state at version 0", absent.State, absent.Version)
	}

	// A nil fold is a programmer bug, refused explicitly.
	if _, err := EventProject(ctx, store, "inventory-tee", "", nil); err == nil {
		t.Error("a nil apply fold must be refused")
	}
}

func TestEventStoreEventFoldIsPureForTheSameEvents(t *testing.T) {
	events := []EventRecord{
		{StreamID: "s", Version: 1, Type: "stocked", Payload: `{"qty":50}`},
		{StreamID: "s", Version: 2, Type: "reserved", Payload: `{"qty":20}`},
	}
	sum := func(state int, event EventRecord) int { return state + len(event.Payload) }
	first := EventFold(events, 0, sum)
	second := EventFold(events, 0, sum)
	if first != second || first != len(`{"qty":50}`)+len(`{"qty":20}`) {
		t.Errorf("fold purity drifted: %d vs %d (want %d)", first, second, len(`{"qty":50}`)+len(`{"qty":20}`))
	}
}

// TestEventStoreConcurrentAppendsSerializeAndConflictTyped runs under
// the race detector: a burst of create-races leaves exactly one winner
// with every loser holding the typed conflict, and a reload-retry loop
// then lands every writer's event exactly once — the optimistic loop
// the demo narrates, proven concurrent.
func TestEventStoreConcurrentAppendsSerializeAndConflictTyped(t *testing.T) {
	store := NewEventStore()
	ctx := context.Background()
	const writers = 8

	// Phase 1 — everyone races to create the same stream.
	var mu sync.Mutex
	wins, conflicts := 0, 0
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.Append(ctx, "hot-stream", EventStreamVersionEmpty, esEvent("claimed", fmt.Sprintf(`{"writer":%d}`, i)))
			mu.Lock()
			defer mu.Unlock()
			var conflict *EventStoreConflict
			switch {
			case err == nil:
				wins++
			case errors.As(err, &conflict) && conflict.ActualVersion == 1 && conflict.ExpectedVersion == 0:
				conflicts++
			default:
				t.Errorf("writer %d: %v, want nil or the typed conflict", i, err)
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 || conflicts != writers-1 {
		t.Fatalf("create race: %d wins / %d conflicts, want exactly 1 / %d", wins, conflicts, writers-1)
	}

	// Phase 2 — every writer appends one event via reload-retry.
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for {
				head, err := store.Version(ctx, "hot-stream")
				if err != nil {
					t.Errorf("writer %d: %v", i, err)
					return
				}
				_, err = store.Append(ctx, "hot-stream", head, esEvent("appended", fmt.Sprintf(`{"writer":%d}`, i)))
				if err == nil {
					return
				}
				var conflict *EventStoreConflict
				if !errors.As(err, &conflict) {
					t.Errorf("writer %d: %v, want only conflicts during the race", i, err)
					return
				}
			}
		}(i)
	}
	wg.Wait()

	version, err := store.Version(ctx, "hot-stream")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if want := int64(1 + writers); version != want {
		t.Errorf("final version = %d, want %d (the create win + one event per writer, exactly once each)", version, want)
	}
}
