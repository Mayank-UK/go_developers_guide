package obs

import (
	"context"
	"fmt"
	"time"
)

// DefaultDependencyTimeout bounds one readiness ping when a check does
// not name its own bound: readiness answers are load-balancer decisions,
// and a prober that waits on a wedged dependency fails the WHOLE probe
// deadline — the check must be faster than the question.
const DefaultDependencyTimeout = time.Second

// Pinger is the readiness port one dependency satisfies: cheap, side
// effect free, and honest — a nil error means the dependency answered.
// Adapters live in the data layer (pgx pool ping, go-redis ping); the
// domain sees the interface it owns.
type Pinger interface {
	Ping(ctx context.Context) error
}

// DependencyCheck names one dependency, the pinger that proves it, and
// the bound its proof runs under.
type DependencyCheck struct {
	Name    string
	Pinger  Pinger
	Timeout time.Duration
}

// DependencyStatus is one check's outcome: name, up/down, how long the
// proof took, and the failure reason verbatim when down (the reason is
// server-observed, never invented).
type DependencyStatus struct {
	Name    string
	OK      bool
	Latency time.Duration
	Error   string
}

// ReadinessReport is the whole probe: ready only when every dependency
// answered, with each dependency's row carried alongside — a 503 that
// names its cause, not a bare one.
type ReadinessReport struct {
	Ready  bool
	Checks []DependencyStatus
}

// CheckDependencies runs every check SEQUENTIALLY inside its own bounded
// timeout, so one hung dependency consumes at most its own bound and the
// report still names it. A nil Pinger is a wiring bug and reports down
// with a programmer-honest message rather than panicking mid-probe.
func CheckDependencies(ctx context.Context, checks []DependencyCheck) ReadinessReport {
	report := ReadinessReport{Ready: true}
	for _, check := range checks {
		status := checkDependency(ctx, check)
		if !status.OK {
			report.Ready = false
		}
		report.Checks = append(report.Checks, status)
	}
	return report
}

// checkDependency runs one bounded ping and shapes its row.
func checkDependency(ctx context.Context, check DependencyCheck) DependencyStatus {
	if check.Pinger == nil {
		return DependencyStatus{
			Name:  check.Name,
			OK:    false,
			Error: "no pinger wired (composition-root bug)",
		}
	}
	timeout := check.Timeout
	if timeout <= 0 {
		timeout = DefaultDependencyTimeout
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	err := check.Pinger.Ping(pingCtx)
	latency := time.Since(started)
	if err != nil {
		return DependencyStatus{
			Name:    check.Name,
			OK:      false,
			Latency: latency,
			Error:   fmt.Sprintf("%v (bound %s)", err, timeout),
		}
	}
	return DependencyStatus{Name: check.Name, OK: true, Latency: latency}
}
