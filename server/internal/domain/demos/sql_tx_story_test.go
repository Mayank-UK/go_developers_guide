package demos

import (
	"testing"
)

// The transactions-story builder's pins: numbered annotated steps, the
// savepoint choreography's counts rendered as the story they tell, and
// the goose ledger passed through shaped.

func TestDBMigrationsPayloadNumbersAndAnnotatesTheStory(t *testing.T) {
	t.Parallel()

	observation := TxStoryObservation{
		Steps: []TxStoryRow{
			{Action: "begin", SQL: "BEGIN", Count: 6},
			{Action: "insert scratch post", SQL: "INSERT INTO posts …", Count: 7},
			{Action: "savepoint sp1", SQL: "SAVEPOINT sp1", Count: 7},
			{Action: "insert second post", SQL: "INSERT INTO posts …", Count: 8},
			{Action: "rollback to savepoint", SQL: "ROLLBACK TO SAVEPOINT sp1", Count: 7},
			{Action: "rollback", SQL: "ROLLBACK", Count: 6},
		},
		History: []MigrationRow{
			{Version: 0, Applied: true, AppliedAt: "2026-10-26 21:20:00"},
			{Version: 1, Applied: true, AppliedAt: "2026-10-26 21:20:00"},
		},
	}
	payload := BuildSQLMigrationsPayload(observation)

	if len(payload.Steps) != 6 {
		t.Fatalf("steps = %d, want the six-step dance", len(payload.Steps))
	}
	for i, step := range payload.Steps {
		if step.Step != i+1 {
			t.Errorf("step %d numbered %d", i, step.Step)
		}
		if step.Note == "" {
			t.Errorf("step %q lacks its teaching note", step.Action)
		}
	}
	if payload.Steps[0].Count != 6 || payload.Steps[3].Count != 8 || payload.Steps[4].Count != 7 || payload.Steps[5].Count != 6 {
		t.Errorf("the story's counts drifted: %+v", payload.Steps)
	}
	if len(payload.History) != 2 || payload.History[0].Version != 0 || !payload.History[0].Applied {
		t.Errorf("the goose ledger did not pass through shaped: %+v", payload.History)
	}
	if payload.Summary == "" {
		t.Error("the summary sentence is missing")
	}
}
