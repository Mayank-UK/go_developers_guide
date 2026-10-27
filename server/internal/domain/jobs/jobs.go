// Package jobs is the guide's background-jobs domain area (topic 29): the
// ports and vocabulary for durable work queues — a task specification the
// application mints, an Enqueuer that hands it to a broker, an Inspector
// that reads queue vital signs and the dead-letter archive, and the retry
// policy seam that decides how long a failed attempt waits before the next
// one. Implementations live in internal/data/jobs (asynq over the guide's
// Redis); nothing here imports asynq — the dependency rule keeps this
// package the seam, so a test double or a different broker could sit
// behind the same three interfaces.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// TaskSpec is one unit of background work as the application defines it:
// WHO should run it (Type, the handler's name), WHAT to hand that handler
// (Payload, already-encoded bytes the handler owns the format of), and HOW
// the broker should treat it (Queue, MaxRetry, ProcessIn, Timeout). The
// id is CALLER-MINTED on purpose — it is the dedup and forensics key
// across every retry and every archive of this task, so it must be stable
// across redeliveries, which a broker-assigned id can never be.
type TaskSpec struct {
	// ID is the caller's stable identity for this task. Two live tasks
	// with the same id are a broker error (uniqueness while pending,
	// scheduled, retrying, or archived) — mint per business event.
	ID string
	// Type names the handler: the worker's mux routes on this string.
	Type string
	// Payload is the handler's input, encoded by the caller (JSON in this
	// guide). Opaque to the broker; a small, self-describing payload is
	// the whole point — the queue carries work, not objects.
	Payload []byte
	// Queue selects which weighted queue the task lands in.
	Queue string
	// MaxRetry is how many times a failed attempt may be retried; the
	// first execution plus MaxRetry is the total attempt budget. 0 means
	// one shot — fail straight to the archive.
	MaxRetry int
	// ProcessIn, when positive, delays the first attempt: the task sits
	// in the SCHEDULED state until the deadline passes. Zero means now.
	ProcessIn time.Duration
	// Timeout bounds ONE attempt's handler execution — each retry gets a
	// fresh budget. Zero means unbounded (rely on context cancellation).
	Timeout time.Duration
}

// Validate refuses a spec the broker would reject anyway, with names that
// point at the caller's bug instead of a broker stack: the routing key
// (Type) and the destination (Queue) must be non-blank, and MaxRetry may
// not be negative (the retry budget, not a punishment).
func (s TaskSpec) Validate() error {
	if blank(s.ID) {
		return errors.New("task spec: id must not be blank")
	}
	if blank(s.Type) {
		return fmt.Errorf("task spec %q: type must not be blank", s.ID)
	}
	if blank(s.Queue) {
		return fmt.Errorf("task spec %q: queue must not be blank", s.ID)
	}
	if s.MaxRetry < 0 {
		return fmt.Errorf("task spec %q: max retry %d must not be negative", s.ID, s.MaxRetry)
	}
	return nil
}

// TaskRef is the broker's receipt for an enqueued task: the id the caller
// minted, echoed back, plus where it landed. The ref is what an
// application stores next to its business row when it wants to answer
// "where is that job?" later — the Inspector's whole API starts from
// these values.
type TaskRef struct {
	ID    string
	Queue string
	Type  string
}

// QueueSnapshot is a queue's vital signs at one instant: the full state
// census and the two numbers that page a human. Depth is Pending — work
// waiting, unstarted. OldestPendingAge is the age of the OLDEST pending
// task: the moment either number trends up, the queue is draining slower
// than it fills, and latency (the snapshot's derived field for it) is the
// honest first symptom of a worker outage.
type QueueSnapshot struct {
	Name             string
	Size             int
	Pending          int
	Active           int
	Scheduled        int
	Retry            int
	Archived         int
	Completed        int
	OldestPendingAge time.Duration
	Processed        int64
	Failed           int64
}

// ArchivedTask is one task the retry budget exhausted: the dead-letter
// record an operator reads before deciding to replay or delete. LastErr
// is the final attempt's error text — the whole point of the archive is
// that this string survives the failure it names.
type ArchivedTask struct {
	ID       string
	Type     string
	Queue    string
	Retried  int
	MaxRetry int
	LastErr  string
}

// Enqueuer is the produce-side port: hand the broker a validated TaskSpec,
// get back the TaskRef. Implementations must honor the caller's context —
// an enqueue that outlives its request is a lie about durability (the
// caller may have already told the client "accepted").
type Enqueuer interface {
	// Enqueue validates the spec, persists it to the queue, and answers
	// the ref. A returned error means NOT enqueued — safe to retry the
	// whole enqueue (a caller-minted id makes the retry idempotent).
	Enqueue(ctx context.Context, spec TaskSpec) (TaskRef, error)
}

// Inspector is the operate-side port: the read-and-repair surface over
// queues and their dead-letter archive. This is the API an on-call human
// walks at 3 a.m. — Stats for the two numbers, Archived for what died,
// Replay for the one worth another chance, DeleteTask for the rest.
type Inspector interface {
	// Stats answers a queue's snapshot, or an error naming the queue.
	Stats(ctx context.Context, queue string) (QueueSnapshot, error)
	// Archived lists the queue's dead-letter records, newest first. An
	// empty archive answers an empty slice, not an error.
	Archived(ctx context.Context, queue string) ([]ArchivedTask, error)
	// Replay moves one archived task back to pending — the manual re-run
	// of the dead-letter story. The task's retry counters stay where the
	// failure left them; another failure re-archives without another
	// retry.
	Replay(ctx context.Context, queue, id string) error
	// DeleteTask removes a task from a queue's pending, scheduled, retry,
	// or archived state — the operator's last resort, and this demo's
	// teardown.
	DeleteTask(ctx context.Context, queue, id string) error
}

// RetryDelayFunc decides how long a failed attempt waits before its
// retry: attempt is the 1-based number of the attempt that just failed.
// The asynq adapter adapts this shape onto the broker's own signature
// (which also passes the task); keeping the task out of the domain seam
// leaves the policy unit-testable with two numbers and an error.
type RetryDelayFunc func(attempt int, err error) time.Duration

func blank(value string) bool {
	return len(value) == 0
}
