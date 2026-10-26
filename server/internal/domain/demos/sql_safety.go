// Topic 17 databases-sql's injection demo: parameterized queries versus
// string concatenation, run against the real posts table. The runner (in
// internal/data/db) executes the PARAMETERIZED form with the hostile
// inputs as data — every attempt lands as a title nobody stored, zero
// rows, table intact — and this builder assembles the anti-pattern
// exhibit: the exact SQL string concatenation WOULD have produced, shown
// as the museum piece it is, never executed.
package demos

import (
	"context"
	"fmt"
)

// SQLSafetyRunner is the port the demo reads through: it runs the
// parameterized lookup for each attack input and reports what the
// database saw. The pgx-backed implementation lives in internal/data/db.
type SQLSafetyRunner interface {
	// ObserveSQLSafety executes the parameterized statement once per
	// attack input and reports matched rows plus the table's live count.
	ObserveSQLSafety(ctx context.Context, attacks []string) (SQLSafetyObservation, error)
}

// SQLSafetyAttempt is ONE attack's live result: what was sent, what the
// parameterized path returned, and how many posts exist afterwards.
type SQLSafetyAttempt struct {
	// Input is the hostile string, exactly as it arrived.
	Input string
	// SQL is the statement that actually executed (the parameterized
	// form, $1 and all — the same text the payload displays).
	SQL string
	// RowsMatched is what the lookup returned: the attack compared as
	// DATA against every title, matching none.
	RowsMatched int
	// PostsTotal is the live row count AFTER the attempt — the table the
	// DROP-style payloads wanted is still there, fully populated.
	PostsTotal int
}

// SQLSafetyObservation is the runner's raw haul: one attempt per attack
// input, in attack order.
type SQLSafetyObservation struct {
	Attempts []SQLSafetyAttempt
}

// SQLSafetyAttackRow is one attack as the payload serves it: the live
// parameterized outcome beside the never-executed concatenation exhibit.
type SQLSafetyAttackRow struct {
	Input            string `json:"input"`
	Intent           string `json:"intent"`
	ParameterizedSQL string `json:"parameterizedSql"`
	RowsMatched      int    `json:"rowsMatched"`
	PostsAfter       int    `json:"postsAfter"`
	AssembledSQL     string `json:"assembledSql"`
	AssembledEffect  string `json:"assembledEffect"`
	Verdict          string `json:"verdict"`
}

// SQLSafetyPayload is the wire shape of GET /api/demo/db/sql-safety
// (camelCase JSON), deterministic for a seeded database: zero matches,
// the full seed count, and verdicts that never move.
type SQLSafetyPayload struct {
	Attacks []SQLSafetyAttackRow `json:"attacks"`
	Summary string               `json:"summary"`
}

// sqlSafetyAttacks are the demo's canonical hostile inputs, with the
// intent each represents and the effect the CONCATENATED form would have
// had — the strings a builder must assemble to exhibit, never to run.
var sqlSafetyAttacks = []struct {
	input  string
	intent string
	effect string
}{
	{
		input:  "x' OR '1'='1",
		intent: "escape the string literal and match EVERY row",
		effect: "the WHERE clause becomes title = 'x' OR '1'='1 — always true, every post leaks",
	},
	{
		input:  "'; DROP TABLE posts; --",
		intent: "terminate the statement and execute a second one",
		effect: "two statements would run: the lookup, then DROP TABLE posts",
	},
}

// SQLSafetyAttackInputs returns the canonical hostile inputs, in exhibit
// order — the runner executes each as $1 and the builder pairs the
// results with the same slice, so the two can never drift.
func SQLSafetyAttackInputs() []string {
	return []string{sqlSafetyAttacks[0].input, sqlSafetyAttacks[1].input}
}

// BuildSQLSafetyPayload shapes the live observation into the demo
// payload: each attack's real parameterized outcome beside the
// concatenation exhibit, plus the one-sentence verdict the whole demo
// exists to earn.
func BuildSQLSafetyPayload(observation SQLSafetyObservation) SQLSafetyPayload {
	rows := make([]SQLSafetyAttackRow, 0, len(sqlSafetyAttacks))
	for i, attack := range sqlSafetyAttacks {
		live := SQLSafetyAttempt{PostsTotal: -1}
		if i < len(observation.Attempts) {
			live = observation.Attempts[i]
		}
		rows = append(rows, SQLSafetyAttackRow{
			Input:            attack.input,
			Intent:           attack.intent,
			ParameterizedSQL: live.SQL,
			RowsMatched:      live.RowsMatched,
			PostsAfter:       live.PostsTotal,
			AssembledSQL:     fmt.Sprintf("SELECT count(*) FROM posts WHERE title = '%s'", attack.input),
			AssembledEffect:  attack.effect,
			Verdict:          "inert — bound as data, compared as data",
		})
	}
	return SQLSafetyPayload{
		Attacks: rows,
		Summary: "The parameterized protocol sends the SQL text and the values as separate messages: " +
			"the driver cannot confuse them and neither can the input.",
	}
}
