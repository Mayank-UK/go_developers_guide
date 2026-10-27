package messaging

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"guide/internal/domain/demos"
)

// The streams adapter's tests run against the RUNNING stack Redis
// (go_developers_guide_redis_dev) — the compose network's in-network
// address by default, GDG_REDIS_ADDR to override. Every test rides a
// THROWAWAY namespace minted per test; the adapter's own teardown
// removes what it created, and a defensive t.Cleanup force-clears the
// prefix even when an assertion fails mid-story. The degradation cases
// use a constructed bad address and a canceled context — no store
// required.

// testRedisAddr resolves the store under test.
func testRedisAddr() string {
	if addr := os.Getenv("GDG_REDIS_ADDR"); addr != "" {
		return addr
	}
	return "redis:6379"
}

// newTestStreamStore builds a store over a fresh client and clears the
// namespace prefix on test end, whatever happened.
func newTestStreamStore(t *testing.T) (*StreamStore, *goredis.Client, string) {
	t.Helper()
	client := goredis.NewClient(&goredis.Options{Addr: testRedisAddr()})
	namespace, err := newTestNamespace()
	if err != nil {
		client.Close()
		t.Fatalf("mint throwaway namespace: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		clearTestPattern(ctx, t, client, "gdg:demo:msg:")
		client.Close()
	})
	return NewStreamStore(client), client, namespace
}

// newTestNamespace mints the same throwaway namespace the demo handler
// mints — one crypto/random mint, shared shape.
func newTestNamespace() (string, error) {
	return demos.NewMessagingDemoNamespace()
}

// clearTestPattern is the defensive belt: SCAN+DEL everything under
// pattern, used only by test cleanup.
func clearTestPattern(ctx context.Context, t *testing.T, client *goredis.Client, pattern string) {
	t.Helper()
	var keys []string
	iterator := client.Scan(ctx, 0, pattern+"*", 100).Iterator()
	for iterator.Next(ctx) {
		keys = append(keys, iterator.Val())
	}
	if err := iterator.Err(); err != nil {
		t.Logf("defensive clear %s: %v", pattern, err)
		return
	}
	if len(keys) > 0 {
		if _, err := client.Del(ctx, keys...).Result(); err != nil {
			t.Logf("defensive clear %s: %v", pattern, err)
		}
	}
}

func TestStreamStoreWalksAppendReadAckPendingClaim(t *testing.T) {
	store, _, namespace := newTestStreamStore(t)
	stream := namespace + ":orders"
	group := "test-workers"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := store.EnsureGroup(ctx, stream, group, "0-0"); err != nil {
		t.Fatalf("ensure group: %v", err)
	}
	if err := store.EnsureGroup(ctx, stream, group, "0-0"); err != nil {
		t.Fatalf("ensure group must be idempotent (BUSYGROUP is success): %v", err)
	}

	appended := make([]string, 0, 3)
	for _, step := range []string{"created", "paid", "shipped"} {
		id, err := store.Append(ctx, stream, map[string]string{"step": step})
		if err != nil {
			t.Fatalf("append %s: %v", step, err)
		}
		appended = append(appended, id)
	}

	read, err := store.ReadNew(ctx, stream, group, "worker-a", 10)
	if err != nil {
		t.Fatalf("read new: %v", err)
	}
	if len(read) != 3 {
		t.Fatalf("read = %d entries, want 3", len(read))
	}
	for i, entry := range read {
		if entry.ID != appended[i] {
			t.Errorf("entry %d = %q, want %q (insertion order)", i, entry.ID, appended[i])
		}
	}

	// The queue property: a second consumer receives nothing new.
	second, err := store.ReadNew(ctx, stream, group, "worker-b", 10)
	if err != nil {
		t.Fatalf("read new (b): %v", err)
	}
	if len(second) != 0 {
		t.Errorf("second consumer read %d entries, want 0 (already owned)", len(second))
	}

	// Ack the outer entries, stall the middle one.
	if _, err := store.Ack(ctx, stream, group, appended[0], appended[2]); err != nil {
		t.Fatalf("ack: %v", err)
	}
	pending, err := store.PendingEntries(ctx, stream, group)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != appended[1] || pending[0].Consumer != "worker-a" {
		t.Fatalf("pending = %+v, want exactly %q owned by worker-a", pending, appended[1])
	}

	// The recovery: worker-b claims the stalled entry, values intact.
	claimed, err := store.ClaimIdle(ctx, stream, group, "worker-b", 0, 10)
	if err != nil {
		t.Fatalf("claim idle: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != appended[1] || claimed[0].Values["step"] != "paid" {
		t.Fatalf("claimed = %+v, want exactly the stalled entry with its values", claimed)
	}
	acked, err := store.Ack(ctx, stream, group, appended[1])
	if err != nil {
		t.Fatalf("ack stalled: %v", err)
	}
	if acked != 1 {
		t.Errorf("ack answered %d, want 1", acked)
	}
	pendingAfter, err := store.PendingEntries(ctx, stream, group)
	if err != nil {
		t.Fatalf("pending after: %v", err)
	}
	if len(pendingAfter) != 0 {
		t.Errorf("pending after = %d entries, want 0", len(pendingAfter))
	}

	if err := store.Destroy(ctx, stream, group); err != nil {
		t.Fatalf("destroy: %v", err)
	}
}

func TestStreamStoreReadNewOnAnIdleStreamAnswersEmptyNotError(t *testing.T) {
	store, _, namespace := newTestStreamStore(t)
	stream := namespace + ":idle"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// MKSTREAM creates the stream empty; a group at $ then reads nothing.
	if err := store.EnsureGroup(ctx, stream, "idle-group", "$"); err != nil {
		t.Fatalf("ensure group on an empty stream: %v", err)
	}
	read, err := store.ReadNew(ctx, stream, "idle-group", "worker", 10)
	if err != nil {
		t.Fatalf("reading an idle stream must not error: %v", err)
	}
	if read == nil || len(read) != 0 {
		t.Errorf("idle read = %#v, want a non-nil empty slice", read)
	}

	// An entry appended AFTER the group exists at $ is new to it.
	if _, err := store.Append(ctx, stream, map[string]string{"step": "late"}); err != nil {
		t.Fatalf("append late entry: %v", err)
	}
	read, err = store.ReadNew(ctx, stream, "idle-group", "worker", 10)
	if err != nil {
		t.Fatalf("read after late append: %v", err)
	}
	if len(read) != 1 || read[0].Values["step"] != "late" {
		t.Fatalf("late read = %+v, want the one appended entry", read)
	}

	if err := store.Destroy(ctx, stream, "idle-group"); err != nil {
		t.Fatalf("destroy: %v", err)
	}
}

func TestStreamStoreProcessOnceIsAnAtomicDedupSeam(t *testing.T) {
	store, _, _ := newTestStreamStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	first, err := store.ProcessOnce(ctx, "dedup-probe-1")
	if err != nil {
		t.Fatalf("first process-once: %v", err)
	}
	second, err := store.ProcessOnce(ctx, "dedup-probe-1")
	if err != nil {
		t.Fatalf("second process-once: %v", err)
	}
	other, err := store.ProcessOnce(ctx, "dedup-probe-2")
	if err != nil {
		t.Fatalf("other process-once: %v", err)
	}
	if !first || second || !other {
		t.Errorf("dedup answered first=%v second=%v other=%v, want true false true", first, second, other)
	}
}

func TestStreamStoreDegradesFastOnAnUnreachableAddress(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{
		Addr:         "127.0.0.1:1", // nothing listens here
		DialTimeout:  500 * time.Millisecond,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	})
	defer client.Close()
	store := NewStreamStore(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	if _, err := store.Append(ctx, "gdg:demo:msg:unreachable", map[string]string{"step": "created"}); err == nil {
		t.Fatal("append to a dead address must fail")
	} else if !strings.Contains(err.Error(), "XADD") {
		t.Errorf("the failure must name its command: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("degradation took %v — timeouts must bound it", elapsed)
	}
}

func TestStreamStoreHonorsContextCancellation(t *testing.T) {
	store, _, _ := newTestStreamStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled BEFORE the command runs

	if _, err := store.Append(ctx, "gdg:demo:msg:canceled", map[string]string{"step": "created"}); !errors.Is(err, context.Canceled) {
		t.Errorf("append on a canceled context = %v, want context.Canceled in the chain", err)
	}
	if _, err := store.ReadNew(ctx, "gdg:demo:msg:canceled", "g", "c", 10); !errors.Is(err, context.Canceled) {
		t.Errorf("read on a canceled context = %v, want context.Canceled in the chain", err)
	}
}
