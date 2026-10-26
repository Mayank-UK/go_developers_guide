package mongodb

import (
	"context"
	"strings"
	"testing"

	"guide/internal/domain/demos"
)

// The console runner's whole scripted run, executed against the real
// MongoDB on a throwaway database. The determinism contract is the part
// to pin: beat order fixed, counts derivable from the seed, explain
// stages stable (the COLLSCAN by absence of an index, the IXSCAN by the
// hint), and the SECOND run proving idempotency — zero new upserts, the
// same total, the same _id for the surveyed document.

func TestMongoConsoleRunnerPlaysEveryBeat(t *testing.T) {
	store := newTestEventStore(t)
	runner := NewConsoleRunner(store)
	ctx := context.Background()

	observation, err := runner.RunConsole(ctx)
	if err != nil {
		t.Fatalf("run the console: %v", err)
	}

	if observation.Collection != EventsCollection {
		t.Errorf("collection = %q, want %q", observation.Collection, EventsCollection)
	}
	if observation.Seed.TotalAfter != int64(len(seedEvents)) {
		t.Errorf("seed total = %d, want %d", observation.Seed.TotalAfter, len(seedEvents))
	}
	if observation.Seed.Upserted != int64(len(seedEvents)) {
		t.Errorf("first run upserted = %d, want %d — an empty collection plants everything",
			observation.Seed.Upserted, len(seedEvents))
	}
	if len(observation.Seed.IndexesEnsured) != 3 {
		t.Errorf("indexes ensured = %v, want all three", observation.Seed.IndexesEnsured)
	}

	if len(observation.Refusal.Errors) == 0 {
		t.Error("the refusal beat must carry the validation issues")
	}

	if len(observation.Queries) != 4 {
		t.Fatalf("query beats = %d, want 4 (filter/sort/limit, containment, count, projection)", len(observation.Queries))
	}
	if got := observation.Queries[0].Documents; len(got) != 2 {
		t.Errorf("newest-deploys beat returned %d documents, want the limit of 2", len(got))
	}
	if got := observation.Queries[1].Documents; len(got) != 5 {
		t.Errorf("prod-tag beat returned %d documents, want the 5 seeded prod events", len(got))
	}
	if want := `db.events.countDocuments({type: "deploy"})`; observation.Queries[2].Command != want {
		t.Errorf("count command = %q, want %q", observation.Queries[2].Command, want)
	}
	projected := observation.Queries[3].Documents
	if len(projected) != 3 {
		t.Fatalf("projection returned %d documents, want 3 deploys", len(projected))
	}
	for _, document := range projected {
		if len(document) != 3 {
			t.Errorf("projected document has %d fields (%v), want exactly evtKey/severity/occurredAt", len(document), document)
		}
	}

	if len(observation.Explains) != 2 {
		t.Fatalf("explain rows = %d, want the COLLSCAN/IXSCAN pair", len(observation.Explains))
	}
	if observation.Explains[0].Stage != "COLLSCAN" || observation.Explains[0].DocsExamined != int64(len(seedEvents)) {
		t.Errorf("unindexed explain = %s docs=%d, want COLLSCAN docs=%d",
			observation.Explains[0].Stage, observation.Explains[0].DocsExamined, len(seedEvents))
	}
	indexed := observation.Explains[1]
	if indexed.Stage != "IXSCAN" || indexed.IndexName != queryIndexName {
		t.Errorf("indexed explain = %s on %q, want IXSCAN on %s", indexed.Stage, indexed.IndexName, queryIndexName)
	}
	if indexed.KeysExamined != 3 || indexed.DocsExamined != 3 {
		t.Errorf("indexed exam counts = keys %d docs %d, want the 3 deploys on both", indexed.KeysExamined, indexed.DocsExamined)
	}

	if observation.TTL.IndexName != ttlIndexName {
		t.Errorf("TTL index = %q, want %q", observation.TTL.IndexName, ttlIndexName)
	}
	if observation.TTL.Key != "{occurredAt: 1}" {
		t.Errorf("TTL key pattern = %q, want {{occurredAt: 1}}", observation.TTL.Key)
	}
	if observation.TTL.ExpireAfterSeconds != int64(TTLSeconds) {
		t.Errorf("TTL window = %d, want the long %d spec", observation.TTL.ExpireAfterSeconds, TTLSeconds)
	}

	if len(observation.BSONTypes) != 5 {
		t.Fatalf("BSON survey rows = %d, want the five surveyed fields", len(observation.BSONTypes))
	}
	surveyed := observation.BSONTypes[0]
	if surveyed.Field != "_id" || surveyed.BSONType != "objectId" {
		t.Fatalf("first survey row = %s/%s, want _id/objectId", surveyed.Field, surveyed.BSONType)
	}
	if len(surveyed.Value) != 24 {
		t.Errorf("the surveyed _id %q is not a 24-hex-digit ObjectId", surveyed.Value)
	}
	if !strings.Contains(surveyed.Note, "re-upserts keep it stable") {
		t.Errorf("the _id note must teach the stability: %q", surveyed.Note)
	}
}

func TestMongoConsoleRunnerIsIdempotentAcrossRuns(t *testing.T) {
	store := newTestEventStore(t)
	runner := NewConsoleRunner(store)
	ctx := context.Background()

	first, err := runner.RunConsole(ctx)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := runner.RunConsole(ctx)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	if second.Seed.CountBefore != first.Seed.TotalAfter {
		t.Errorf("second run before = %d, want the first run's %d", second.Seed.CountBefore, first.Seed.TotalAfter)
	}
	if second.Seed.Upserted != 0 {
		t.Errorf("second run upserted = %d, want 0 — the seed replays as replacements", second.Seed.Upserted)
	}
	if second.Seed.TotalAfter != first.Seed.TotalAfter {
		t.Errorf("totals drifted: %d then %d", first.Seed.TotalAfter, second.Seed.TotalAfter)
	}

	firstID := consoleSurveyedID(first)
	secondID := consoleSurveyedID(second)
	if firstID == "" || firstID != secondID {
		t.Errorf("surveyed _id = %q then %q — minted-from-the-instant ids must be stable", firstID, secondID)
	}
}

// consoleSurveyedID pulls the surveyed document's _id value out of the
// observation (row 0 of the BSON survey).
func consoleSurveyedID(observation demos.MongoConsoleObservation) string {
	if len(observation.BSONTypes) == 0 {
		return ""
	}
	return observation.BSONTypes[0].Value
}
