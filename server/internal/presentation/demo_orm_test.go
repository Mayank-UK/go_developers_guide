package presentation

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The sqlc-gorm topic's orm-probe endpoint, exercised through the same
// middleware chain production runs behind (the family rule that every new
// demo endpoint ships with a Go test). The runner port is faked here —
// the real probe has its own testcontainers tests in internal/data/
// ormprobe — so both the measured happy path and the honest 503
// degradation are pinned without a database. This httptest mux IS the
// route proof for the unwired endpoint: GET /api/demo/db/orm-probe
// serves the payload shape the client slice expects.

// fakeORMProbeRunner stands in for the data-layer runner: a canned
// observation or the failure its field carries.
type fakeORMProbeRunner struct {
	observation demos.ORMProbeObservation
	err         error
}

func (f fakeORMProbeRunner) Probe(context.Context) (demos.ORMProbeObservation, error) {
	return f.observation, f.err
}

// ormProbeFixtureObservation mirrors the real runner's facts closely
// enough to pin the handler's mapping: the four trials in canonical
// order with their measured counts, and a rolled-back unit of work.
func ormProbeFixtureObservation() demos.ORMProbeObservation {
	return demos.ORMProbeObservation{
		Dataset: demos.ORMProbeDatasetFacts{Posts: 12, Tags: 5, Edges: 24},
		Trials: []demos.ORMProbeTrialFacts{
			{Strategy: demos.ORMProbeStrategyPgx, Statements: 1, Rows: 12, DurationMicros: 812,
				SampleStatements: []string{"SELECT p.id, p.title … ORDER BY p.id"}},
			{Strategy: demos.ORMProbeStrategySQLC, Statements: 1, Rows: 12, DurationMicros: 903,
				SampleStatements: []string{"-- name: ListPostsWithTags :many SELECT …"}},
			{Strategy: demos.ORMProbeStrategyGormLazy, Statements: 13, Rows: 12, DurationMicros: 3241,
				SampleStatements: []string{
					`SELECT * FROM "ormprobe_posts" ORDER BY id`,
					`SELECT * FROM "ormprobe_tags" … "post_id" = 1`,
				}},
			{Strategy: demos.ORMProbeStrategyGormEager, Statements: 3, Rows: 12, DurationMicros: 1187,
				SampleStatements: []string{`SELECT * FROM "ormprobe_post_tags" … IN (1,2,3)`}},
		},
		ResultsAgree: true,
		UnitOfWork: demos.ORMProbeUnitOfWorkFacts{
			Repositories: []string{"GormPostsRepository", "GormTagsRepository"},
			Steps: []demos.ORMProbeStepFacts{
				{Action: "begin", Detail: "one unit of work"},
				{Action: "rollback", Detail: "deliberate failure"},
			},
			HookApplied: true,
			PostsBefore: 12,
			PostsAfter:  12,
			RolledBack:  true,
		},
	}
}

func newORMDemoServer(runner demos.ORMProbeRunner, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/db/orm-probe", NewDBORMProbeHandler(runner, logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestDBORMProbeDemoServesTheMeasuredExhibit(t *testing.T) {
	var buf strings.Builder
	server := newORMDemoServer(fakeORMProbeRunner{observation: ormProbeFixtureObservation()}, newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/db/orm-probe", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q, want application/json", ct)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	decoded := struct {
		Dataset struct {
			Posts int `json:"posts"`
			Tags  int `json:"tags"`
			Edges int `json:"edges"`
		} `json:"dataset"`
		Trials []struct {
			Strategy        string   `json:"strategy"`
			Statements      int      `json:"statements"`
			RoundTripRatio  string   `json:"roundTripRatio"`
			Rows            int      `json:"rows"`
			DurationMicros  int64    `json:"durationMicros"`
			SampleStatement []string `json:"sampleStatements"`
		} `json:"trials"`
		ResultsAgree bool `json:"resultsAgree"`
		UnitOfWork   struct {
			Repositories []string `json:"repositories"`
			HookApplied  bool     `json:"hookApplied"`
			PostsBefore  int      `json:"postsBefore"`
			PostsAfter   int      `json:"postsAfter"`
			RolledBack   bool     `json:"rolledBack"`
		} `json:"unitOfWork"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}

	if decoded.Dataset.Posts != 12 || decoded.Dataset.Tags != 5 || decoded.Dataset.Edges != 24 {
		t.Errorf("dataset drifted: %+v", decoded.Dataset)
	}
	if len(decoded.Trials) != 4 {
		t.Fatalf("trials = %d, want the four measured strategies", len(decoded.Trials))
	}
	wantCounts := map[string]int{
		"pgx-baseline": 1, "sqlc-join": 1, "gorm-lazy": 13, "gorm-eager": 3,
	}
	for _, trial := range decoded.Trials {
		if trial.Statements != wantCounts[trial.Strategy] {
			t.Errorf("%s statements = %d, want %d", trial.Strategy, trial.Statements, wantCounts[trial.Strategy])
		}
		if trial.Rows != 12 || len(trial.SampleStatement) == 0 {
			t.Errorf("%s trial drifted: %+v", trial.Strategy, trial)
		}
	}
	if decoded.Trials[2].RoundTripRatio != "13x" {
		t.Errorf("lazy ratio = %q, want 13x", decoded.Trials[2].RoundTripRatio)
	}
	if !decoded.ResultsAgree {
		t.Error("the payload must state the apples-to-apples verdict")
	}
	unit := decoded.UnitOfWork
	if !unit.RolledBack || !unit.HookApplied || unit.PostsBefore != 12 || unit.PostsAfter != 12 {
		t.Errorf("unit of work drifted: %+v", unit)
	}
	if len(unit.Repositories) != 2 {
		t.Errorf("unit repositories = %v, want the two repositories", unit.Repositories)
	}
}

func TestDBORMProbeDemoDegradesTo503WhenTheDatabaseIsUnreachable(t *testing.T) {
	var buf strings.Builder
	server := newORMDemoServer(
		fakeORMProbeRunner{err: errors.New("ping the probe's database: connect: connection refused")},
		newTestLogger(&buf))

	rec := httptest.NewRequest(http.MethodGet, "/api/demo/db/orm-probe", nil)
	rec.Header.Set(core.RequestIDHeader, "rid-orm-503")
	writer := httptest.NewRecorder()
	server.ServeHTTP(writer, rec)

	if writer.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", writer.Code)
	}
	body := writer.Body.String()
	if !strings.Contains(body, `"code":"service_unavailable"`) || !strings.Contains(body, `"requestId":"rid-orm-503"`) {
		t.Errorf("503 body is not the honest service_unavailable envelope: %s", body)
	}
	if !strings.Contains(buf.String(), "rid-orm-503") {
		t.Errorf("503 must be logged with its request id:\n%s", buf.String())
	}
}

func TestDBORMProbeDemoEndpointRejectsNonGet(t *testing.T) {
	var buf strings.Builder
	server := newORMDemoServer(fakeORMProbeRunner{observation: ormProbeFixtureObservation()}, newTestLogger(&buf))

	rec := doRequest(server, http.MethodPost, "/api/demo/db/orm-probe", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("POST Allow header lost: %q", allow)
	}
}
