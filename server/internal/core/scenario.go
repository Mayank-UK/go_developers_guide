package core

import "sync"

// DataMode selects which backing store the demos answer from: fake
// in-memory data, or the live stores (wired from Phase 6 on).
type DataMode string

// The two legal data modes.
const (
	DataModeFake DataMode = "fake"
	DataModeLive DataMode = "live"
)

// Valid reports whether the mode is one of the two legal values.
func (m DataMode) Valid() bool {
	return m == DataModeFake || m == DataModeLive
}

// FailureInjection arms the demos' failure path. The family wire contract
// (fastapi verbatim) is the three-value enum, not a bool: the teaching
// distinguishes error injection (topics 30) from latency injection
// (topic 39), and the client sends these exact strings.
type FailureInjection string

// The three legal failure-injection modes.
const (
	FailureNone    FailureInjection = "none"
	FailureError   FailureInjection = "error"
	FailureLatency FailureInjection = "latency"
)

// Valid reports whether the mode is one of the three legal values.
func (f FailureInjection) Valid() bool {
	return f == FailureNone || f == FailureError || f == FailureLatency
}

// ScenarioState is the immutable snapshot of the demo scenario; the wire
// is camelCase and matches the fastapi guide's /api/demo/scenario exactly:
// {"dataMode":"fake"|"live","injectFailure":"none"|"error"|"latency"}.
// Defaults: fake data, no failure injection.
type ScenarioState struct {
	DataMode      DataMode         `json:"dataMode"`
	InjectFailure FailureInjection `json:"injectFailure"`
}

// ScenarioPatch is the partial-update body for PATCH /api/demo/scenario.
// A nil field means "unchanged" — the wire form relies on JSON's absent
// key, which decodes to a nil pointer, so a present-but-wrong type fails
// decoding (and becomes a 422) rather than silently meaning "unset".
type ScenarioPatch struct {
	DataMode      *DataMode         `json:"dataMode,omitempty"`
	InjectFailure *FailureInjection `json:"injectFailure,omitempty"`
}

// ScenarioStore is the single runtime toggle the demo endpoints read:
// which data mode the demos answer from, and which failure injection is
// armed. It is mutex-guarded (reads and writes) so concurrent demo calls
// and PATCHes race-detector clean; state changes are copy-in/copy-out so
// snapshots handed to callers can never be mutated under them.
type ScenarioStore struct {
	mu      sync.RWMutex
	current ScenarioState
}

// NewScenarioStore builds a store at the documented defaults: fake data,
// failure injection off.
func NewScenarioStore() *ScenarioStore {
	return &ScenarioStore{current: ScenarioState{DataMode: DataModeFake, InjectFailure: FailureNone}}
}

// Snapshot returns the current state.
func (s *ScenarioStore) Snapshot() ScenarioState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

// Update merges only the fields the patch sets, validating the merged
// result. An invalid patch — an unknown data mode or failure-injection
// value — returns a ValidationFailed error and leaves the stored state
// untouched. The store defends its own invariant because patches can also
// arrive from untyped callers, not just the HTTP body decoder.
func (s *ScenarioStore) Update(patch ScenarioPatch) (ScenarioState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if patch.DataMode != nil && !patch.DataMode.Valid() {
		return s.current, ValidationFailed(
			"Invalid scenario patch: unknown data mode.",
			map[string]any{"dataMode": string(*patch.DataMode), "expected": []string{string(DataModeFake), string(DataModeLive)}},
		)
	}
	if patch.InjectFailure != nil && !patch.InjectFailure.Valid() {
		return s.current, ValidationFailed(
			"Invalid scenario patch: unknown failure injection.",
			map[string]any{
				"injectFailure": string(*patch.InjectFailure),
				"expected":      []string{string(FailureNone), string(FailureError), string(FailureLatency)},
			},
		)
	}

	next := s.current
	if patch.DataMode != nil {
		next.DataMode = *patch.DataMode
	}
	if patch.InjectFailure != nil {
		next.InjectFailure = *patch.InjectFailure
	}
	s.current = next
	return next, nil
}
