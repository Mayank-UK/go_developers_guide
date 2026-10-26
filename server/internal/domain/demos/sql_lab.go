// Topic 17 databases-sql's SQL-by-hand demo: a CTE chain and two window
// functions over the SEEDED posts, each served with the exact SQL that
// produced it and the rows as they came back. The queries read only, so
// the demo is deterministic for the guide's fixed six-post collection —
// the same rows every load, on the fake's clock or the live store's.
package demos

import (
	"context"
)

// SQLLabRunner is the port the demo reads through. The pgx-backed
// implementation lives in internal/data/db.
type SQLLabRunner interface {
	// RunSQLLab executes the three exhibits against the seeded posts and
	// returns their raw rows.
	RunSQLLab(ctx context.Context) (SQLLabObservation, error)
}

// TagTotalRow is exhibit A's row: one tag with how many posts carry it
// (the CTE chain's aggregate).
type TagTotalRow struct {
	Tag   string
	Posts int
}

// LengthRankRow is exhibit B's row: one post with its body length in
// runes and its RANK among all posts by that measure.
type LengthRankRow struct {
	Title string
	Runes int
	Rank  int
}

// PublishGapRow is exhibit C's row: one post with the whole days between
// it and the previous post (LAG over the ordered timeline); the oldest
// post has no previous, expressed by HasGap=false.
type PublishGapRow struct {
	Title   string
	Created string // YYYY-MM-DD, the date the seed pinned
	Days    int
	HasGap  bool
}

// SQLLabObservation is the runner's raw haul, one slice per exhibit.
type SQLLabObservation struct {
	TagTotals   []TagTotalRow
	LengthRanks []LengthRankRow
	PublishGaps []PublishGapRow
}

// SQLLabQuery is one exhibit as the payload serves it: its id, its SQL,
// what it teaches, its columns, and its rows (already shaped as flat
// string rows — the demo renders, it does not compute).
type SQLLabQuery struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	SQL     string   `json:"sql"`
	Teach   string   `json:"teach"`
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// SQLLabPayload is the wire shape of GET /api/demo/db/sql-lab (camelCase
// JSON): three exhibits, rows deterministic over the seed.
type SQLLabPayload struct {
	Queries []SQLLabQuery `json:"queries"`
}

// sqlLabExhibits carries each exhibit's display metadata, keyed by the
// observation it consumes — the SQL text mirrors the statements the
// runner executes (they live in internal/data/db/sql_demos.go, where the
// topic's must-source quotes point).
var sqlLabExhibits = []struct {
	id      string
	title   string
	teach   string
	columns []string
}{
	{
		id:      "tag-totals",
		title:   "A CTE chain: name the intermediate, then aggregate it",
		teach:   "WITH gives every stage a NAME — the edges are filtered to the seeded ids ($1), joined and counted once, then grouped — so the final query reads as its own explanation instead of a nested subquery pile.",
		columns: []string{"tag", "posts"},
	},
	{
		id:      "rank-by-length",
		title:   "RANK(): compare every row against its peers in one pass",
		teach:   "RANK() OVER (ORDER BY …) numbers rows by a measure WITHOUT collapsing them — every post stays, each carries its standing; ties share a rank and the next rank skips.",
		columns: []string{"title", "runes", "rank"},
	},
	{
		id:      "publish-gaps",
		title:   "LAG(): reach back one row in an ordered window",
		teach:   "LAG(created_at) hands each row the previous row's value — subtracting the two turns a timeline into the DAYS BETWEEN posts, something the relational model has no column for.",
		columns: []string{"title", "created", "daysSincePrevious"},
	},
}

// BuildSQLLabPayload shapes the observation into the demo payload: one
// exhibit per query family, rows flattened into display cells, the
// gap-less oldest row rendering null as an explicit "—" cell.
func BuildSQLLabPayload(observation SQLLabObservation) SQLLabPayload {
	queries := make([]SQLLabQuery, 0, len(sqlLabExhibits))
	for _, exhibit := range sqlLabExhibits {
		query := SQLLabQuery{ID: exhibit.id, Title: exhibit.title, Teach: exhibit.teach, Columns: exhibit.columns}
		switch exhibit.id {
		case "tag-totals":
			query.SQL = SQLTagTotals
			for _, row := range observation.TagTotals {
				query.Rows = append(query.Rows, []any{row.Tag, row.Posts})
			}
		case "rank-by-length":
			query.SQL = SQLRankByLength
			for _, row := range observation.LengthRanks {
				query.Rows = append(query.Rows, []any{row.Title, row.Runes, row.Rank})
			}
		case "publish-gaps":
			query.SQL = SQLPublishGaps
			for _, row := range observation.PublishGaps {
				gap := any("—")
				if row.HasGap {
					gap = row.Days
				}
				query.Rows = append(query.Rows, []any{row.Title, row.Created, gap})
			}
		}
		if query.Rows == nil {
			query.Rows = [][]any{}
		}
		queries = append(queries, query)
	}
	return SQLLabPayload{Queries: queries}
}

// The three statements, displayed verbatim in the payload and executed
// by the data layer's runner — ONE source of truth: the runner (in
// internal/data/db, which imports this package) executes these exported
// constants, so the payload can never drift from what actually ran.
// (The opening backtick sits alone on the declaration line so every
// subsequent line of the constant is the SQL itself, verbatim — the
// snippet matcher quotes these lines as they are.)
const (
	SQLTagTotals = `
WITH tagged AS (
    SELECT pt.tag_name AS tag, count(*) AS posts
    FROM post_tags pt
    WHERE pt.post_id = ANY($1::uuid[])
    GROUP BY pt.tag_name
)
SELECT tag, posts FROM tagged ORDER BY tag
`

	SQLRankByLength = `
SELECT title, length(body) AS runes,
       RANK() OVER (ORDER BY length(body) DESC) AS rank
FROM posts
WHERE id = ANY($1::uuid[])
ORDER BY rank, title
`

	SQLPublishGaps = `
SELECT title,
       to_char(created_at, 'YYYY-MM-DD') AS created,
       (created_at::date - LAG(created_at::date) OVER (ORDER BY created_at)) AS days_since_previous
FROM posts
WHERE id = ANY($1::uuid[])
ORDER BY created_at
`
)
