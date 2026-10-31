package presentation

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewPrometheusHandler serves GET /api/metrics: the client_golang scrape
// endpoint for the VM monitoring rollout, carrying the DEFAULT collectors
// only — the Go runtime collector (go_goroutines, go_info, the gc and
// memory families) and the process collector (process_cpu_seconds_total,
// process_resident_memory_bytes) — rendered by promhttp in the Prometheus
// text exposition format, no custom business families on top. The
// registry is DEDICATED rather than prometheus.DefaultRegisterer: the
// package-level registry is global state, and a second constructor call
// registering the same collectors on it panics ("duplicate metrics
// collector registration") — a local registry keeps every instance
// self-contained and lets tests construct the handler freely.
//
// Why /api/metrics and not the Prometheus-conventional /metrics:
// /metrics already serves topic 33's hand-rolled stdlib exposition of the
// guide's own HTTP families, and that endpoint's composition-root wiring
// is quoted teaching material — it stays exactly as taught. /api/* is
// this server's operational namespace (/api/health, /api/ready), so the
// client_golang scrape rides the same prefix and the taught surface stays
// untouched; a Prometheus scrape config sets metrics_path: /api/metrics.
func NewPrometheusHandler() http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}
