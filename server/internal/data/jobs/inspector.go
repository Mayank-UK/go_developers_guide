package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	goredis "github.com/redis/go-redis/v9"

	"guide/internal/domain/jobs"
)

// Compile-time proof the adapter satisfies the domain's operate port.
var _ jobs.Inspector = (*Inspector)(nil)

// Inspector adapts the domain's operate port onto the asynq inspector:
// queue vital signs (the two numbers), the dead-letter archive's
// contents, the replay that re-runs one archived task, and the delete
// that removes it. The archived list maps ErrTaskNotFound and
// ErrQueueNotFound into the same wrap-and-name posture every adapter in
// this guide uses.
type Inspector struct {
	inspector *asynq.Inspector
}

// NewInspector wires the adapter over the shared go-redis client.
func NewInspector(redis *goredis.Client) *Inspector {
	return &Inspector{inspector: asynq.NewInspectorFromRedisClient(redis)}
}

// Stats answers a queue's snapshot. Latency — the age of the oldest
// pending task — IS the "oldest-job age" number: a healthy queue answers
// single-digit milliseconds; a worker outage lets it grow unboundedly
// while the depth grows beside it.
func (i *Inspector) Stats(ctx context.Context, queue string) (jobs.QueueSnapshot, error) {
	info, err := i.inspector.GetQueueInfo(queue)
	if err != nil {
		return jobs.QueueSnapshot{}, fmt.Errorf("stats for queue %q: %w", queue, err)
	}
	return jobs.QueueSnapshot{
		Name:             info.Queue,
		Size:             info.Size,
		Pending:          info.Pending,
		Active:           info.Active,
		Scheduled:        info.Scheduled,
		Retry:            info.Retry,
		Archived:         info.Archived,
		Completed:        info.Completed,
		OldestPendingAge: info.Latency,
		Processed:        int64(info.ProcessedTotal),
		Failed:           int64(info.FailedTotal),
	}, nil
}

// Archived lists the queue's dead-letter records, newest first. An empty
// archive answers an empty slice, never nil.
func (i *Inspector) Archived(ctx context.Context, queue string) ([]jobs.ArchivedTask, error) {
	entries, err := i.inspector.ListArchivedTasks(queue)
	if err != nil {
		return nil, fmt.Errorf("list archived tasks in queue %q: %w", queue, err)
	}
	rows := make([]jobs.ArchivedTask, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, jobs.ArchivedTask{
			ID:       entry.ID,
			Type:     entry.Type,
			Queue:    entry.Queue,
			Retried:  entry.Retried,
			MaxRetry: entry.MaxRetry,
			LastErr:  entry.LastErr,
		})
	}
	return rows, nil
}

// Replay moves one archived task back to pending — the dead-letter
// story's manual re-run. The retry counters stay where the failure left
// them, so another failed attempt re-archives immediately; only a fix
// (in code or payload) changes the outcome.
func (i *Inspector) Replay(ctx context.Context, queue, id string) error {
	if err := i.inspector.RunTask(queue, id); err != nil {
		return fmt.Errorf("replay task %q in queue %q: %w", id, queue, err)
	}
	return nil
}

// DeleteTask removes a task from a queue's pending, scheduled, retry, or
// archived state — the operator's last resort and this demo's teardown.
func (i *Inspector) DeleteTask(ctx context.Context, queue, id string) error {
	if err := i.inspector.DeleteTask(queue, id); err != nil {
		return fmt.Errorf("delete task %q in queue %q: %w", id, queue, err)
	}
	return nil
}

// Close releases the inspector's bookkeeping (the shared pool stays
// open — it belongs to the process).
func (i *Inspector) Close() error {
	if err := i.inspector.Close(); err != nil {
		return fmt.Errorf("close jobs inspector: %w", err)
	}
	return nil
}
