package demos

import (
	"strings"
	"testing"
)

// The full-text-search builder's pins: the fixed query, the stemmed
// tsquery, ranked matches with formatted ranks, the plan lines, the
// catalog's own index definition, and the never-empty guarantees.

func TestDBSearchPayloadCarriesQueryRanksPlanAndIndex(t *testing.T) {
	t.Parallel()

	observation := SQLSearchObservation{
		TsQuery: "'log' | 'error'",
		Matches: []SearchMatchRow{
			{Title: "Structured logging with slog", Rank: "0.0608"},
			{Title: "Errors are values", Rank: "0.0304"},
		},
		Plan:            []string{"Seq Scan on posts  (cost=0.00..1.06 rows=6 width=…)", "  Filter: (to_tsvector(…) @@ …)"},
		IndexDefinition: "CREATE INDEX posts_search_idx ON public.posts USING gin (to_tsvector('english'::regconfig, ((title || ' '::text) || body)))",
	}
	payload := BuildSQLSearchPayload(observation)

	if payload.Query != "logging or errors" {
		t.Errorf("query = %q, want the fixed demo input", payload.Query)
	}
	if payload.TsQuery != "'log' | 'error'" {
		t.Errorf("tsQuery = %q, want the stemmed pair", payload.TsQuery)
	}
	if !strings.Contains(payload.SQL, "websearch_to_tsquery") || !strings.Contains(payload.SQL, "ts_rank") {
		t.Errorf("the displayed SQL must be the executed statement, got %q", payload.SQL)
	}
	if len(payload.Matches) != 2 || payload.Matches[0].Title != "Structured logging with slog" {
		t.Errorf("matches drifted: %+v", payload.Matches)
	}
	if len(payload.Plan) != 2 || !strings.Contains(payload.Plan[0], "Seq Scan") {
		t.Errorf("plan lines drifted: %q", payload.Plan)
	}
	if !strings.Contains(payload.IndexDefinition, "USING gin") {
		t.Errorf("index definition must be the catalog's own gin rendering: %q", payload.IndexDefinition)
	}
	if payload.Teach == "" {
		t.Error("the teach line is missing")
	}
}

func TestDBSearchPayloadMaterializesEmptyCollections(t *testing.T) {
	t.Parallel()

	payload := BuildSQLSearchPayload(SQLSearchObservation{})
	if payload.Matches == nil || payload.Plan == nil {
		t.Error("matches and plan must render [] — never null")
	}
}
