package demos

import (
	"strings"
	"testing"
)

// The sql-safety builder's pins: the parameterized outcomes ride beside
// the never-executed concatenation exhibit, the assembled strings show
// exactly what concatenation WOULD have sent, and the verdicts never
// move.

func TestDBSQLSafetyPayloadPairsEveryAttackWithItsExhibit(t *testing.T) {
	t.Parallel()

	observation := SQLSafetyObservation{Attempts: []SQLSafetyAttempt{
		{Input: "x' OR '1'='1", SQL: "SELECT count(*) FROM posts WHERE title = $1", RowsMatched: 0, PostsTotal: 6},
		{Input: "'; DROP TABLE posts; --", SQL: "SELECT count(*) FROM posts WHERE title = $1", RowsMatched: 0, PostsTotal: 6},
	}}
	payload := BuildSQLSafetyPayload(observation)

	if len(payload.Attacks) != 2 {
		t.Fatalf("attacks = %d, want the canonical two", len(payload.Attacks))
	}
	first := payload.Attacks[0]
	if first.Input != "x' OR '1'='1" || first.RowsMatched != 0 || first.PostsAfter != 6 {
		t.Errorf("first attack drifted: %+v", first)
	}
	if first.ParameterizedSQL != "SELECT count(*) FROM posts WHERE title = $1" {
		t.Errorf("the displayed SQL must be the executed SQL, got %q", first.ParameterizedSQL)
	}
	if !strings.Contains(first.AssembledSQL, "OR '1'='1") {
		t.Errorf("the exhibit must show the injected OR, got %q", first.AssembledSQL)
	}
	if !strings.Contains(payload.Attacks[1].AssembledSQL, "DROP TABLE posts") {
		t.Errorf("the second exhibit must show the DROP payload, got %q", payload.Attacks[1].AssembledSQL)
	}
	for _, attack := range payload.Attacks {
		if attack.Verdict != "inert — bound as data, compared as data" {
			t.Errorf("attack %q verdict = %q", attack.Input, attack.Verdict)
		}
		if attack.AssembledEffect == "" {
			t.Errorf("attack %q lacks its concatenated-effect line", attack.Input)
		}
	}
	if payload.Summary == "" {
		t.Error("the summary sentence is missing")
	}
}

func TestDBSQLSafetyAttackInputsMatchTheExhibitOrder(t *testing.T) {
	t.Parallel()

	inputs := SQLSafetyAttackInputs()
	if len(inputs) != 2 || inputs[0] != "x' OR '1'='1" || inputs[1] != "'; DROP TABLE posts; --" {
		t.Errorf("attack inputs = %q, want the canonical pair in exhibit order", inputs)
	}
}

func TestDBSQLSafetyBuilderToleratesShortObservations(t *testing.T) {
	t.Parallel()

	// A runner that somehow answered fewer attempts than attacks must not
	// panic the demo — the row renders with the honest sentinel count.
	payload := BuildSQLSafetyPayload(SQLSafetyObservation{})
	if len(payload.Attacks) != 2 {
		t.Fatalf("attacks = %d, want every exhibit row regardless", len(payload.Attacks))
	}
	if payload.Attacks[0].PostsAfter != -1 || payload.Attacks[0].ParameterizedSQL != "" {
		t.Errorf("missing observation did not render its sentinel: %+v", payload.Attacks[0])
	}
}
