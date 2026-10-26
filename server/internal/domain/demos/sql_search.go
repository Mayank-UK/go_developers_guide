// Topic 17 databases-sql's full-text search demo: tsvector matching,
// ts_rank ordering, and the GIN expression index that serves it — over
// the seeded posts, with the query plan PostgreSQL actually chose (plain
// EXPLAIN: the plan, without the timing noise ANALYZE adds; the terminal
// companion runs the ANALYZE version live). The query is fixed, so the
// matches, their ranks, and the plan are deterministic on the seed.
package demos

import (
	"context"
)

// SQLSearchRunner is the port the demo reads through. The pgx-backed
// implementation lives in internal/data/db.
type SQLSearchRunner interface {
	// RunSearch executes the fixed demo query and the EXPLAIN that shows
	// how PostgreSQL answered it.
	RunSearch(ctx context.Context) (SQLSearchObservation, error)
}

// SearchMatchRow is one ranked hit: the post's title and its ts_rank.
type SearchMatchRow struct {
	Title string
	Rank  string
}

// SQLSearchObservation is the runner's raw haul: the stemmed query text
// PostgreSQL built, the ranked matches, the EXPLAIN plan lines, and the
// index definition straight from the catalog.
type SQLSearchObservation struct {
	TsQuery         string
	Matches         []SearchMatchRow
	Plan            []string
	IndexDefinition string
}

// SearchMatch is one hit as the payload serves it.
type SearchMatch struct {
	Title string `json:"title"`
	Rank  string `json:"rank"`
}

// SQLSearchPayload is the wire shape of GET /api/demo/db/search
// (camelCase JSON): the fixed natural-language query, the tsquery it
// stemmed into, the ranked rows, the SQL, the plan, and the index.
type SQLSearchPayload struct {
	Query           string        `json:"query"`
	TsQuery         string        `json:"tsQuery"`
	SQL             string        `json:"sql"`
	Matches         []SearchMatch `json:"matches"`
	Plan            []string      `json:"plan"`
	IndexDefinition string        `json:"indexDefinition"`
	Teach           string        `json:"teach"`
}

// SearchDemoQuery is the demo's fixed natural-language input. The
// websearch syntax's "or" keeps both stems in play — plainto_tsquery
// would AND them, and no single post carries both.
const SearchDemoQuery = "logging or errors"

// SearchDemoSQL is the statement the runner executes — one source of
// truth with the payload's display (the runner imports this constant).
// websearch_to_tsquery is the user-facing entry: quotes, "or", and minus
// work the way people already type.
// (The opening backtick sits alone so every line below is the SQL
// itself, verbatim — the topic quotes these lines as they are.)
const SearchDemoSQL = `
SELECT title,
       ts_rank(to_tsvector('english', title || ' ' || body),
               websearch_to_tsquery('english', $1)) AS rank
FROM posts
WHERE to_tsvector('english', title || ' ' || body)
      @@ websearch_to_tsquery('english', $1)
ORDER BY rank DESC, title
`

// BuildSQLSearchPayload shapes the observation into the demo payload.
func BuildSQLSearchPayload(observation SQLSearchObservation) SQLSearchPayload {
	matches := make([]SearchMatch, 0, len(observation.Matches))
	for _, row := range observation.Matches {
		matches = append(matches, SearchMatch{Title: row.Title, Rank: row.Rank})
	}
	plan := append([]string(nil), observation.Plan...)
	if plan == nil {
		plan = []string{}
	}
	return SQLSearchPayload{
		Query:           SearchDemoQuery,
		TsQuery:         observation.TsQuery,
		SQL:             SearchDemoSQL,
		Matches:         matches,
		Plan:            plan,
		IndexDefinition: observation.IndexDefinition,
		Teach: "websearch_to_tsquery stems the input ('logging or errors' becomes 'log' | 'error'), " +
			"to_tsvector stems the indexed text the same way, and @@ asks the GIN index which rows carry " +
			"a stem — ts_rank then orders the survivors by how heavily they match.",
	}
}
