package presentation

import (
	"net/http"
	"net/http/pprof"
)

// NewInternalHandler serves the guide's INTERNAL observability surface:
// the Go runtime's pprof endpoints, mounted under the dedicated
// /internal/ prefix instead of the public mux root. Why a dedicated
// path: importing net/http/pprof REGISTERS its handlers on
// http.DefaultServeMux as a package side effect — but this server never
// serves the default mux, so those registrations are inert until a mux
// the composition root owns mounts them deliberately, here. The prefix
// gives ops one string to filter ("nothing under /internal/ crosses the
// edge") whether the boundary is a firewall rule, an ingress path, or a
// bind to a loopback-only listener; the profiles inside are the
// runtime's own: index (heap snapshot by default), profile (CPU over N
// seconds), goroutine, block, mutex, cmdline, symbol, and trace.
func NewInternalHandler() http.Handler {
	internal := http.NewServeMux()
	internal.HandleFunc("/debug/pprof/", pprof.Index)
	internal.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	internal.HandleFunc("/debug/pprof/profile", pprof.Profile)
	internal.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	internal.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return http.StripPrefix("/internal", internal)
}
