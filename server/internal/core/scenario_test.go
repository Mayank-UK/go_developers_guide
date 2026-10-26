package core

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

func TestScenarioStoreDefaults(t *testing.T) {
	store := NewScenarioStore()
	state := store.Snapshot()
	if state.DataMode != DataModeFake {
		t.Errorf("default data mode = %q, want fake", state.DataMode)
	}
	if state.InjectFailure != FailureNone {
		t.Errorf("default failure injection = %q, want none", state.InjectFailure)
	}
}

func TestScenarioUpdateAppliesSetFieldsOnly(t *testing.T) {
	store := NewScenarioStore()

	live := DataModeLive
	state, err := store.Update(ScenarioPatch{DataMode: &live})
	if err != nil {
		t.Fatalf("update dataMode: %v", err)
	}
	if state.DataMode != DataModeLive || state.InjectFailure != FailureNone {
		t.Errorf("after dataMode patch: %+v", state)
	}

	latency := FailureLatency
	state, err = store.Update(ScenarioPatch{InjectFailure: &latency})
	if err != nil {
		t.Fatalf("update injectFailure: %v", err)
	}
	if state.DataMode != DataModeLive || state.InjectFailure != FailureLatency {
		t.Errorf("partial patch must leave dataMode untouched: %+v", state)
	}

	// An empty patch is a no-op returning the current state.
	state, err = store.Update(ScenarioPatch{})
	if err != nil {
		t.Fatalf("empty patch: %v", err)
	}
	if state.DataMode != DataModeLive || state.InjectFailure != FailureLatency {
		t.Errorf("empty patch changed state: %+v", state)
	}
}

func TestScenarioUpdateRejectsUnknownEnumValues(t *testing.T) {
	cases := []struct {
		name  string
		patch ScenarioPatch
	}{
		{
			name:  "unknown data mode",
			patch: ScenarioPatch{DataMode: ptr(DataMode("bogus"))},
		},
		{
			name:  "unknown failure injection",
			patch: ScenarioPatch{InjectFailure: ptr(FailureInjection("bogus"))},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := NewScenarioStore()
			_, err := store.Update(tc.patch)
			if err == nil {
				t.Fatal("unknown enum value must be rejected")
			}
			if !errors.Is(err, ErrValidationFailed) {
				t.Errorf("error %v must be a ValidationFailed taxonomy error", err)
			}
			var appErr *Error
			if !errors.As(err, &appErr) || appErr.Details == nil {
				t.Error("rejection must carry details naming the bad value")
			}
			// The stored state is untouched by a rejected patch.
			if state := store.Snapshot(); state != (ScenarioState{DataMode: DataModeFake, InjectFailure: FailureNone}) {
				t.Errorf("rejected patch mutated the store: %+v", state)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestScenarioStoreIsRaceClean(t *testing.T) {
	store := NewScenarioStore()
	var wg sync.WaitGroup
	live := DataModeLive
	latency := FailureLatency
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = store.Snapshot()
		}()
		go func() {
			defer wg.Done()
			_, _ = store.Update(ScenarioPatch{DataMode: &live, InjectFailure: &latency})
		}()
	}
	wg.Wait()
	state := store.Snapshot()
	if !state.DataMode.Valid() || !state.InjectFailure.Valid() {
		t.Errorf("concurrent updates left an invalid state: %+v", state)
	}
}

func TestScenarioStateWireShape(t *testing.T) {
	// Snapshot marshals the fastapi wire verbatim: camelCase keys, enum
	// string values.
	state := ScenarioState{DataMode: DataModeFake, InjectFailure: FailureNone}
	if got, want := mustMarshal(t, state), `{"dataMode":"fake","injectFailure":"none"}`; got != want {
		t.Errorf("snapshot wire = %s, want %s", got, want)
	}
	state = ScenarioState{DataMode: DataModeLive, InjectFailure: FailureLatency}
	if got, want := mustMarshal(t, state), `{"dataMode":"live","injectFailure":"latency"}`; got != want {
		t.Errorf("snapshot wire = %s, want %s", got, want)
	}
}
