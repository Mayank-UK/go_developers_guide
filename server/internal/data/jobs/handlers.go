package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	goredis "github.com/redis/go-redis/v9"

	"guide/internal/domain/demos"
)

// DemoRetryDelay is the demo's retry policy: exponential with a hard cap
// — 500ms for the first failure, doubling per attempt, never more than
// 2s. It stands in for asynq's default (sidekiq's formula: n⁴ + 15 +
// jitter seconds, which would make a three-attempt demo take half a
// minute) so the choreography is visible in seconds while keeping the
// SHAPE production uses: fast first retry, growing gaps, bounded worst
// case. The attempt is 1-BASED (the domain's vocabulary — the adapter
// translates the broker's 0-based retry count onto it), and a
// non-positive input answers the first rung instead of shifting
// negatively and panicking. A production worker plugs its own policy
// here — the server takes it as a RetryDelayFunc seam, not a constant.
func DemoRetryDelay(attempt int, _ error) (delay time.Duration) {
	const base = 500 * time.Millisecond
	const capAt = 2 * time.Second
	if attempt < 1 {
		return base
	}
	shifted := base << (attempt - 1)
	if shifted > capAt || shifted <= 0 {
		return capAt
	}
	return shifted
}

// NewDemoMux builds the worker's handler table: one handler per demo
// task type, each recording its invocation to the run's redis ledger
// before doing its (deliberately simple) work. The ledger write is part
// of the contract — the story's payload derives every proof sentence
// from what these handlers appended.
func NewDemoMux(client *goredis.Client) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(demos.JobsTaskFlaky, flakyHandler(client))
	mux.HandleFunc(demos.JobsTaskPoison, poisonHandler(client))
	mux.HandleFunc(demos.JobsTaskDigest, digestHandler(client))
	mux.HandleFunc(demos.JobsTaskTick, tickHandler(client))
	return mux
}

// flakyHandler fails the first demos.JobsFlakyFailFirst attempts, then
// succeeds. The attempt number is a redis INCR keyed by the task's
// caller-minted id — durable across redeliveries, so a task the broker
// hands over twice counts its attempts honestly instead of starting
// over. The INCR IS the "have we met before" state; everything else is
// derived from it.
func flakyHandler(client *goredis.Client) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, task *asynq.Task) error {
		payload, err := demos.ParseJobsDemoPayload(task.Payload())
		if err != nil {
			// A payload we cannot parse will never parse on a retry
			// either — skip the ladder and fail straight through.
			return fmt.Errorf("flaky task %s: %w", taskIDFrom(ctx), err)
		}
		attempt, err := incrAttempt(ctx, client, payload)
		if err != nil {
			return err
		}
		if attempt <= demos.JobsFlakyFailFirst {
			if err := recordDemoEvent(ctx, client, payload, demos.JobsTaskFlaky, attempt, demos.JobsOutcomeFailed); err != nil {
				return err
			}
			return fmt.Errorf("flaky task %s: attempt %d of %d failed (a flaky upstream, by design)",
				payload.TaskID, attempt, demos.JobsFlakyFailFirst+1)
		}
		if err := recordDemoEvent(ctx, client, payload, demos.JobsTaskFlaky, attempt, demos.JobsOutcomeSucceeded); err != nil {
			return err
		}
		return nil
	}
}

// poisonHandler always fails — the poison message the retry budget
// cannot fix. The failure text is deliberately descriptive: when this
// task lands in the archive, this string is the whole post-mortem an
// operator reads. The attempt number comes from the broker's own retry
// state in the context — poison behavior derives from delivery history,
// not from an app-side counter.
func poisonHandler(client *goredis.Client) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, task *asynq.Task) error {
		payload, err := demos.ParseJobsDemoPayload(task.Payload())
		if err != nil {
			return fmt.Errorf("poison task %s: %w", taskIDFrom(ctx), err)
		}
		attempt := retryCountFrom(ctx) + 1
		if err := recordDemoEvent(ctx, client, payload, demos.JobsTaskPoison, attempt, demos.JobsOutcomeFailed); err != nil {
			return err
		}
		return fmt.Errorf("poison task %s: upstream answered 500 for attempt %d — retrying cannot fix this",
			payload.TaskID, attempt)
	}
}

// digestHandler is the canonical job shape: parse the payload, do a
// small bounded piece of work (here: record it), succeed on the first
// attempt. This is the handler a real "send the receipt email" task
// would grow from — the queue carries the DECISION out of band, and the
// handler's job is to finish it durably.
func digestHandler(client *goredis.Client) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, task *asynq.Task) error {
		payload, err := demos.ParseJobsDemoPayload(task.Payload())
		if err != nil {
			return fmt.Errorf("digest task %s: %w", taskIDFrom(ctx), err)
		}
		return recordDemoEvent(ctx, client, payload, demos.JobsTaskDigest, retryCountFrom(ctx)+1, demos.JobsOutcomeSucceeded)
	}
}

// tickHandler records one cron fire. Tick tasks get broker-assigned ids
// (a cron entry enqueues a NEW task per fire — a caller-minted id would
// collide with itself every second), so the ledger's TaskID is the
// payload's logical id, not the broker's.
func tickHandler(client *goredis.Client) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, task *asynq.Task) error {
		payload, err := demos.ParseJobsDemoPayload(task.Payload())
		if err != nil {
			return fmt.Errorf("tick task %s: %w", taskIDFrom(ctx), err)
		}
		return recordDemoEvent(ctx, client, payload, demos.JobsTaskTick, 1, demos.JobsOutcomeSucceeded)
	}
}

// taskIDFrom reads the broker-assigned task id from the handler's
// context, for error strings about tasks whose payload could not be
// parsed (the payload's own id is unreachable in that branch).
func taskIDFrom(ctx context.Context) string {
	if id, ok := asynq.GetTaskID(ctx); ok {
		return id
	}
	return "unknown"
}

// incrAttempt advances and answers the task's durable attempt counter:
// INCR on a per-task key under the run's namespace, with the residue TTL
// re-armed. The counter is what makes double-delivery (a worker crash
// mid-handler, a claim race) count itself honestly.
func incrAttempt(ctx context.Context, client *goredis.Client, payload demos.JobsDemoPayload) (int, error) {
	key := attemptKey(payload)
	attempt, err := client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("INCR %s: %w", key, err)
	}
	if err := client.Expire(ctx, key, demos.JobsDemoEventTTL).Err(); err != nil {
		return 0, fmt.Errorf("EXPIRE %s: %w", key, err)
	}
	return int(attempt), nil
}

// retryCountFrom reads the retry count the broker hands the handler in
// its context — the retry state the broker itself carries.
func retryCountFrom(ctx context.Context) int {
	retried, ok := asynq.GetRetryCount(ctx)
	if !ok {
		return 0
	}
	return retried
}

// recordDemoEvent appends one invocation record to the run's ledger
// (RPUSH + TTL) and answers the write's error so the handler can fail
// the delivery with it. An event the story cannot read back is a story
// that cannot be told honestly, and silently dropping rows would let the
// payload claim a timeline that did not happen — so a failed write fails
// the handler, and asynq's retry ladder redelivers the task: the attempt
// counter counts the redelivery honestly and the ledger gains the event
// on the next pass. A Redis blip is an operational failure, not a
// programmer bug — the error return is the convention-true shape (a
// panic here would crash a worker over an outage the broker already
// knows how to retry).
func recordDemoEvent(ctx context.Context, client *goredis.Client, payload demos.JobsDemoPayload, taskType string, attempt int, outcome string) error {
	event := demos.JobsDemoEvent{
		TaskID:  payload.TaskID,
		Type:    taskType,
		Attempt: attempt,
		Outcome: outcome,
		At:      time.Now().UTC(),
	}
	encoded, err := demos.MarshalJobsDemoEvent(event)
	if err != nil {
		return fmt.Errorf("jobs demo: marshal ledger event: %w", err)
	}
	key := eventsKey(payload.Run)
	if err := client.RPush(ctx, key, encoded).Err(); err != nil {
		return fmt.Errorf("jobs demo: RPUSH %s: %w", key, err)
	}
	if err := client.Expire(ctx, key, demos.JobsDemoEventTTL).Err(); err != nil {
		return fmt.Errorf("jobs demo: EXPIRE %s: %w", key, err)
	}
	return nil
}

// eventsKey is the run's ledger key.
func eventsKey(run string) string { return run + ":events" }

// attemptKey is one task's durable attempt-counter key.
func attemptKey(payload demos.JobsDemoPayload) string {
	return payload.Run + ":attempts:" + payload.TaskID
}
