// Demo API handler for topic 37 deployment — the stack viewer. Thin
// by the family rule: the handler owns exactly the file resolution
// (the deploy/ artifacts directory, resolved by the composition root
// when the repo sits beside the process) and the reads; parsing,
// derivation, and the secret gate belong to the pure builder in
// internal/domain/demos. When the artifacts are not beside the
// process — the distroless prod image ships no repository — the
// endpoint degrades to the typed 503 instead of guessing, the same
// honest-unavailable stance the dependency topic's manifest demo
// holds.
package presentation

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*DeployStackHandler)(nil)

// DeployStackHandler serves GET /api/demo/deploy/stack, the
// deployment topic's demo payload: the parsed topology of the repo's
// real deployment artifacts, with per-artifact provenance.
type DeployStackHandler struct {
	artifactsDir string
	logger       *slog.Logger
}

// NewDeployStackHandler wires the stack viewer over the deployment
// artifacts directory (empty when the repo is not beside the process).
func NewDeployStackHandler(artifactsDir string, logger *slog.Logger) *DeployStackHandler {
	return &DeployStackHandler{artifactsDir: artifactsDir, logger: logger}
}

// ServeHTTP implements http.Handler: read the artifacts, hand the
// facts to the builder, write the payload.
func (h *DeployStackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.artifactsDir == "" {
		core.WriteError(w, r, core.ServiceUnavailable(
			"The deployment story is parsed live from the repo's deploy/ artifacts, which do not sit beside this process — run the guide from the repository (the dev container) to serve it."),
			h.logger)
		return
	}
	facts, err := h.readFacts()
	if err != nil {
		if os.IsNotExist(err) {
			core.WriteError(w, r, core.Wrap(err, core.KindServiceUnavailable,
				"The deployment artifacts are not where this process expected them — deploy/docker-compose.prod.yml and deploy/k8s/ must sit in the resolved directory."),
				h.logger)
			return
		}
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"Reading the deployment artifacts failed."), h.logger)
		return
	}
	payload, err := demos.BuildDeployStackPayload(facts)
	if err != nil {
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"Deriving the deployment stack payload failed."), h.logger)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// readFacts loads the compose file and every k8s manifest from the
// artifacts directory, manifest names relative to it.
func (h *DeployStackHandler) readFacts() (demos.DeployStackFacts, error) {
	compose, err := os.ReadFile(filepath.Join(h.artifactsDir, demos.DeployComposeFile))
	if err != nil {
		return demos.DeployStackFacts{}, err
	}
	entries, err := os.ReadDir(filepath.Join(h.artifactsDir, demos.DeployK8sDir))
	if err != nil {
		return demos.DeployStackFacts{}, err
	}
	facts := demos.DeployStackFacts{ComposeYAML: compose}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(h.artifactsDir, demos.DeployK8sDir, entry.Name()))
		if err != nil {
			return demos.DeployStackFacts{}, err
		}
		facts.Manifests = append(facts.Manifests, demos.DeployManifestFile{
			Name: demos.DeployK8sDir + "/" + entry.Name(),
			JSON: raw,
		})
	}
	if len(facts.Manifests) == 0 {
		return demos.DeployStackFacts{}, os.ErrNotExist
	}
	return facts, nil
}
