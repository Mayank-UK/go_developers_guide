// Topic 17 databases-sql's isolation demo: two concurrent transactions,
// orchestrated step by step over a scratch row, so the OUTCOME — not a
// race — is what the payload reports. A writer updates without
// committing; a reader on a second connection looks (and under
// PostgreSQL's default READ COMMITTED sees the OLD row, because every
// statement takes its own snapshot); the writer commits; the reader
// looks again and sees the new value; then the lock exhibit: the writer
// holds SELECT … FOR UPDATE and the reader's FOR UPDATE WAITS until the
// writer commits. The orchestration is channel- and poll-driven — no
// sleeps — so the observed sequence is deterministic.
package demos

import (
	"context"
)

// SQLRaceRunner is the port the demo reads through. The pgx-backed
// implementation lives in internal/data/db.
type SQLRaceRunner interface {
	// RunRace choreographs the two-transaction dance on a scratch row and
	// reports every observed step.
	RunRace(ctx context.Context) (RaceObservation, error)
}

// RaceStepRow is one observed step: which transaction acted, what it did,
// and what it saw.
type RaceStepRow struct {
	Phase    string
	Actor    string
	Action   string
	Observed string
}

// RaceObservation is the runner's raw haul plus the instance's isolation
// level, as the database itself reports it.
type RaceObservation struct {
	Isolation string
	Steps     []RaceStepRow
}

// RaceStep is one step as the payload serves it.
type RaceStep struct {
	Phase    string `json:"phase"`
	Actor    string `json:"actor"`
	Action   string `json:"action"`
	Observed string `json:"observed"`
	Note     string `json:"note"`
}

// SQLRacePayload is the wire shape of GET /api/demo/db/race (camelCase
// JSON). Every observed string is pinned by the choreography; the
// scratch row is deleted before the demo returns, so nothing persists.
type SQLRacePayload struct {
	Isolation        string     `json:"isolation"`
	ScratchBefore    string     `json:"scratchBefore"`
	ScratchAfter     string     `json:"scratchAfter"`
	Steps            []RaceStep `json:"steps"`
	LockWaited       bool       `json:"lockWaited"`
	SerializableNote string     `json:"serializableNote"`
}

// raceStepNotes explains each phase in one sentence.
var raceStepNotes = map[string]string{
	"uncommitted-write":  "the writer's UPDATE is done but not committed: it holds the row's new version where only its own transaction can see it.",
	"reader-looks":       "a plain SELECT never blocks and never reads dirty data — under READ COMMITTED each statement takes a fresh snapshot, and the only committed version is the old one.",
	"writer-commits":     "the new version becomes THE committed truth for every future snapshot.",
	"reader-looks-again": "the same SELECT, one statement later, sees the update — nonrepeatable reads within one transaction are READ COMMITTED's documented trade.",
	"lock-acquired":      "SELECT … FOR UPDATE takes the row lock the writer will hold: the honest way to say 'I am about to change this row'.",
	"lock-waited":        "the reader's FOR UPDATE waits — row locks serialize writers — and proceeds the moment the holder commits.",
}

// BuildSQLRacePayload shapes the observation into the demo payload: the
// pinned before/after titles, every step with its note, the lock verdict,
// and what SERIALIZABLE would have changed (taught by comparison, not
// run: under it one of these transactions would abort with a
// serialization failure and the application would retry).
func BuildSQLRacePayload(observation RaceObservation) SQLRacePayload {
	steps := make([]RaceStep, 0, len(observation.Steps))
	lockWaited := false
	for _, row := range observation.Steps {
		if row.Phase == "lock-waited" {
			lockWaited = true
		}
		steps = append(steps, RaceStep{
			Phase:    row.Phase,
			Actor:    row.Actor,
			Action:   row.Action,
			Observed: row.Observed,
			Note:     raceStepNotes[row.Phase],
		})
	}
	return SQLRacePayload{
		Isolation:     observation.Isolation,
		ScratchBefore: "race demo scratch",
		ScratchAfter:  "race demo scratch (updated)",
		Steps:         steps,
		LockWaited:    lockWaited,
		SerializableNote: "Under SERIALIZABLE these two transactions could not both succeed: the database would " +
			"reject one with a serialization failure and the application would retry it — correctness by " +
			"aborted optimism instead of locks held while you think.",
	}
}
