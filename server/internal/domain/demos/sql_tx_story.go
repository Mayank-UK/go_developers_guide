// Topic 17 databases-sql's transactions demo: one real transaction
// round-trip — BEGIN, INSERT, SAVEPOINT, a second INSERT, ROLLBACK TO the
// savepoint, ROLLBACK the whole thing — narrated by the counts observed
// INSIDE it, beside goose's migration ledger (the version-table history
// that makes re-running migrations a no-op, reprise from topic 16). The
// story runs on a scratch post and rolls back entirely, so the running
// demo leaves the database exactly as it found it.
package demos

import (
	"context"
	"fmt"
)

// SQLTxRunner is the port the demo reads through. The pgx-backed
// implementation lives in internal/data/db.
type SQLTxRunner interface {
	// RunTxStory executes the begin / savepoint / rollback dance on a
	// scratch row and reports the post count observed at every step,
	// plus goose's ledger.
	RunTxStory(ctx context.Context) (TxStoryObservation, error)
}

// TxStoryRow is one observed step: what was done, the statement that did
// it, and how many posts the transaction's own SELECT count(*) reported
// right afterwards (0 for the steps outside any transaction's snapshot).
type TxStoryRow struct {
	Action string
	SQL    string
	Count  int
}

// TxStoryObservation is the runner's raw haul: the steps in execution
// order and the goose ledger rows.
type TxStoryObservation struct {
	Steps   []TxStoryRow
	History []MigrationRow
}

// TxStoryStep is one step as the payload serves it.
type TxStoryStep struct {
	Step   int    `json:"step"`
	Action string `json:"action"`
	SQL    string `json:"sql"`
	Count  int    `json:"postsVisible"`
	Note   string `json:"note"`
}

// SQLMigrationsPayload is the wire shape of GET /api/demo/db/migrations
// (camelCase JSON). The counts are deterministic on a seeded database
// (6 seed posts, one scratch insert at a time); the ledger's appliedAt
// stamps vary per environment by design — same migrations, different
// clocks.
type SQLMigrationsPayload struct {
	History []SchemaExplorerHistory `json:"history"`
	Steps   []TxStoryStep           `json:"steps"`
	Summary string                  `json:"summary"`
}

// txStoryNotes explains each step in one sentence — the curated half the
// database cannot know.
var txStoryNotes = map[string]string{
	"begin":                 "a transaction opens: everything until COMMIT or ROLLBACK is one all-or-nothing unit.",
	"insert scratch post":   "the insert is visible to THIS transaction (count includes it) and to nobody else.",
	"savepoint sp1":         "a bookmark inside the transaction: partial work can be undone without throwing the unit away.",
	"insert second post":    "a second scratch post — visible in the count, still uncommitted.",
	"rollback to savepoint": "the second insert is gone; the first survives; the transaction stays open.",
	"rollback":              "the whole unit is discarded — the count outside confirms the database never saw any of it.",
}

// BuildSQLMigrationsPayload shapes the observation into the demo payload:
// numbered, annotated steps in execution order, the goose ledger, and the
// one-sentence moral.
func BuildSQLMigrationsPayload(observation TxStoryObservation) SQLMigrationsPayload {
	steps := make([]TxStoryStep, 0, len(observation.Steps))
	for i, row := range observation.Steps {
		steps = append(steps, TxStoryStep{
			Step:   i + 1,
			Action: row.Action,
			SQL:    row.SQL,
			Count:  row.Count,
			Note:   txStoryNotes[row.Action],
		})
	}
	history := make([]SchemaExplorerHistory, 0, len(observation.History))
	for _, row := range observation.History {
		history = append(history, SchemaExplorerHistory{
			Version:   row.Version,
			Applied:   row.Applied,
			AppliedAt: row.AppliedAt,
		})
	}
	return SQLMigrationsPayload{
		History: history,
		Steps:   steps,
		Summary: fmt.Sprintf("The savepoint unwound one insert and kept the other; the final ROLLBACK unwound " +
			"everything. Atomicity is not a property statements have — it is the property the transaction " +
			"boundary gives them."),
	}
}
