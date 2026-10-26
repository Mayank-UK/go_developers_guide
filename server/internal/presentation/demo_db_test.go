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

// The sql-schema topic's two demo endpoints, exercised through the same
// middleware chain production runs behind (the family rule that every new
// demo endpoint ships with a Go test). The schema explorer's port is
// faked here — the pgx-backed introspector has its own tests against the
// real PostgreSQL in internal/data/db — so both the happy path and the
// honest 503 degradation are pinned without a database.

// fakeIntrospector stands in for the pgx-backed introspector: it returns a
// canned snapshot shaped like the real one (a curated slice of the actual
// schema) or the failure its field carries.
type fakeIntrospector struct {
	snapshot demos.SchemaSnapshot
	err      error
}

func (f fakeIntrospector) Introspect(context.Context) (demos.SchemaSnapshot, error) {
	return f.snapshot, f.err
}

// explorerFixtureSnapshot mirrors the live schema's shape closely enough
// to pin the handler's mapping: two tables with columns, one constraint,
// one index, two migrations in the ledger.
func explorerFixtureSnapshot() demos.SchemaSnapshot {
	return demos.SchemaSnapshot{
		Tables: []string{"audit_log", "posts"},
		Columns: []demos.ColumnRow{
			{Table: "posts", Name: "id", Type: "uuid", Nullable: false, HasDefault: true, Default: "gen_random_uuid()"},
			{Table: "posts", Name: "title", Type: "text", Nullable: false},
			{Table: "audit_log", Name: "post_id", Type: "uuid", Nullable: true},
		},
		Constraints: []demos.ConstraintRow{
			{Table: "posts", Name: "posts_pkey", Kind: "PRIMARY KEY", Definition: "PRIMARY KEY (id)"},
			{Table: "audit_log", Name: "audit_log_post_id_fkey", Kind: "FOREIGN KEY",
				Definition: "FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE SET NULL"},
		},
		Indexes: []demos.IndexRow{
			{Table: "posts", Name: "posts_created_at_idx", Kind: "btree", Columns: []string{"created_at"},
				Definition: "CREATE INDEX posts_created_at_idx ON public.posts USING btree (created_at DESC)"},
		},
		History: []demos.MigrationRow{
			{Version: 1, Applied: true, AppliedAt: "2026-10-26 21:20:00"},
			{Version: 2, Applied: true, AppliedAt: "2026-10-26 21:20:00"},
		},
	}
}

func newDBDemoServer(introspector demos.SchemaIntrospector, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/db/schema-explorer", NewDBSchemaExplorerHandler(introspector, logger))
	mux.Handle("GET /api/demo/db/uuid", NewDBUUIDHandler(logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestDBSchemaExplorerDemoServesTheShapedSnapshot(t *testing.T) {
	var buf strings.Builder
	server := newDBDemoServer(fakeIntrospector{snapshot: explorerFixtureSnapshot()}, newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/db/schema-explorer", "")
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
		Tables []struct {
			Name    string `json:"name"`
			Purpose string `json:"purpose"`
			Columns []struct {
				Name    string `json:"name"`
				Type    string `json:"type"`
				Default string `json:"default"`
			} `json:"columns"`
		} `json:"tables"`
		Constraints []struct {
			Table      string `json:"table"`
			Kind       string `json:"kind"`
			Definition string `json:"definition"`
		} `json:"constraints"`
		Indexes []struct {
			Name string   `json:"name"`
			Kind string   `json:"kind"`
			Cols []string `json:"columns"`
		} `json:"indexes"`
		History []struct {
			Version int64 `json:"version"`
		} `json:"history"`
		Notes []struct {
			Subject string `json:"subject"`
		} `json:"notes"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if len(decoded.Tables) != 2 || decoded.Tables[0].Name != "audit_log" || decoded.Tables[1].Name != "posts" {
		t.Errorf("tables drifted: %+v", decoded.Tables)
	}
	if decoded.Tables[1].Purpose == "" {
		t.Error("the posts table must carry its curated purpose")
	}
	if got := decoded.Tables[1].Columns[0]; got.Name != "id" || got.Type != "uuid" || got.Default != "gen_random_uuid()" {
		t.Errorf("posts.id column drifted: %+v", got)
	}
	if len(decoded.Constraints) != 2 || decoded.Constraints[0].Kind != "FOREIGN KEY" ||
		!strings.Contains(decoded.Constraints[0].Definition, "ON DELETE SET NULL") {
		t.Errorf("constraints drifted: %+v", decoded.Constraints)
	}
	if len(decoded.Indexes) != 1 || decoded.Indexes[0].Kind != "btree" || len(decoded.Indexes[0].Cols) != 1 {
		t.Errorf("indexes drifted: %+v", decoded.Indexes)
	}
	if len(decoded.History) != 2 || decoded.History[0].Version != 1 || decoded.History[1].Version != 2 {
		t.Errorf("migration history drifted: %+v", decoded.History)
	}
	if len(decoded.Notes) == 0 {
		t.Error("curated rationale notes were dropped entirely")
	}
}

func TestDBSchemaExplorerDemoDegradesTo503WhenTheDatabaseIsUnreachable(t *testing.T) {
	var buf strings.Builder
	server := newDBDemoServer(fakeIntrospector{err: errors.New("connect: connection refused")}, newTestLogger(&buf))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/demo/db/schema-explorer", nil)
	req.Header.Set(core.RequestIDHeader, "rid-db-503")
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"service_unavailable"`) || !strings.Contains(body, `"requestId":"rid-db-503"`) {
		t.Errorf("503 body is not the honest service_unavailable envelope: %s", body)
	}
	if !strings.Contains(buf.String(), "rid-db-503") {
		t.Errorf("503 must be logged with its request id:\n%s", buf.String())
	}
}

func TestDBUUIDLabDemoServesAFreshBatch(t *testing.T) {
	var buf strings.Builder
	server := newDBDemoServer(fakeIntrospector{snapshot: explorerFixtureSnapshot()}, newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/db/uuid", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	decoded := struct {
		Layout []struct {
			Field string `json:"field"`
			Bits  int    `json:"bits"`
		} `json:"layout"`
		Anatomy struct {
			Value   string `json:"value"`
			Version int    `json:"version"`
			Length  int    `json:"length"`
		} `json:"anatomy"`
		Batch struct {
			Count                         int      `json:"count"`
			GeneratedInOrder              []string `json:"generatedInOrder"`
			SortedLexically               []string `json:"sortedLexically"`
			SortingMatchesGenerationOrder bool     `json:"sortingMatchesGenerationOrder"`
		} `json:"batch"`
		Collision struct {
			Generated     int  `json:"generated"`
			Distinct      int  `json:"distinct"`
			CollisionFree bool `json:"collisionFree"`
		} `json:"collision"`
		Comparison []struct {
			Strategy string `json:"strategy"`
		} `json:"comparison"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if len(decoded.Layout) != 5 {
		t.Errorf("layout rows = %d, want the five RFC 9562 fields", len(decoded.Layout))
	}
	if decoded.Anatomy.Version != 7 || decoded.Anatomy.Length != 36 {
		t.Errorf("anatomy drifted: %+v", decoded.Anatomy)
	}
	if decoded.Batch.Count != 8 || len(decoded.Batch.GeneratedInOrder) != 8 || len(decoded.Batch.SortedLexically) != 8 {
		t.Errorf("batch drifted: %+v", decoded.Batch)
	}
	if !decoded.Batch.SortingMatchesGenerationOrder {
		t.Error("lexical sorting must match generation order — the generator's monotonicity contract")
	}
	for _, value := range decoded.Batch.GeneratedInOrder {
		if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
			t.Errorf("batch value %q is not canonical 8-4-4-4-12 form", value)
		}
		if value[14] != '7' {
			t.Errorf("batch value %q does not carry version 7 in its third group", value)
		}
	}
	if decoded.Collision.Generated != 10000 || decoded.Collision.Distinct != 10000 || !decoded.Collision.CollisionFree {
		t.Errorf("collision check drifted: %+v", decoded.Collision)
	}
	if len(decoded.Comparison) != 3 || decoded.Comparison[0].Strategy != "bigserial" {
		t.Errorf("comparison rows drifted: %+v", decoded.Comparison)
	}
}

func TestDBDemoEndpointsRejectNonGet(t *testing.T) {
	var buf strings.Builder
	server := newDBDemoServer(fakeIntrospector{snapshot: explorerFixtureSnapshot()}, newTestLogger(&buf))

	for _, path := range []string{"/api/demo/db/schema-explorer", "/api/demo/db/uuid"} {
		rec := doRequest(server, http.MethodPost, path, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s status = %d, want 405", path, rec.Code)
		}
		if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
			t.Errorf("POST %s Allow header lost: %q", path, allow)
		}
	}
}
