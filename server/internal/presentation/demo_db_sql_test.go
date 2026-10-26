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

// The topic 17 demo endpoints, exercised through the same middleware
// chain production runs behind. The runner ports are faked here — the
// pgx-backed runner has its own tests against throwaway databases in
// internal/data/db — so the happy payloads AND the honest 503
// degradation are pinned without a database.

// fakeSQLDemos serves canned observations for all six ports, or the
// failure its field carries.
type fakeSQLDemos struct {
	err error
}

func (f fakeSQLDemos) ObserveSQLSafety(ctx context.Context, attacks []string) (demos.SQLSafetyObservation, error) {
	attempts := make([]demos.SQLSafetyAttempt, 0, len(attacks))
	for _, attack := range attacks {
		attempts = append(attempts, demos.SQLSafetyAttempt{Input: attack, SQL: "SELECT count(*) FROM posts WHERE title = $1", RowsMatched: 0, PostsTotal: 6})
	}
	return demos.SQLSafetyObservation{Attempts: attempts}, f.err
}

func (f fakeSQLDemos) RunTxStory(context.Context) (demos.TxStoryObservation, error) {
	return demos.TxStoryObservation{
		Steps: []demos.TxStoryRow{
			{Action: "begin", SQL: "BEGIN", Count: 6},
			{Action: "insert scratch post", SQL: "INSERT", Count: 7},
			{Action: "savepoint sp1", SQL: "SAVEPOINT sp1", Count: 7},
			{Action: "insert second post", SQL: "INSERT", Count: 8},
			{Action: "rollback to savepoint", SQL: "ROLLBACK TO SAVEPOINT sp1", Count: 7},
			{Action: "rollback", SQL: "ROLLBACK", Count: 6},
		},
		History: []demos.MigrationRow{{Version: 0, Applied: true, AppliedAt: "2026-10-26"}},
	}, f.err
}

func (f fakeSQLDemos) RunSQLLab(context.Context) (demos.SQLLabObservation, error) {
	return demos.SQLLabObservation{
		TagTotals:   []demos.TagTotalRow{{Tag: "go", Posts: 4}},
		LengthRanks: []demos.LengthRankRow{{Title: "Structured logging with slog", Runes: 233, Rank: 1}},
		PublishGaps: []demos.PublishGapRow{{Title: "Why Go for backends", Created: "2026-01-05"}, {Title: "ServeMux patterns in Go 1.22", Created: "2026-01-07", Days: 2, HasGap: true}},
	}, f.err
}

func (f fakeSQLDemos) RunSearch(context.Context) (demos.SQLSearchObservation, error) {
	return demos.SQLSearchObservation{
		TsQuery:         "'log' | 'error'",
		Matches:         []demos.SearchMatchRow{{Title: "Errors are values", Rank: "0.0414"}},
		Plan:            []string{"Seq Scan on posts"},
		IndexDefinition: "CREATE INDEX posts_search_idx ON public.posts USING gin (to_tsvector(…))",
	}, f.err
}

func (f fakeSQLDemos) RunRace(context.Context) (demos.RaceObservation, error) {
	return demos.RaceObservation{
		Isolation: "read committed",
		Steps: []demos.RaceStepRow{
			{Phase: "uncommitted-write", Actor: "T1 (writer)", Action: "UPDATE", Observed: "x"},
			{Phase: "reader-looks", Actor: "T2 (reader)", Action: "SELECT", Observed: "race demo scratch"},
			{Phase: "writer-commits", Actor: "T1 (writer)", Action: "COMMIT", Observed: "x"},
			{Phase: "reader-looks-again", Actor: "T2 (reader)", Action: "SELECT", Observed: "race demo scratch (updated)"},
			{Phase: "lock-acquired", Actor: "T1 (writer)", Action: "FOR UPDATE", Observed: "x"},
			{Phase: "lock-waited", Actor: "T2 (reader)", Action: "FOR UPDATE", Observed: "waited for the lock"},
		},
	}, f.err
}

func (f fakeSQLDemos) RunAuditStory(context.Context) (demos.AuditObservation, error) {
	return demos.AuditObservation{Steps: []demos.AuditStepRow{
		{Phase: "commit-open", Action: "BEGIN", LiveTitle: "(no post yet)", AuditRows: 0},
		{Phase: "commit-change", Action: "two writes", LiveTitle: "audit scratch (edited)", AuditRows: 2},
		{Phase: "commit-done", Action: "COMMIT", LiveTitle: "audit scratch (edited)", AuditRows: 2},
		{Phase: "rollback-open", Action: "BEGIN", LiveTitle: "audit scratch (edited)", AuditRows: 2},
		{Phase: "rollback-inside", Action: "two writes", LiveTitle: "audit scratch (doomed)", AuditRows: 3},
		{Phase: "rollback-done", Action: "ROLLBACK", LiveTitle: "audit scratch (edited)", AuditRows: 2},
	}}, f.err
}

var sqlDemoRoutes = []string{
	"/api/demo/db/sql-safety",
	"/api/demo/db/migrations",
	"/api/demo/db/sql-lab",
	"/api/demo/db/search",
	"/api/demo/db/race",
	"/api/demo/db/audit",
}

// NewDBSQLDemoHandler dispatches one route path to its handler — the
// test harness's mirror of main.go's six explicit registrations.
func NewDBSQLDemoHandler(path string, runner fakeSQLDemos, logger *slog.Logger) http.Handler {
	switch path {
	case "/api/demo/db/sql-safety":
		return NewDBSQLSafetyHandler(runner, logger)
	case "/api/demo/db/migrations":
		return NewDBMigrationsHandler(runner, logger)
	case "/api/demo/db/sql-lab":
		return NewDBSQLLabHandler(runner, logger)
	case "/api/demo/db/search":
		return NewDBSearchHandler(runner, logger)
	case "/api/demo/db/race":
		return NewDBRaceHandler(runner, logger)
	default:
		return NewDBAuditHandler(runner, logger)
	}
}

func newSQLDemoServer(runner fakeSQLDemos, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	for _, route := range sqlDemoRoutes {
		mux.Handle("GET "+route, NewDBSQLDemoHandler(route, runner, logger))
	}
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestDBSQLDemoEndpointsServeTheirPayloads(t *testing.T) {
	var buf strings.Builder
	server := newSQLDemoServer(fakeSQLDemos{}, newTestLogger(&buf))

	cases := []struct {
		path     string
		required []string
	}{
		{sqlDemoRoutes[0], []string{`"attacks"`, `"assembledSql"`, `"inert — bound as data, compared as data"`}},
		{sqlDemoRoutes[1], []string{`"history"`, `"steps"`, `"savepoint sp1"`}},
		{sqlDemoRoutes[2], []string{`"tag-totals"`, `"rank-by-length"`, `"publish-gaps"`}},
		{sqlDemoRoutes[3], []string{`"tsQuery"`, `"matches"`, `"indexDefinition"`}},
		{sqlDemoRoutes[4], []string{`"read committed"`, `"lockWaited":true`, `"serializableNote"`}},
		{sqlDemoRoutes[5], []string{`"committedTogether":true`, `"rolledBackTogether":true`, `"auditRows"`}},
	}
	for _, tc := range cases {
		rec := doRequest(server, http.MethodGet, tc.path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", tc.path, rec.Code)
		}
		var decoded map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("GET %s body not a JSON object: %v", tc.path, err)
		}
		for _, want := range tc.required {
			if !strings.Contains(rec.Body.String(), want) {
				t.Errorf("GET %s body missing %q", tc.path, want)
			}
		}
		if rec.Header().Get(core.RequestIDHeader) == "" {
			t.Errorf("GET %s must carry x-request-id", tc.path)
		}
	}
}

func TestDBSQLDemoEndpointsDegradeTo503WhenTheDatabaseIsUnreachable(t *testing.T) {
	var buf strings.Builder
	server := newSQLDemoServer(fakeSQLDemos{err: errors.New("connect: connection refused")}, newTestLogger(&buf))

	for _, path := range sqlDemoRoutes {
		rec := doRequest(server, http.MethodGet, path, "")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("GET %s status = %d, want 503", path, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `"code":"service_unavailable"`) {
			t.Errorf("GET %s body is not the honest envelope: %s", path, body)
		}
	}
	if !strings.Contains(buf.String(), "service_unavailable") && !strings.Contains(buf.String(), "503") {
		t.Errorf("the 503s must be logged:\n%s", buf.String())
	}
}

func TestDBSQLDemoEndpointsRejectNonGet(t *testing.T) {
	var buf strings.Builder
	server := newSQLDemoServer(fakeSQLDemos{}, newTestLogger(&buf))

	for _, path := range sqlDemoRoutes {
		rec := doRequest(server, http.MethodPost, path, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s status = %d, want 405", path, rec.Code)
		}
		if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
			t.Errorf("POST %s Allow header lost: %q", path, allow)
		}
	}
}
