// Topic 17 databases-sql's audit-in-transaction demo: the topic 16
// pattern made real. Phase one COMMITS a scratch post and its audit row
// together — both readable afterwards, neither ever alone. Phase two
// starts a second change with its own audit row and ROLLS BACK — and the
// proof is the payload's own observation: the title is unchanged AND the
// audit row count did not grow, because the audit row rode the same
// transaction out of existence. The demo cleans up after itself in a
// final transaction (a demo affordance production audit trails do not
// grant themselves).
package demos

import (
	"context"
)

// SQLAuditRunner is the port the demo reads through. The pgx-backed
// implementation lives in internal/data/db.
type SQLAuditRunner interface {
	// RunAuditStory choreographs the commit-together / roll-back-together
	// phases on a scratch post and reports every observation.
	RunAuditStory(ctx context.Context) (AuditObservation, error)
}

// AuditStepRow is one observed moment: the phase, what was done, and the
// two facts the pattern's correctness hangs on — the post's live title
// and the audit rows recorded for it at that moment.
type AuditStepRow struct {
	Phase     string
	Action    string
	LiveTitle string
	AuditRows int
}

// AuditObservation is the runner's raw haul.
type AuditObservation struct {
	Steps []AuditStepRow
}

// AuditStep is one step as the payload serves it.
type AuditStep struct {
	Phase     string `json:"phase"`
	Action    string `json:"action"`
	LiveTitle string `json:"liveTitle"`
	AuditRows int    `json:"auditRows"`
	Note      string `json:"note"`
}

// SQLAuditPayload is the wire shape of GET /api/demo/db/audit (camelCase
// JSON). Titles and counts are pinned by the choreography; the scratch
// rows are removed before the demo returns.
type SQLAuditPayload struct {
	Steps              []AuditStep `json:"steps"`
	CommittedTogether  bool        `json:"committedTogether"`
	RolledBackTogether bool        `json:"rolledBackTogether"`
	Summary            string      `json:"summary"`
}

// auditStepNotes explains each step in one sentence.
var auditStepNotes = map[string]string{
	"commit-open":     "a second transaction opens to change the just-created post — and to record that change.",
	"commit-change":   "inside the transaction the new title IS live and the audit row IS recorded; no other transaction can see either.",
	"commit-done":     "COMMIT makes both facts true for everyone at the same instant — the post changed AND the history says so.",
	"rollback-open":   "the rollback phase replays the same two writes: change the title, append the audit row.",
	"rollback-inside": "inside the transaction both are visible again — atomicity is invisible until the boundary decides.",
	"rollback-done":   "ROLLBACK discards BOTH writes together: the title is unchanged and the audit row never existed — the log cannot tell a story that did not happen.",
}

// BuildAuditPayload shapes the observation into the demo payload: the
// steps with their notes and the two verdicts the story exists to earn.
// The verdicts are DERIVED from the observations, never asserted: the
// commit verdict requires the changed post AND its recorded history to
// be visible outside the transaction; the rollback verdict requires the
// outside world to be exactly where it stood before the doomed unit
// opened — title unchanged, audit count unchanged.
func BuildAuditPayload(observation AuditObservation) SQLAuditPayload {
	steps := make([]AuditStep, 0, len(observation.Steps))
	for _, row := range observation.Steps {
		steps = append(steps, AuditStep{
			Phase:     row.Phase,
			Action:    row.Action,
			LiveTitle: row.LiveTitle,
			AuditRows: row.AuditRows,
			Note:      auditStepNotes[row.Phase],
		})
	}

	var afterCommit, beforeDoomed, afterRollback *AuditStepRow
	for i := range observation.Steps {
		row := observation.Steps[i]
		switch row.Phase {
		case "commit-done":
			afterCommit = &observation.Steps[i]
		case "rollback-open":
			beforeDoomed = &observation.Steps[i]
		case "rollback-done":
			afterRollback = &observation.Steps[i]
		}
	}
	committedTogether := afterCommit != nil &&
		afterCommit.LiveTitle == "audit scratch (edited)" && afterCommit.AuditRows == 2
	rolledBackTogether := beforeDoomed != nil && afterRollback != nil &&
		afterRollback.LiveTitle == beforeDoomed.LiveTitle && afterRollback.AuditRows == beforeDoomed.AuditRows

	return SQLAuditPayload{
		Steps:              steps,
		CommittedTogether:  committedTogether,
		RolledBackTogether: rolledBackTogether,
		Summary: "Same transaction, both writes: the audit rows and the change they describe commit or roll " +
			"back TOGETHER — which is why the audit append must ride inside the changing transaction, " +
			"never after it.",
	}
}
