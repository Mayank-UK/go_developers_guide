package demos

import (
	"reflect"
	"testing"
)

// The mongo-console payload builder, pinned on a hand-built observation:
// beats map in order, keys come out sorted, nil collections materialize
// as empty slices/maps, the two fixed teaching notes are stamped once,
// and the same observation always builds the identical payload.

func TestDBMongoConsoleBuilderMapsBeatsInOrder(t *testing.T) {
	observation := mongoConsoleFixture()

	payload := BuildMongoConsolePayload(observation)

	if payload.Database != "gdg_demo" || payload.Collection != "events" {
		t.Errorf("database/collection = %q/%q, want gdg_demo/events", payload.Database, payload.Collection)
	}
	if len(payload.Queries) != 2 || payload.Queries[0].Op != "find" || payload.Queries[1].Op != "count" {
		t.Errorf("query beats = %+v, want the fixture's find then count in order", payload.Queries)
	}
	if len(payload.Queries[0].Documents) != 1 || payload.Queries[0].Documents[0]["evtKey"] != "deploy-api-0309" {
		t.Errorf("find documents = %v, want the fixture's row", payload.Queries[0].Documents)
	}
	if len(payload.Explains) != 2 || payload.Explains[1].Stage != "IXSCAN" {
		t.Errorf("explains = %+v, want COLLSCAN then IXSCAN", payload.Explains)
	}
	if payload.TTL.ExpireAfterSeconds != 315360000 {
		t.Errorf("TTL window = %d, want the fixture's", payload.TTL.ExpireAfterSeconds)
	}
}

func TestDBMongoConsoleBuilderSortsKeysAndStampsNotes(t *testing.T) {
	observation := mongoConsoleFixture()
	observation.Seed.SeedKeys = []string{"z-last", "a-first"}
	observation.Seed.IndexesEnsured = []string{"c", "b", "a"}

	payload := BuildMongoConsolePayload(observation)

	if want := []string{"a-first", "z-last"}; !reflect.DeepEqual(payload.Seed.SeedKeys, want) {
		t.Errorf("seed keys = %v, want sorted %v", payload.Seed.SeedKeys, want)
	}
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(payload.Seed.IndexesEnsured, want) {
		t.Errorf("indexes ensured = %v, want sorted %v", payload.Seed.IndexesEnsured, want)
	}
	if payload.Refusal.Note == "" {
		t.Error("the refusal note must be stamped by the builder")
	}
	if payload.TTL.SweeperNote == "" {
		t.Error("the TTL sweeper note must be stamped by the builder")
	}
}

func TestDBMongoConsoleBuilderNeverServesNilCollections(t *testing.T) {
	payload := BuildMongoConsolePayload(MongoConsoleObservation{
		Refusal: MongoRefusalRow{Attempted: nil, Errors: nil},
		Queries: nil, Explains: nil, BSONTypes: nil,
	})

	if payload.Seed.SeedKeys == nil || payload.Seed.IndexesEnsured == nil {
		t.Error("seed string collections must render as [], never null")
	}
	if payload.Refusal.Attempted == nil || payload.Refusal.Errors == nil {
		t.Error("refusal collections must render as {}/[], never null")
	}
	if payload.Queries == nil || payload.Explains == nil || payload.BSONTypes == nil {
		t.Error("beat collections must render as [], never null")
	}
}

func TestDBMongoConsoleBuilderIsDeterministic(t *testing.T) {
	first := BuildMongoConsolePayload(mongoConsoleFixture())
	second := BuildMongoConsolePayload(mongoConsoleFixture())

	if !reflect.DeepEqual(first, second) {
		t.Errorf("the same observation built different payloads:\n%+v\n%+v", first, second)
	}
}

// mongoConsoleFixture is a small hand-built observation shaped like the
// runner's real haul — two queries, the explain pair, a TTL spec, and
// the BSON survey's first row.
func mongoConsoleFixture() MongoConsoleObservation {
	return MongoConsoleObservation{
		Database:   "gdg_demo",
		Collection: "events",
		Seed: MongoSeedReport{
			SeedKeys:       []string{"deploy-api-0126", "deploy-api-0309"},
			CountBefore:    0,
			Upserted:       8,
			TotalAfter:     8,
			IndexesEnsured: []string{keyIdx, queryIdx, ttlIdx},
		},
		Refusal: MongoRefusalRow{
			Attempted: map[string]string{"key": "BAD", "type": "restart"},
			Errors:    []string{"key — lowercase kebab-case (got BAD)", "type — one of … (got restart)"},
		},
		Queries: []MongoQueryBeat{
			{Op: "find", Description: "newest deploys",
				Command: `db.events.find({type: "deploy"}).sort({occurredAt: -1}).limit(2)`,
				Outcome: "2 of the deploy events, newest first",
				Documents: []map[string]string{
					{"evtKey": "deploy-api-0309", "severity": "info", "occurredAt": "2026-03-09T08:55:00Z"},
				}},
			{Op: "count", Description: "deploy count",
				Command: `db.events.countDocuments({type: "deploy"})`,
				Outcome: "3 deploy events", Documents: []map[string]string{}},
		},
		Explains: []MongoExplainRow{
			{Label: "no index on severity", Query: `find({severity: "critical"})`,
				Stage: "COLLSCAN", KeysExamined: 0, DocsExamined: 8},
			{Label: "compound index, hinted", Query: `find({type: "deploy"})`,
				Stage: "IXSCAN", IndexName: queryIdx, KeysExamined: 3, DocsExamined: 3},
		},
		TTL: MongoTTLShowcase{IndexName: ttlIdx, Key: "{occurredAt: 1}", ExpireAfterSeconds: 315360000},
		BSONTypes: []MongoBSONTypeRow{
			{Field: "_id", BSONType: "objectId", Value: "68a1c2d30000000000000000",
				Note: "assigned by the server at the first insert"},
		},
	}
}

// Fixture spellings of the adapter's index names — the builder is pure,
// so the names arrive as data.
const (
	keyIdx   = "evtKey_1"
	queryIdx = "type_1_occurredAt_-1"
	ttlIdx   = "occurredAt_1"
)
