package mongodb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"guide/internal/core"
	"guide/internal/domain/events"
)

// These tests run against the REAL MongoDB of the dev stack (Protocol
// v2: the long-lived compose container, zero ephemeral churn) — but never
// against a shared database. Every test draws a DEDICATED throwaway
// database (gdg_mongo_test_ prefix plus pid and timestamp), dropped in
// t.Cleanup, exactly the discipline the PostgreSQL tests keep. The
// degradation cases never touch the network at all: they construct bad
// URIs and assert the typed 503 mapping.

// testURI points at the compose stack's MongoDB through its in-network
// service name. GDG_MONGO_TEST_URI overrides it (e.g. port 27018 when
// running tests from the host).
const testURI = "mongodb://mongo:27017"

// newThrowawayClient connects (with the ping proof), creates a throwaway
// database name, and schedules its destruction. Tests build stores and
// runners against the named database and never see each other's state.
func newThrowawayClient(t *testing.T) (*mongo.Client, string) {
	t.Helper()

	uri := os.Getenv("GDG_MONGO_TEST_URI")
	if uri == "" {
		uri = testURI
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	client, err := Connect(ctx, uri)
	if err != nil {
		t.Fatalf("connect the stack's MongoDB (is the dev stack up?): %v", err)
	}

	name := fmt.Sprintf("gdg_mongo_test_%d_%d", os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), connectTimeout)
		defer dropCancel()
		if err := client.Database(name).Drop(dropCtx); err != nil {
			t.Errorf("drop throwaway database %s: %v", name, err)
		}
		if err := client.Disconnect(dropCtx); err != nil {
			t.Errorf("disconnect test client: %v", err)
		}
	})
	return client, name
}

// newTestEventStore wires an adapter over a fresh throwaway database.
func newTestEventStore(t *testing.T) *EventStore {
	t.Helper()
	client, database := newThrowawayClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	store, err := NewEventStore(ctx, client, database)
	if err != nil {
		t.Fatalf("wire the event store: %v", err)
	}
	return store
}

func TestMongoEventStoreUpsertsAreIdempotent(t *testing.T) {
	store := newTestEventStore(t)
	ctx := context.Background()
	draft := events.Draft{
		Key: "deploy-api-0309", Type: events.TypeDeploy, Severity: events.SeverityInfo,
		Source: "ci", Message: "api image 1.5.1 rolled out", Tags: []string{"deploy", "prod"},
		OccurredAt: time.Date(2026, 3, 9, 8, 55, 0, 0, time.UTC),
	}

	first, err := store.Upsert(ctx, draft2event(draft))
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	second, err := store.Upsert(ctx, draft2event(draft))
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("re-upsert changed the id: %q then %q — upserts must be stable", first.ID, second.ID)
	}
	if total, err := store.Count(ctx, events.Query{Limit: events.MaxQueryLimit}); err != nil || total != 1 {
		t.Errorf("count after re-upsert = %d (err %v), want 1 — replays replace, they never duplicate", total, err)
	}
}

func TestMongoEventStoreRefusesInvalidDraftsWithZeroIO(t *testing.T) {
	store := newTestEventStore(t)
	ctx := context.Background()
	record := events.NewRecordEventUseCase(store)

	_, err := record.Execute(ctx, events.Draft{
		Key: "BAD KEY", Type: "restart", Severity: "sev-1",
		OccurredAt: time.Date(2026, 3, 14, 7, 30, 0, 0, time.UTC),
	})
	if !errors.Is(err, core.ErrValidationFailed) {
		t.Fatalf("invalid draft must be the typed 422, got %v", err)
	}
	if total, err := store.Count(ctx, events.Query{Limit: events.MaxQueryLimit}); err != nil || total != 0 {
		t.Errorf("count after the refusal = %d (err %v), want 0 — the driver never saw the draft", total, err)
	}
}

func TestMongoEventStoreRecentFiltersSortsAndLimits(t *testing.T) {
	store := newTestEventStore(t)
	ctx := context.Background()
	record := events.NewRecordEventUseCase(store)
	seed := []events.Draft{
		{Key: "deploy-api-0126", Type: events.TypeDeploy, Severity: events.SeverityInfo, Source: "ci",
			Message: "one", Tags: []string{"deploy", "prod"}, OccurredAt: time.Date(2026, 1, 26, 10, 4, 0, 0, time.UTC)},
		{Key: "deploy-web-0210", Type: events.TypeDeploy, Severity: events.SeverityInfo, Source: "ci",
			Message: "two", Tags: []string{"deploy", "staging"}, OccurredAt: time.Date(2026, 2, 10, 16, 42, 0, 0, time.UTC)},
		{Key: "alert-disk-0221", Type: events.TypeAlert, Severity: events.SeverityCritical, Source: "node-exporter",
			Message: "three", Tags: []string{"alert", "prod"}, OccurredAt: time.Date(2026, 2, 21, 4, 30, 0, 0, time.UTC)},
		{Key: "deploy-api-0309", Type: events.TypeDeploy, Severity: events.SeverityInfo, Source: "ci",
			Message: "four", Tags: []string{"deploy", "prod"}, OccurredAt: time.Date(2026, 3, 9, 8, 55, 0, 0, time.UTC)},
	}
	for _, draft := range seed {
		if _, err := record.Execute(ctx, draft); err != nil {
			t.Fatalf("seed %s: %v", draft.Key, err)
		}
	}

	deploys, err := store.Recent(ctx, events.Query{Type: events.TypeDeploy, Limit: 2})
	if err != nil {
		t.Fatalf("recent deploys: %v", err)
	}
	if len(deploys) != 2 {
		t.Fatalf("recent deploys = %d, want the limit of 2", len(deploys))
	}
	if deploys[0].Key != "deploy-api-0309" || deploys[1].Key != "deploy-web-0210" {
		t.Errorf("newest-first order = %s, %s — want 0309 then 0210", deploys[0].Key, deploys[1].Key)
	}

	prod, err := store.Recent(ctx, events.Query{Tag: "prod", Limit: 10})
	if err != nil {
		t.Fatalf("recent prod: %v", err)
	}
	if len(prod) != 3 {
		t.Errorf("tag containment = %d events, want 3", len(prod))
	}

	if total, err := store.Count(ctx, events.Query{Type: events.TypeDeploy}); err != nil || total != 3 {
		t.Errorf("count deploys = %d (err %v), want 3", total, err)
	}
}

func TestMongoEventStoreEnsuresIndexesAndTheTTLSpec(t *testing.T) {
	store := newTestEventStore(t)
	ctx := context.Background()

	cursor, err := store.collection.Indexes().List(ctx)
	if err != nil {
		t.Fatalf("list indexes: %v", err)
	}
	type indexSpec struct {
		unique     bool
		ttlSeconds *int64
	}
	specs := map[string]indexSpec{}
	for cursor.Next(ctx) {
		var spec struct {
			Name               string `bson:"name"`
			Unique             bool   `bson:"unique"`
			ExpireAfterSeconds *int64 `bson:"expireAfterSeconds"`
		}
		if err := cursor.Decode(&spec); err != nil {
			t.Fatalf("decode index spec: %v", err)
		}
		specs[spec.Name] = indexSpec{unique: spec.Unique, ttlSeconds: spec.ExpireAfterSeconds}
	}
	if err := cursor.Err(); err != nil {
		t.Fatalf("index cursor: %v", err)
	}

	for _, name := range []string{keyIndexName, queryIndexName, ttlIndexName} {
		if _, ok := specs[name]; !ok {
			t.Errorf("index %s missing — the store must ensure all three at construction", name)
		}
	}
	if !specs[keyIndexName].unique {
		t.Errorf("index %s must be unique — it is the upsert address", keyIndexName)
	}
	ttl := specs[ttlIndexName].ttlSeconds
	if ttl == nil || *ttl != int64(TTLSeconds) {
		t.Errorf("TTL spec = %v, want expireAfterSeconds %d (assert the SPEC — nothing expires mid-suite)", ttl, TTLSeconds)
	}

	// The unique index must actually bite: a second document at the same
	// business key is refused by the server, not by the app.
	_, err = store.collection.InsertOne(ctx, bson.D{
		{Key: "evtKey", Value: "duplicate-key"}, {Key: "type", Value: events.TypeDeploy},
	})
	if err != nil {
		t.Fatalf("first insert at the key: %v", err)
	}
	_, err = store.collection.InsertOne(ctx, bson.D{
		{Key: "evtKey", Value: "duplicate-key"}, {Key: "type", Value: events.TypeDeploy},
	})
	if !mongo.IsDuplicateKeyError(err) {
		t.Errorf("second insert at the key = %v, want the duplicate-key refusal", err)
	}
}

func TestMongoConnectDegradesBadURIsToTheTyped503(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	// An unreachable host: the URI parses, the ping cannot land.
	if _, err := Connect(ctx, "mongodb://127.0.0.1:1"); err == nil {
		t.Fatal("an unreachable MongoDB must refuse Connect")
	} else if !errors.Is(err, core.ErrServiceUnavailable) {
		t.Errorf("unreachable host = %v, want the typed service_unavailable", err)
	}

	// A syntactically broken URI: the options parser refuses it.
	if _, err := Connect(ctx, "this is not a uri"); err == nil {
		t.Fatal("a malformed URI must refuse Connect")
	} else if !errors.Is(err, core.ErrServiceUnavailable) {
		t.Errorf("malformed URI = %v, want the typed service_unavailable", err)
	}
}

func TestMongoUnavailableConsoleRunnerServesTheTyped503(t *testing.T) {
	cause := errors.New("dial tcp 127.0.0.1:1: connect: connection refused")
	runner := NewUnavailableConsoleRunner(cause)

	_, err := runner.RunConsole(context.Background())
	if !errors.Is(err, core.ErrServiceUnavailable) {
		t.Fatalf("unavailable runner = %v, want the typed service_unavailable carrying the cause", err)
	}
	if !errors.Is(err, cause) && !errors.Is(errors.Unwrap(errors.Unwrap(err)), cause) {
		t.Errorf("the boot-time cause must ride the chain: %v", err)
	}
}

// draft2event is the one-line translation tests use to hand the adapter
// a stored-form event (the adapter's own Upsert path uses the domain's
// use case everywhere else).
func draft2event(draft events.Draft) events.Event {
	return events.Event{
		Key: draft.Key, Type: draft.Type, Severity: draft.Severity, Source: draft.Source,
		Message: draft.Message, Tags: draft.Tags, OccurredAt: draft.OccurredAt,
	}
}
