package presentation

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"guide/internal/core"
)

// Demo scenario API handlers — thin over the injected ScenarioStore: GET
// reports the current snapshot; PATCH applies a partial update (absent
// keys mean unchanged) and answers with the resulting full state. Invalid
// bodies never reach the store: anything that is not one JSON object with
// known-typed fields becomes the 422 validation_failed envelope.

// maxScenarioBodyBytes bounds the PATCH body: a scenario patch is a couple
// of fields, so anything larger is hostile or broken, not input.
const maxScenarioBodyBytes = 4 << 10

// ScenarioGetHandler serves GET /api/demo/scenario.
type ScenarioGetHandler struct {
	store *core.ScenarioStore
}

// NewScenarioGetHandler wires the app's scenario store.
func NewScenarioGetHandler(store *core.ScenarioStore) *ScenarioGetHandler {
	return &ScenarioGetHandler{store: store}
}

// ServeHTTP implements http.Handler for the scenario snapshot.
func (h *ScenarioGetHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.store.Snapshot())
}

// ScenarioPatchHandler serves PATCH /api/demo/scenario.
type ScenarioPatchHandler struct {
	store  *core.ScenarioStore
	logger *slog.Logger
}

// NewScenarioPatchHandler wires the scenario store and the process logger.
func NewScenarioPatchHandler(store *core.ScenarioStore, logger *slog.Logger) *ScenarioPatchHandler {
	return &ScenarioPatchHandler{store: store, logger: logger}
}

// ServeHTTP implements http.Handler for the scenario update. The body must
// be a single JSON object; unknown keys are ignored, but a null body,
// trailing garbage, a body over the size bound, or a field of the wrong
// type is rejected as validation_failed before the store is touched.
func (h *ScenarioPatchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	patch, err := decodeScenarioPatch(w, r)
	if err != nil {
		core.WriteError(w, r, err, h.logger)
		return
	}
	state, err := h.store.Update(patch)
	if err != nil {
		core.WriteError(w, r, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// decodeScenarioPatch reads and shape-checks the PATCH body, returning the
// typed patch or a ValidationFailed error carrying what went wrong.
func decodeScenarioPatch(w http.ResponseWriter, r *http.Request) (core.ScenarioPatch, error) {
	invalid := func(reason string) (core.ScenarioPatch, error) {
		return core.ScenarioPatch{}, core.ValidationFailed(
			"Invalid scenario patch body.", map[string]string{"body": reason},
		)
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxScenarioBodyBytes))
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return invalid(err.Error())
	}
	// Exactly one JSON value: trailing content (a second object, stray
	// tokens) means the body is not one patch document.
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return invalid("body must contain exactly one JSON object")
	}
	// JSON null decodes into any struct without error, but a patch must be
	// an object — pydantic parity with the family server.
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return invalid("body must be a JSON object, not null")
	}

	var patch core.ScenarioPatch
	if err := json.Unmarshal(raw, &patch); err != nil {
		return invalid(err.Error())
	}
	return patch, nil
}
