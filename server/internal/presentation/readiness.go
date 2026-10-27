package presentation

import (
	"log/slog"
	"net/http"
	"strings"

	"guide/internal/core"
	"guide/internal/domain/obs"
)

// Compile-time proof the handler satisfies http.Handler.
var _ http.Handler = (*ReadinessHandler)(nil)

// readinessCheckRow is one dependency's wire row — shared by the ready
// body and the not-ready envelope's details, so the two answers differ
// only in envelope, never in facts.
type readinessCheckRow struct {
	Name      string  `json:"name"`
	OK        bool    `json:"ok"`
	LatencyMs float64 `json:"latencyMs"`
	Error     string  `json:"error,omitempty"`
}

// readinessBody is the 200 answer: {"status":"ready","checks":[…]}.
type readinessBody struct {
	Status string              `json:"status"`
	Checks []readinessCheckRow `json:"checks"`
}

// readinessDetails rides the standard error envelope's details slot on
// the 503, so a load balancer gets the honest "why" without a second
// endpoint shape.
type readinessDetails struct {
	Checks []readinessCheckRow `json:"checks"`
}

// ReadinessHandler serves GET /api/ready — the readiness probe the
// health endpoint deliberately is not: it answers "are this process's
// DEPENDENCIES answering", by pinging each injected dependency inside
// its own bounded timeout. Ready answers 200 with every dependency's
// row; not-ready answers the standard 503 service_unavailable envelope
// naming the down dependencies in the message and carrying every row in
// details — an honest degradation, never a bare 503 and never a lie.
type ReadinessHandler struct {
	checks []obs.DependencyCheck
	logger *slog.Logger
}

// NewReadinessHandler wires the probe over the dependency checks the
// composition root built (pingers over the process's real pool and
// Redis client).
func NewReadinessHandler(checks []obs.DependencyCheck, logger *slog.Logger) *ReadinessHandler {
	return &ReadinessHandler{checks: checks, logger: logger}
}

// ServeHTTP implements http.Handler: probe, then map.
func (h *ReadinessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	report := obs.CheckDependencies(r.Context(), h.checks)
	rows := make([]readinessCheckRow, 0, len(report.Checks))
	var down []string
	for _, check := range report.Checks {
		rows = append(rows, readinessCheckRow{
			Name:      check.Name,
			OK:        check.OK,
			LatencyMs: float64(check.Latency.Microseconds()) / 1000,
			Error:     check.Error,
		})
		if !check.OK {
			down = append(down, check.Name)
		}
	}
	if report.Ready {
		writeJSON(w, http.StatusOK, readinessBody{Status: "ready", Checks: rows})
		return
	}
	err := core.New(core.KindServiceUnavailable, "Service not ready: "+strings.Join(down, ", ")+" unavailable.")
	err.Details = readinessDetails{Checks: rows}
	core.WriteError(w, r, err, h.logger)
}
