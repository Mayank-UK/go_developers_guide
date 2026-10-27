package obs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// stubPinger is the test double for the readiness port: it answers after
// the configured delay with the configured error.
type stubPinger struct {
	delay time.Duration
	err   error
}

func (p stubPinger) Ping(ctx context.Context) error {
	select {
	case <-time.After(p.delay):
		return p.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestCheckDependenciesReportsReadyWhenAllAnswer(t *testing.T) {
	report := CheckDependencies(context.Background(), []DependencyCheck{
		{Name: "postgres", Pinger: stubPinger{delay: time.Millisecond}},
		{Name: "redis", Pinger: stubPinger{delay: 2 * time.Millisecond}},
	})
	if !report.Ready || len(report.Checks) != 2 {
		t.Fatalf("report = %+v, want ready with two rows", report)
	}
	for _, check := range report.Checks {
		if !check.OK || check.Error != "" || check.Latency <= 0 {
			t.Errorf("healthy row drifted: %+v", check)
		}
	}
}

func TestCheckDependenciesNamesTheDownDependency(t *testing.T) {
	cause := errors.New("connection refused")
	report := CheckDependencies(context.Background(), []DependencyCheck{
		{Name: "postgres", Pinger: stubPinger{delay: time.Millisecond}},
		{Name: "redis", Pinger: stubPinger{err: cause}},
	})
	if report.Ready {
		t.Fatal("one down dependency must fail the whole report")
	}
	if len(report.Checks) != 2 || report.Checks[0].Name != "postgres" || !report.Checks[0].OK {
		t.Fatalf("rows drifted: %+v", report.Checks)
	}
	redis := report.Checks[1]
	if redis.OK || !strings.Contains(redis.Error, "connection refused") {
		t.Errorf("down row must carry the cause verbatim: %+v", redis)
	}
}

func TestCheckDependenciesBoundsEachPing(t *testing.T) {
	started := time.Now()
	report := CheckDependencies(context.Background(), []DependencyCheck{
		{Name: "wedged", Pinger: stubPinger{delay: 30 * time.Second}, Timeout: 25 * time.Millisecond},
	})
	elapsed := time.Since(started)
	if elapsed > 2*time.Second {
		t.Errorf("wedged dependency consumed %v — the bound did not hold", elapsed)
	}
	if report.Ready {
		t.Error("a timed-out dependency must report down")
	}
	if !strings.Contains(report.Checks[0].Error, "bound 25ms") {
		t.Errorf("timeout row must name its bound: %+v", report.Checks[0])
	}
}

func TestCheckDependenciesDefaultsTheBoundAndFlagsMissingPinger(t *testing.T) {
	report := CheckDependencies(context.Background(), []DependencyCheck{
		{Name: "defaulted", Pinger: stubPinger{delay: time.Millisecond}},
	})
	if !report.Ready {
		t.Errorf("default bound must accept a fast ping: %+v", report.Checks)
	}
	wired := CheckDependencies(context.Background(), []DependencyCheck{{Name: "broken"}})
	if wired.Ready || !strings.Contains(wired.Checks[0].Error, "no pinger wired") {
		t.Errorf("nil pinger must report down honestly: %+v", wired.Checks)
	}
}
