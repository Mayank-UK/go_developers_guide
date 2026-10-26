package demos

import (
	"testing"
)

// The isolation-race builder's pins: every choreographed step carries its
// phase note, the pinned before/after titles, the lock verdict derived
// from the observed phases, and the serializable comparison note.

func TestDBRacePayloadTellsTheOrchestratedStory(t *testing.T) {
	t.Parallel()

	observation := RaceObservation{
		Isolation: "read committed",
		Steps: []RaceStepRow{
			{Phase: "uncommitted-write", Actor: "T1 (writer)", Action: "BEGIN; UPDATE … (uncommitted)", Observed: "title updated inside T1 only"},
			{Phase: "reader-looks", Actor: "T2 (reader)", Action: "SELECT title …", Observed: "race demo scratch"},
			{Phase: "writer-commits", Actor: "T1 (writer)", Action: "COMMIT", Observed: "the new version is THE committed truth"},
			{Phase: "reader-looks-again", Actor: "T2 (reader)", Action: "SELECT title … (again)", Observed: "race demo scratch (updated)"},
			{Phase: "lock-acquired", Actor: "T1 (writer)", Action: "BEGIN; SELECT … FOR UPDATE", Observed: "row lock held"},
			{Phase: "lock-waited", Actor: "T2 (reader)", Action: "SELECT … FOR UPDATE (waits)", Observed: "waited for the lock, then read \"race demo scratch (updated)\""},
		},
	}
	payload := BuildSQLRacePayload(observation)

	if payload.Isolation != "read committed" {
		t.Errorf("isolation = %q, want the database's own answer", payload.Isolation)
	}
	if payload.ScratchBefore != "race demo scratch" || payload.ScratchAfter != "race demo scratch (updated)" {
		t.Errorf("pinned titles drifted: %q -> %q", payload.ScratchBefore, payload.ScratchAfter)
	}
	if len(payload.Steps) != 6 {
		t.Fatalf("steps = %d, want the six-phase choreography", len(payload.Steps))
	}
	for _, step := range payload.Steps {
		if step.Note == "" {
			t.Errorf("phase %q lacks its teaching note", step.Phase)
		}
	}
	if !payload.LockWaited {
		t.Error("the lock-waited phase must set the verdict")
	}
	if payload.SerializableNote == "" || payload.SerializableNote[:5] != "Under" {
		t.Errorf("the serializable comparison note is missing: %q", payload.SerializableNote)
	}
}

func TestDBRacePayloadLockVerdictFollowsThePhases(t *testing.T) {
	t.Parallel()

	payload := BuildSQLRacePayload(RaceObservation{Steps: []RaceStepRow{
		{Phase: "uncommitted-write", Actor: "T1", Action: "UPDATE", Observed: "x"},
	}})
	if payload.LockWaited {
		t.Error("no lock-waited phase means no lock verdict — the payload must not claim one")
	}
}
