package presentation

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The jobs console demo endpoint, exercised through the same middleware
// chain production runs behind (the family rule that every new demo
// endpoint ships with a Go test). The runner port is faked here — the
// asynq-backed story runner has its own tests against the running stack
// Redis in internal/data/jobs — so the happy path and BOTH honest 503
// degradations (broker unreachable, worker not draining) are pinned
// without a store.

// fakeJobsConsoleRunner stands in for the story runner: it returns a
// canned run shaped like a real walk, or the failure its field carries.
type fakeJobsConsoleRunner struct {
	run demos.JobsConsoleRun
	err error
}

func (f fakeJobsConsoleRunner) RunConsoleStory(_ context.Context, _ string) (demos.JobsConsoleRun, error) {
	return f.run, f.err
}

// jobsConsoleFixtureRun mirrors a real console story closely enough to
// pin the handler's mapping (the domain builder's own tests derive
// every proof from exactly this shape).
func jobsConsoleFixtureRun() demos.JobsConsoleRun {
	ns := "gdg:demo:jobs:0123456789ab"
	return demos.JobsConsoleRun{
		Namespace: ns,
		Queues: []demos.JobsQueueRow{
			{Name: demos.JobsDemoQueue, Weight: 6},
			{Name: demos.JobsDemoLowQueue, Weight: 1},
		},
		FlakyID:  ns + ":flaky",
		PoisonID: ns + ":poison",
		DigestID: ns + ":digest",
		CronSpec: demos.JobsDemoCronSpec,
		Events: []demos.JobsDemoEvent{
			{TaskID: ns + ":flaky", Type: demos.JobsTaskFlaky, Attempt: 1, Outcome: demos.JobsOutcomeFailed, At: time.Unix(1000, 0)},
			{TaskID: ns + ":poison", Type: demos.JobsTaskPoison, Attempt: 1, Outcome: demos.JobsOutcomeFailed, At: time.Unix(1000, 0)},
			{TaskID: ns + ":flaky", Type: demos.JobsTaskFlaky, Attempt: 2, Outcome: demos.JobsOutcomeFailed, At: time.Unix(1002, 0)},
			{TaskID: ns + ":digest", Type: demos.JobsTaskDigest, Attempt: 1, Outcome: demos.JobsOutcomeSucceeded, At: time.Unix(1003, 0)},
			{TaskID: ns + ":flaky", Type: demos.JobsTaskFlaky, Attempt: 3, Outcome: demos.JobsOutcomeSucceeded, At: time.Unix(1004, 0)},
			{TaskID: ns + ":poison", Type: demos.JobsTaskPoison, Attempt: 2, Outcome: demos.JobsOutcomeFailed, At: time.Unix(1005, 0)},
			{TaskID: ns + ":poison", Type: demos.JobsTaskPoison, Attempt: 2, Outcome: demos.JobsOutcomeFailed, At: time.Unix(1007, 0)},
		},
		CronTicks:        2,
		PoisonArchived:   true,
		PoisonLastErr:    "poison task: upstream answered 500 for attempt 2 — retrying cannot fix this",
		PoisonAttempts:   2,
		ReplayAttempts:   1,
		PoisonReArchived: true,
		PoisonDeleted:    true,
		SchedulerStopped: true,
		LedgerDeleted:    true,
		PeakDepth:        2,
		PeakOldestAge:    640 * time.Millisecond,
		FinalDepth:       0,
		FinalOldestAge:   0,
		Elapsed:          5 * time.Second,
	}
}

func newJobsDemoServer(t *testing.T) http.Handler {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/jobs/console",
		NewJobsConsoleHandler(fakeJobsConsoleRunner{run: jobsConsoleFixtureRun()}, logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestJobsConsoleDemoServesTheShapedRun(t *testing.T) {
	server := newJobsDemoServer(t)

	rec := doRequest(server, http.MethodGet, "/api/demo/jobs/console", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q, want application/json", ct)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	decoded := struct {
		Namespace string `json:"namespace"`
		Queues    []struct {
			Name   string `json:"name"`
			Weight int    `json:"weight"`
		} `json:"queues"`
		Tasks []struct {
			ID       string `json:"id"`
			Attempts []struct {
				Attempt int    `json:"attempt"`
				Outcome string `json:"outcome"`
			} `json:"attempts"`
		} `json:"tasks"`
		Beats []struct {
			Beat string `json:"beat"`
		} `json:"beats"`
		Observability struct {
			PeakDepth  int `json:"peakDepth"`
			FinalDepth int `json:"finalDepth"`
		} `json:"observability"`
		Proofs struct {
			Retry   string `json:"retry"`
			Archive string `json:"archive"`
			Replay  string `json:"replay"`
		} `json:"proofs"`
		Reasoning string `json:"reasoning"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if !strings.HasPrefix(decoded.Namespace, demos.JobsDemoKeyPrefix+":") {
		t.Errorf("namespace = %q, want under the jobs prefix (minted per request)", decoded.Namespace)
	}
	if len(decoded.Queues) != 2 || decoded.Queues[1].Weight != 1 {
		t.Errorf("queues = %+v, want the weighted pair", decoded.Queues)
	}
	if len(decoded.Tasks) != 3 || len(decoded.Tasks[0].Attempts) != 3 {
		t.Errorf("tasks = %+v, want three tasks with the flaky ladder", decoded.Tasks)
	}
	if len(decoded.Beats) != 8 {
		t.Errorf("beats = %d, want 8", len(decoded.Beats))
	}
	if !strings.Contains(decoded.Proofs.Retry, "succeeded on attempt 3") {
		t.Errorf("retry proof must derive the ladder: %q", decoded.Proofs.Retry)
	}
	if !strings.Contains(decoded.Proofs.Replay, "grew by 1 attempt") {
		t.Errorf("replay proof must derive the re-execution: %q", decoded.Proofs.Replay)
	}
	if decoded.Observability.FinalDepth != 0 {
		t.Errorf("final depth = %d, want 0 at rest", decoded.Observability.FinalDepth)
	}
}

// TestJobsConsoleDemoDegradesTo503WithTheRightMessage pins BOTH honest
// degradations: an unreachable broker and a reachable one nobody is
// draining are different outages, and each gets its own envelope text.
func TestJobsConsoleDemoDegradesTo503WithTheRightMessage(t *testing.T) {
	logger := slog.Default()
	cases := []struct {
		name   string
		err    error
		wantIn string
	}{
		{
			name:   "broker unreachable",
			err:    errors.New("dial tcp: connection refused"),
			wantIn: "not reachable right now",
		},
		{
			name:   "worker not draining",
			err:    demos.ErrJobsWorkerNotDraining,
			wantIn: "hosted worker is not running",
		},
	}
	for _, tc := range cases {
		mux := http.NewServeMux()
		mux.Handle("GET /api/demo/jobs/console",
			NewJobsConsoleHandler(fakeJobsConsoleRunner{err: tc.err}, logger))
		server := Chain(RequestID, RequestLogger(logger), Recover(logger), EnvelopeMuxErrors(logger))(mux)

		rec := doRequest(server, http.MethodGet, "/api/demo/jobs/console", "")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: status = %d, want 503", tc.name, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `"code":"service_unavailable"`) {
			t.Errorf("%s: 503 body is not the honest envelope: %s", tc.name, body)
		}
		if !strings.Contains(body, tc.wantIn) {
			t.Errorf("%s: 503 body must say what is wrong in plain words (want %q): %s", tc.name, tc.wantIn, body)
		}
		if rid := rec.Header().Get(core.RequestIDHeader); rid == "" {
			t.Errorf("%s: 503 must still carry x-request-id", tc.name)
		}
	}
}

func TestJobsConsoleDemoRejectsNonGet(t *testing.T) {
	server := newJobsDemoServer(t)

	rec := doRequest(server, http.MethodPost, "/api/demo/jobs/console", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow header lost: %q", allow)
	}
}
