package demos

import (
	"strings"
	"testing"
	"time"

	"guide/internal/core"
)

// The errors explorer payload builder, pinned to the family rules:
// every table row's status and default message derive from core's own
// tables (never restated), the walk notes derive from observed facts
// (including the honest default when mode and status disagree), and the
// probe answer mirrors the scenario it served.

func TestErrorsExplorerKindsMatchTheCoreTaxonomy(t *testing.T) {
	payload := BuildErrorsExplorerPayload(ErrorsWalk{
		InjectFailure: core.FailureNone,
		Status:        200,
		Elapsed:       2 * time.Millisecond,
		RequestID:     "explorer-test-id",
		Body:          `{"status":"healthy","injectFailure":"none","detail":"…"}`,
	})

	wantOrder := []string{
		"validation_failed", "unauthorized", "forbidden", "not_found",
		"conflict", "rate_limited", "service_unavailable", "internal",
	}
	if len(payload.Kinds) != len(wantOrder) {
		t.Fatalf("kinds = %d rows, want %d", len(payload.Kinds), len(wantOrder))
	}
	for i, row := range payload.Kinds {
		if row.Kind != wantOrder[i] {
			t.Errorf("kinds[%d] = %q, want %q (declared taxonomy order)", i, row.Kind, wantOrder[i])
		}
		kind := core.Kind(row.Kind)
		if row.HTTPStatus != core.HTTPStatusFor(kind) {
			t.Errorf("kind %q status = %d, want core.HTTPStatusFor = %d", row.Kind, row.HTTPStatus, core.HTTPStatusFor(kind))
		}
		if row.DefaultMessage == "" || row.RaisedBy == "" || row.LoggedAs == "" {
			t.Errorf("kind %q row incomplete: %+v", row.Kind, row)
		}
		if row.Trigger.Mode != ErrorsTriggerLive && row.Trigger.Mode != ErrorsTriggerRecipe {
			t.Errorf("kind %q trigger mode = %q, want live or recipe", row.Kind, row.Trigger.Mode)
		}
		if row.Trigger.Method == "" || row.Trigger.Path == "" || row.Trigger.Note == "" {
			t.Errorf("kind %q trigger incomplete: %+v", row.Kind, row.Trigger)
		}
	}
}

// TestErrorsExplorerDefaultMessagesComeFromTheSentinels pins the
// single-source rule: the table's message column IS the sentinel's
// message, so the two can never drift.
func TestErrorsExplorerDefaultMessagesComeFromTheSentinels(t *testing.T) {
	sentinels := map[string]*core.Error{
		"validation_failed":   core.ErrValidationFailed,
		"unauthorized":        core.ErrUnauthorized,
		"forbidden":           core.ErrForbidden,
		"not_found":           core.ErrNotFound,
		"conflict":            core.ErrConflict,
		"rate_limited":        core.ErrRateLimited,
		"service_unavailable": core.ErrServiceUnavailable,
		"internal":            core.ErrInternal,
	}
	payload := BuildErrorsExplorerPayload(ErrorsWalk{InjectFailure: core.FailureNone, Status: 200})
	for _, row := range payload.Kinds {
		if row.DefaultMessage != sentinels[row.Kind].Message {
			t.Errorf("kind %q message = %q, want the sentinel's %q", row.Kind, row.DefaultMessage, sentinels[row.Kind].Message)
		}
	}
}

// TestErrorsExplorerRecipesSayWhy pins the honesty rule: a recipe row
// must explain in plain words why its trigger needs conditions the
// running stack does not have.
func TestErrorsExplorerRecipesSayWhy(t *testing.T) {
	payload := BuildErrorsExplorerPayload(ErrorsWalk{InjectFailure: core.FailureNone, Status: 200})
	recipes := 0
	for _, row := range payload.Kinds {
		if row.Trigger.Mode != ErrorsTriggerRecipe {
			continue
		}
		recipes++
		if !strings.Contains(row.Trigger.Note, "no live trigger") && !strings.Contains(row.Trigger.Note, "by design") &&
			!strings.Contains(row.Trigger.Note, "honestly is not") {
			t.Errorf("recipe row %q must say why no live trigger exists: %q", row.Kind, row.Trigger.Note)
		}
	}
	if recipes != 3 {
		t.Errorf("recipe rows = %d, want exactly 3 (forbidden, conflict, service_unavailable) — the rest must be curl-able", recipes)
	}
}

func TestDescribeErrorsWalkDerivesFromObservedFacts(t *testing.T) {
	cases := []struct {
		name  string
		walk  ErrorsWalk
		want  string
		wantM string
	}{
		{
			name: "healthy walk",
			walk: ErrorsWalk{InjectFailure: core.FailureNone, Status: 200, Elapsed: 3 * time.Millisecond, RequestID: "rid-1"},
			want: "nothing injected: the probe answered 200 healthy in 3 ms",
		},
		{
			name:  "recovered panic walk",
			walk:  ErrorsWalk{InjectFailure: core.FailureError, Status: 500, Elapsed: time.Millisecond, RequestID: "rid-2"},
			want:  "error injection honored: the probe panicked",
			wantM: "rid-2",
		},
		{
			name: "latency walk",
			walk: ErrorsWalk{InjectFailure: core.FailureLatency, Status: 200, Elapsed: 1502 * time.Millisecond, RequestID: "rid-3"},
			want: "latency injection honored: the probe slept out its 1500 ms injection",
		},
		{
			name:  "mode and status disagree — the honest default",
			walk:  ErrorsWalk{InjectFailure: core.FailureError, Status: 200, Elapsed: time.Millisecond, RequestID: "rid-4"},
			want:  "never what the mode promised",
			wantM: "mode=error status=200",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			note := DescribeErrorsWalk(tc.walk)
			if !strings.Contains(note, tc.want) {
				t.Errorf("note %q does not carry %q", note, tc.want)
			}
			if tc.wantM != "" && !strings.Contains(note, tc.wantM) {
				t.Errorf("note %q does not carry %q", note, tc.wantM)
			}
		})
	}
}

func TestBuildErrorsProbeAnswerMirrorsTheScenario(t *testing.T) {
	answer := BuildErrorsProbeAnswer(core.ScenarioState{
		DataMode:      core.DataModeFake,
		InjectFailure: core.FailureLatency,
	})
	if answer.Status != "healthy" {
		t.Errorf("status = %q, want healthy", answer.Status)
	}
	if answer.InjectFailure != "latency" {
		t.Errorf("injectFailure = %q, want latency (the mode the probe served)", answer.InjectFailure)
	}
	if !strings.Contains(answer.Detail, "PATCH /api/demo/scenario") {
		t.Errorf("detail must point at the scenario toggle: %q", answer.Detail)
	}
}
