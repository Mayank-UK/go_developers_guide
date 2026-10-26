package demos

import (
	"testing"
)

// The audit-in-transaction builder's pins: the six annotated steps, and
// the two verdicts DERIVED from the observed world — committed-together
// when the change and its history are visible outside the transaction,
// rolled-back-together when the doomed unit left the outside world
// exactly where it stood.

func auditStoryObservation() AuditObservation {
	return AuditObservation{Steps: []AuditStepRow{
		{Phase: "commit-open", Action: "BEGIN", LiveTitle: "(no post yet)", AuditRows: 0},
		{Phase: "commit-change", Action: "INSERT post; INSERT audit_log; UPDATE title; INSERT audit_log", LiveTitle: "audit scratch (edited)", AuditRows: 2},
		{Phase: "commit-done", Action: "COMMIT", LiveTitle: "audit scratch (edited)", AuditRows: 2},
		{Phase: "rollback-open", Action: "BEGIN", LiveTitle: "audit scratch (edited)", AuditRows: 2},
		{Phase: "rollback-inside", Action: "UPDATE title; INSERT audit_log", LiveTitle: "audit scratch (doomed)", AuditRows: 3},
		{Phase: "rollback-done", Action: "ROLLBACK", LiveTitle: "audit scratch (edited)", AuditRows: 2},
	}}
}

func TestDBAuditPayloadDerivesBothVerdictsFromTheObservations(t *testing.T) {
	t.Parallel()

	payload := BuildAuditPayload(auditStoryObservation())

	if len(payload.Steps) != 6 {
		t.Fatalf("steps = %d, want the six-step story", len(payload.Steps))
	}
	for _, step := range payload.Steps {
		if step.Note == "" {
			t.Errorf("phase %q lacks its teaching note", step.Phase)
		}
	}
	if !payload.CommittedTogether {
		t.Error("the committed-together verdict must hold when the change and its history are both outside-visible")
	}
	if !payload.RolledBackTogether {
		t.Error("the rolled-back-together verdict must hold when the doomed unit restored the prior world exactly")
	}
	if payload.Summary == "" {
		t.Error("the summary sentence is missing")
	}
}

func TestDBAuditPayloadRefusesVerdictsTheObservationsDoNotSupport(t *testing.T) {
	t.Parallel()

	// The rollback left the audit row behind (the bug the pattern exists
	// to prevent): the verdict must refuse to claim rolled-back-together.
	observation := auditStoryObservation()
	observation.Steps[5].AuditRows = 3
	payload := BuildAuditPayload(observation)
	if payload.RolledBackTogether {
		t.Error("a leaked audit row must fail the rolled-back-together verdict")
	}

	// The commit lost the audit row entirely: the other verdict refuses.
	observation = auditStoryObservation()
	observation.Steps[2].AuditRows = 0
	payload = BuildAuditPayload(observation)
	if payload.CommittedTogether {
		t.Error("a missing audit row must fail the committed-together verdict")
	}
}
