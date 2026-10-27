// Demo API handlers for topic 25 security — the family's thin-handler
// invariant over the two story ports. The stories run the REAL engines
// (a fresh limiter per lab run; a fresh subject store per PII run), so
// these handlers hold only their runner port and a logger: run the
// story, shape the payload, write the JSON. The substrate is
// process-local, so a story failure is an honest internal error, never
// a fabricated answer.
package presentation

import (
	"log/slog"
	"net/http"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// Compile-time proofs the two handlers satisfy http.Handler.
var (
	_ http.Handler = (*SecurityLabHandler)(nil)
	_ http.Handler = (*SecurityPIIHandler)(nil)
)

// securityStoryHandler is the shared body both story endpoints serve:
// one runner call, one payload build, one write — with the story's own
// error mapped to the internal envelope.
func securityStoryHandler[Run any](w http.ResponseWriter, r *http.Request, logger *slog.Logger,
	run func() (Run, error), build func(Run) demos.SecurityStoryPayload, what string) {
	result, err := run()
	if err != nil {
		core.WriteError(w, r, core.Wrap(err, core.KindInternal,
			"The "+what+" demo could not run its story against the security console."), logger)
		return
	}
	writeJSON(w, http.StatusOK, build(result))
}

// SecurityLabHandler serves GET /api/demo/security/lab.
type SecurityLabHandler struct {
	runner demos.SecurityLabRunner
	logger *slog.Logger
}

// NewSecurityLabHandler wires the lab story endpoint.
func NewSecurityLabHandler(runner demos.SecurityLabRunner, logger *slog.Logger) *SecurityLabHandler {
	return &SecurityLabHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the lab story.
func (h *SecurityLabHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	securityStoryHandler(w, r, h.logger, func() (demos.SecurityLabRun, error) {
		return h.runner.RunSecurityLabStory(r.Context())
	}, demos.BuildSecurityLabPayload, "security-lab")
}

// SecurityPIIHandler serves GET /api/demo/security/pii.
type SecurityPIIHandler struct {
	runner demos.SecurityPIIRunner
	logger *slog.Logger
}

// NewSecurityPIIHandler wires the PII story endpoint.
func NewSecurityPIIHandler(runner demos.SecurityPIIRunner, logger *slog.Logger) *SecurityPIIHandler {
	return &SecurityPIIHandler{runner: runner, logger: logger}
}

// ServeHTTP implements http.Handler for the PII story.
func (h *SecurityPIIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	securityStoryHandler(w, r, h.logger, func() (demos.SecurityPIIRun, error) {
		return h.runner.RunSecurityPIIStory(r.Context())
	}, demos.BuildSecurityPIIPayload, "security-pii")
}
