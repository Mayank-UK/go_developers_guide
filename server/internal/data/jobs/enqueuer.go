// Package jobs is the guide's background-jobs data layer: the asynq v0.26
// adapters behind the domain ports in internal/domain/jobs — an Enqueuer
// over the asynq client, an Inspector over the asynq inspector, the demo
// task handlers behind a ServeMux, the hosted worker (ServerRunner) the
// composition root starts and stops, and the console story that
// choreographs topic 29's live demo. Everything here shares the process's
// single go-redis client (internal/data/redis.NewClient): asynq is built
// on the same Redis the guide already runs, and an unreachable store
// surfaces as ordinary errors the handlers map to the typed 503 — the
// optional-service discipline, never a crash.
package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	goredis "github.com/redis/go-redis/v9"

	"guide/internal/domain/jobs"
)

// Enqueuer adapts the domain's produce port onto the asynq client. The
// TaskSpec's fields map one-for-one onto broker options — the id becomes
// the task's uniqueness key, the queue its destination, MaxRetry its
// budget, ProcessIn its scheduled-state delay, Timeout its per-attempt
// bound — so a spec is the whole broker conversation.
type Enqueuer struct {
	client *asynq.Client
}

// NewEnqueuer wires the adapter over the shared go-redis client.
// Construction is lazy (no connection is made), matching the family's
// boot posture: a down Redis never blocks startup.
func NewEnqueuer(redis *goredis.Client) *Enqueuer {
	return &Enqueuer{client: asynq.NewClientFromRedisClient(redis)}
}

// Enqueue validates the spec, hands it to the broker, and answers the
// ref. A returned error means the task was NOT enqueued — the caller may
// retry the whole enqueue, and the caller-minted id makes that retry
// idempotent (a duplicate id while the original still lives is a broker
// refusal, not a silent double-send).
func (e *Enqueuer) Enqueue(ctx context.Context, spec jobs.TaskSpec) (jobs.TaskRef, error) {
	if err := spec.Validate(); err != nil {
		return jobs.TaskRef{}, fmt.Errorf("jobs enqueuer: %w", err)
	}
	options := []asynq.Option{
		asynq.TaskID(spec.ID),
		asynq.Queue(spec.Queue),
		asynq.MaxRetry(spec.MaxRetry),
	}
	if spec.ProcessIn > 0 {
		options = append(options, asynq.ProcessIn(spec.ProcessIn))
	}
	if spec.Timeout > 0 {
		options = append(options, asynq.Timeout(spec.Timeout))
	}
	task := asynq.NewTask(spec.Type, spec.Payload)
	info, err := e.client.EnqueueContext(ctx, task, options...)
	if err != nil {
		return jobs.TaskRef{}, fmt.Errorf("enqueue task %q to queue %q: %w", spec.ID, spec.Queue, err)
	}
	return jobs.TaskRef{ID: info.ID, Queue: info.Queue, Type: info.Type}, nil
}

// Close releases the asynq client's bookkeeping (the shared Redis pool
// stays open — it belongs to the process, not this adapter).
func (e *Enqueuer) Close() error {
	if err := e.client.Close(); err != nil {
		return fmt.Errorf("close jobs enqueuer: %w", err)
	}
	return nil
}
