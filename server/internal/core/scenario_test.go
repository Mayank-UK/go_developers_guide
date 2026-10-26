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
	if state.InjectFailure {
		t.Error("default failure injection must be off")
	}
}

func TestScenarioUpdateAppliesSetFieldsOnly(t *testing.T) {
	store := NewScenarioStore()

	live := DataModeLive
	state, err := store.Update(ScenarioPatch{DataMode: &live})
	if err != nil {
		t.Fatalf("update dataMode: %v", err)
	}
	if state.DataMode != DataModeLive || state.InjectFailure {
		t.Errorf("after dataMode patch: %+v", state)
	}

	on := true
	state, err = store.Update(ScenarioPatch{InjectFailure: &on})
	if err != nil {
		t.Fatalf("update injectFailure: %v", err)
	}
	if state.DataMode != DataModeLive || !state.InjectFailure {
		t.Errorf("partial patch must leave dataMode untouched: %+v", state)
	}

	// An empty patch is a no-op returning the current state.
	state, err = store.Update(ScenarioPatch{})
	if err != nil {
		t.Fatalf("empty patch: %v", err)
	}
	if state.DataMode != DataModeLive || !state.InjectFailure {
		t.Errorf("empty patch changed state: %+v", state)
	}
}

func TestScenarioUpdateRejectsUnknownDataMode(t *testing.T) {
	store := NewScenarioStore()
	bogus := DataMode("bogus")
	_, err := store.Update(ScenarioPatch{DataMode: &bogus})
	if err == nil {
		t.Fatal("unknown data mode must be rejected")
	}
	if !errors.Is(err, ErrValidationFailed) {
		t.Errorf("error %v must be a ValidationFailed taxonomy error", err)
	}
	var appErr *Error
	if !errors.As(err, &appErr) || appErr.Details == nil {
		t.Error("rejection must carry details naming the bad value")
	}
	// The stored state is untouched by a rejected patch.
	if state := store.Snapshot(); state.DataMode != DataModeFake || state.InjectFailure {
		t.Errorf("rejected patch mutated the store: %+v", state)
	}
}

func TestScenarioStoreIsRaceClean(t *testing.T) {
	store := NewScenarioStore()
	var wg sync.WaitGroup
	live := DataModeLive
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = store.Snapshot()
		}()
		go func() {
			defer wg.Done()
			_, _ = store.Update(ScenarioPatch{DataMode: &live})
		}()
	}
	wg.Wait()
	if state := store.Snapshot(); !state.DataMode.Valid() {
		t.Errorf("concurrent updates left an invalid state: %+v", state)
	}
}

func TestScenarioStateWireShape(t *testing.T) {
	// Snapshot marshals camelCase: {"dataMode":...,"injectFailure":...}.
	state := ScenarioState{DataMode: DataModeFake}
	if got, want := mustMarshal(t, state), `{"dataMode":"fake","injectFailure":false}`; got != want {
		t.Errorf("snapshot wire = %s, want %s", got, want)
	}
}
