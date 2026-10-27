package messaging

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"guide/internal/domain/messaging"
)

// The outbox adapter's tests run against the REAL PostgreSQL of the dev
// stack (Protocol v2: the long-lived compose containers) — but never
// against its dev database. Every test draws a DEDICATED throwaway
// database through an admin connection (the internal/data/db pattern),
// applies the outbox's own idempotent schema, and drops the database in
// t.Cleanup. The Redis side rides the same running store as the stream
// tests, under throwaway namespaces.

// outboxAdminDSN points at the compose stack's PostgreSQL through its
// in-network service name, at the `postgres` maintenance database.
// GDG_PG_ADMIN_DSN overrides it.
const outboxAdminDSN = "postgres://postgres:guide@postgres:5432/postgres?sslmode=disable"

// newThrowawayOutboxPool creates a fresh database on the live server,
// connects a pool to it, and schedules its destruction.
func newThrowawayOutboxPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	adminDSN := os.Getenv("GDG_PG_ADMIN_DSN")
	if adminDSN == "" {
		adminDSN = outboxAdminDSN
	}
	parsed, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatalf("parse admin DSN: %v", err)
	}
	if strings.TrimPrefix(parsed.Path, "/") == "guide" {
		t.Fatalf("admin DSN must not target the dev database (got %q)", parsed.Path)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	admin, err := pgxpool.New(ctx, adminDSN)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	defer admin.Close()

	dbName := fmt.Sprintf("gdg_test_msg_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := admin.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %s`, dbName)); err != nil {
		t.Fatalf("create throwaway database %s: %v", dbName, err)
	}
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer dropCancel()
		if _, err := admin.Exec(dropCtx, fmt.Sprintf(`DROP DATABASE %s WITH (FORCE)`, dbName)); err != nil {
			t.Logf("drop throwaway database %s: %v", dbName, err)
		}
	})

	dsn := fmt.Sprintf("postgres://postgres:guide@%s:%s/%s?sslmode=disable",
		parsed.Hostname(), parsed.Port(), dbName)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool to %s: %v", dbName, err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// newTestOutbox wires the outbox over a throwaway database and the
// running Redis, with defensive cleanup of the messaging key prefix.
func newTestOutbox(t *testing.T) (*Outbox, *StreamStore, string) {
	t.Helper()
	pool := newThrowawayOutboxPool(t)
	client, namespace := newTestRedisClient(t)
	store := NewStreamStore(client)
	return NewOutbox(pool, store), store, namespace
}

// newTestRedisClient opens a fresh client over the test address with
// the messaging prefix defensively cleared on test end.
func newTestRedisClient(t *testing.T) (*goredis.Client, string) {
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
	return client, namespace
}

// testEvent builds a business row for the outbox tests.
func testEvent(runID, id, kind, payload string) messaging.EventRecord {
	return messaging.EventRecord{ID: id, RunID: runID, Kind: kind, Payload: payload}
}

// testOutboxRow builds an outbox row for the outbox tests.
func testOutboxRow(runID, id, stream, payload string) messaging.OutboxMessage {
	return messaging.OutboxMessage{ID: id, RunID: runID, Stream: stream, Payload: payload}
}

func TestOutboxSchemaIsIdempotent(t *testing.T) {
	outbox, _, _ := newTestOutbox(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := outbox.EnsureSchema(ctx); err != nil {
		t.Fatalf("first ensure schema: %v", err)
	}
	if err := outbox.EnsureSchema(ctx); err != nil {
		t.Fatalf("second ensure schema must be a no-op: %v", err)
	}
}

func TestOutboxCommitsBothRowsOrNeither(t *testing.T) {
	outbox, _, namespace := newTestOutbox(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := outbox.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	committedID := "m-committed000001"
	if err := outbox.RecordEventWithOutbox(ctx,
		testEvent(namespace, committedID, "order.created", "order o-1 created"),
		testOutboxRow(namespace, committedID, namespace+":outbox", "order o-1 created")); err != nil {
		t.Fatalf("record committed pair: %v", err)
	}
	rolledBackID := "m-rolledback00001"
	if err := outbox.RecordEventWithOutboxThenRollback(ctx,
		testEvent(namespace, rolledBackID, "order.cancelled", "order o-1 cancelled"),
		testOutboxRow(namespace, rolledBackID, namespace+":outbox", "order o-1 cancelled")); err != nil {
		t.Fatalf("record rolled-back pair: %v", err)
	}

	committed, err := outbox.CountEvent(ctx, namespace, committedID)
	if err != nil {
		t.Fatalf("count committed event: %v", err)
	}
	rolledBack, err := outbox.CountEvent(ctx, namespace, rolledBackID)
	if err != nil {
		t.Fatalf("count rolled-back event: %v", err)
	}
	if committed != 1 {
		t.Errorf("committed event count = %d, want 1", committed)
	}
	if rolledBack != 0 {
		t.Errorf("rolled-back event count = %d, want 0 — the unit is all-or-nothing", rolledBack)
	}
}

func TestOutboxRelayPublishesOnceAndDrainsEmptyOnTheSecondPass(t *testing.T) {
	outbox, store, namespace := newTestOutbox(t)
	stream := namespace + ":outbox"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := outbox.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	ids := []string{"m-relay000000001", "m-relay000000002"}
	for i, id := range ids {
		payload := fmt.Sprintf("order o-%d created", i+1)
		if err := outbox.RecordEventWithOutbox(ctx,
			testEvent(namespace, id, "order.created", payload),
			testOutboxRow(namespace, id, stream, payload)); err != nil {
			t.Fatalf("record %s: %v", id, err)
		}
	}

	first, err := outbox.RelayPending(ctx, namespace, 10)
	if err != nil {
		t.Fatalf("first relay: %v", err)
	}
	if len(first.Published) != 2 {
		t.Fatalf("first pass published %d, want 2", len(first.Published))
	}
	for i, row := range first.Published {
		if row.OutboxID != ids[i] {
			t.Errorf("published[%d] = %s, want %s in oldest-first order", i, row.OutboxID, ids[i])
		}
		if row.StreamEntry == "" {
			t.Errorf("published[%d] carries no stream entry id", i)
		}
	}

	second, err := outbox.RelayPending(ctx, namespace, 10)
	if err != nil {
		t.Fatalf("second relay: %v", err)
	}
	if len(second.Published) != 0 {
		t.Errorf("second pass published %d, want 0 — published rows never ride twice", len(second.Published))
	}

	// The stream carries both messages, in relay order, with the message
	// id riding as a field for consumer dedup.
	if err := store.EnsureGroup(ctx, stream, "relay-group", "0-0"); err != nil {
		t.Fatalf("ensure group: %v", err)
	}
	entries, err := store.ReadNew(ctx, stream, "relay-group", "relay-worker", 10)
	if err != nil {
		t.Fatalf("read relayed entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("stream holds %d entries, want 2", len(entries))
	}
	for i, entry := range entries {
		if entry.Values["messageId"] != ids[i] {
			t.Errorf("entry %d messageId = %q, want %q", i, entry.Values["messageId"], ids[i])
		}
	}

	events, outboxRows, err := outbox.Cleanup(ctx, namespace)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if events != 2 || outboxRows != 2 {
		t.Errorf("cleanup deleted %d events / %d outbox rows, want 2/2", events, outboxRows)
	}
}

func TestOutboxRelayRejectsABadLimit(t *testing.T) {
	outbox, _, namespace := newTestOutbox(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := outbox.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if _, err := outbox.RelayPending(ctx, namespace, 0); err == nil {
		t.Error("relay with limit 0 must fail fast")
	}
}

// TestOutboxRelayDrainsOnlyItsOwnRun pins the demo's isolation seam: a
// relay scoped to one run never publishes another run's rows — two
// concurrent story requests on the shared database stay independent.
func TestOutboxRelayDrainsOnlyItsOwnRun(t *testing.T) {
	outbox, _, namespace := newTestOutbox(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := outbox.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	// One row for THIS run and one for a foreign run, both pending.
	if err := outbox.RecordEventWithOutbox(ctx,
		testEvent(namespace, "m-own0000000000001", "order.created", "own row"),
		testOutboxRow(namespace, "m-own0000000000001", namespace+":outbox", "own row")); err != nil {
		t.Fatalf("record own row: %v", err)
	}
	foreign := namespace + "-foreign"
	if err := outbox.RecordEventWithOutbox(ctx,
		testEvent(foreign, "m-foreign000000001", "order.created", "foreign row"),
		testOutboxRow(foreign, "m-foreign000000001", foreign+":outbox", "foreign row")); err != nil {
		t.Fatalf("record foreign row: %v", err)
	}

	report, err := outbox.RelayPending(ctx, namespace, 10)
	if err != nil {
		t.Fatalf("relay own run: %v", err)
	}
	if len(report.Published) != 1 || report.Published[0].OutboxID != "m-own0000000000001" {
		t.Fatalf("relay published %+v, want exactly this run's row", report.Published)
	}

	// The foreign run's row is still pending — its own relay drains it.
	foreignReport, err := outbox.RelayPending(ctx, foreign, 10)
	if err != nil {
		t.Fatalf("relay foreign run: %v", err)
	}
	if len(foreignReport.Published) != 1 || foreignReport.Published[0].OutboxID != "m-foreign000000001" {
		t.Fatalf("foreign relay published %+v, want the foreign row untouched until its own pass", foreignReport.Published)
	}

	if _, _, err := outbox.Cleanup(ctx, namespace); err != nil {
		t.Fatalf("cleanup own run: %v", err)
	}
	if _, _, err := outbox.Cleanup(ctx, foreign); err != nil {
		t.Fatalf("cleanup foreign run: %v", err)
	}
}
