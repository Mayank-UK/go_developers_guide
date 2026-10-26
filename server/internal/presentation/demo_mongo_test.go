package presentation

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The topic 19 demo endpoint, exercised through the same middleware
// chain production runs behind. The runner port is faked here — the
// mongo-driver-backed runner has its own tests against throwaway
// databases in internal/data/mongodb — so the happy payload AND the
// honest 503 degradation are pinned without a store.

// fakeMongoConsole serves a canned observation, or the failure its field
// carries.
type fakeMongoConsole struct {
	err error
}

func (f fakeMongoConsole) RunConsole(context.Context) (demos.MongoConsoleObservation, error) {
	observation := demos.MongoConsoleObservation{
		Database:   "gdg_demo",
		Collection: "events",
		Seed: demos.MongoSeedReport{
			SeedKeys:       []string{"deploy-api-0126", "deploy-api-0309"},
			CountBefore:    8,
			Upserted:       0,
			TotalAfter:     8,
			IndexesEnsured: []string{"evtKey_1", "type_1_occurredAt_-1", "occurredAt_1"},
		},
		Refusal: demos.MongoRefusalRow{
			Attempted: map[string]string{"key": "BAD"},
			Errors:    []string{"key — lowercase kebab-case (got BAD)"},
		},
		Queries: []demos.MongoQueryBeat{
			{Op: "find", Command: `db.events.find({type: "deploy"})`, Outcome: "2 of the deploy events",
				Documents: []map[string]string{{"evtKey": "deploy-api-0309"}}},
			{Op: "find+project", Command: `db.events.find({type: "deploy"}, {evtKey: 1, severity: 1, occurredAt: 1, _id: 0})`,
				Outcome:   "3 partial documents",
				Documents: []map[string]string{{"evtKey": "deploy-api-0309", "severity": "info"}}},
		},
		Explains: []demos.MongoExplainRow{
			{Label: "no index on severity", Stage: "COLLSCAN", DocsExamined: 8},
			{Label: "compound index, hinted", Stage: "IXSCAN", IndexName: "type_1_occurredAt_-1", KeysExamined: 3},
		},
		TTL: demos.MongoTTLShowcase{IndexName: "occurredAt_1", Key: "{occurredAt: 1}", ExpireAfterSeconds: 315360000},
		BSONTypes: []demos.MongoBSONTypeRow{
			{Field: "_id", BSONType: "objectId", Value: "68a1c2d30000000000000000"},
		},
	}
	return observation, f.err
}

func newMongoConsoleServer(runner fakeMongoConsole, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/db/mongo-console", NewDBMongoConsoleHandler(runner, logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestDBMongoConsoleDemoEndpointServesItsPayload(t *testing.T) {
	var buf strings.Builder
	server := newMongoConsoleServer(fakeMongoConsole{}, newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/db/mongo-console", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET mongo-console status = %d, want 200", rec.Code)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not a JSON object: %v", err)
	}
	for _, want := range []string{
		`"database"`, `"collection":"events"`,
		`"seedKeys"`, `"upserted":0`, `"totalAfter":8`, `"indexesEnsured"`,
		`"refusal"`, `MongoDB never saw this event`,
		`"queries"`, `"find+project"`,
		`"explains"`, `"COLLSCAN"`, `"IXSCAN"`, `"indexName":"type_1_occurredAt_-1"`,
		`"ttl"`, `"expireAfterSeconds":315360000`,
		`"bsonTypes"`, `"objectId"`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("body missing %q", want)
		}
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("the response must carry x-request-id")
	}
}

func TestDBMongoConsoleDemoEndpointDegradesTo503WhenMongoIsUnreachable(t *testing.T) {
	var buf strings.Builder
	server := newMongoConsoleServer(
		fakeMongoConsole{err: core.Wrap(errors.New("dial: connection refused"), core.KindServiceUnavailable, "MongoDB is not reachable right now.")},
		newTestLogger(&buf),
	)

	rec := doRequest(server, http.MethodGet, "/api/demo/db/mongo-console", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET mongo-console status = %d, want 503", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"service_unavailable"`) {
		t.Errorf("body is not the honest envelope: %s", body)
	}
	if !strings.Contains(body, "not reachable right now") {
		t.Errorf("body must say why it is unavailable: %s", body)
	}
	if !strings.Contains(buf.String(), "service_unavailable") {
		t.Errorf("the 503 must be logged:\n%s", buf.String())
	}
}

func TestDBMongoConsoleDemoEndpointRejectsNonGet(t *testing.T) {
	var buf strings.Builder
	server := newMongoConsoleServer(fakeMongoConsole{}, newTestLogger(&buf))

	rec := doRequest(server, http.MethodPost, "/api/demo/db/mongo-console", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST mongo-console status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("POST Allow header lost: %q", allow)
	}
}
