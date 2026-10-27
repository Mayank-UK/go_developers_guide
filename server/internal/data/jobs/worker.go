package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	goredis "github.com/redis/go-redis/v9"

	"guide/internal/domain/demos"
)

// DemoQueueWeights is the hosted worker's queue map — the weights are
// live policy, not decoration: the main demos queue is polled six times
// as often as the low one whenever both hold work, and the console
// story's poison task rides the low queue so the ratio carries traffic.
// Strict priority is deliberately OFF: weighted fairness keeps a busy
// critical queue from starving the low lanes, which is the choice most
// production workers want.
var DemoQueueWeights = map[string]int{
	demos.JobsDemoQueue:    6,
	demos.JobsDemoLowQueue: 1,
}

// demoWorkerConcurrency bounds the hosted worker's concurrent handlers.
// Two is honest for a demo (and one per queue would also do): the
// choreography's tasks are tiny, and the number is what Config.Concurrency
// means — the worker's lane count, not a goroutine-per-task free-for-all.
const demoWorkerConcurrency = 2

// ErrWorkerAlreadyStarted is what Start answers to a double start — the
// composition root treating it as a boot bug, the same posture a second
// HTTP listener would get.
var ErrWorkerAlreadyStarted = errors.New("jobs worker: already started")

// ServerRunner hosts the demo's asynq worker so the composition root can
// start and stop it with one call each: Start hands the broker a server
// bound to the two weighted queues with the demo's retry policy and
// handler mux; Shutdown drains in-flight handlers inside the shutdown
// timeout and is safe to call twice. The worker shares the process's
// go-redis client — no second pool, no second address to configure. The
// console demo presumes exactly this runner (or an identical one) is
// draining the same queues while it runs.
type ServerRunner struct {
	client     *goredis.Client
	retryDelay asynq.RetryDelayFunc

	startOnce sync.Mutex
	started   bool
	server    *asynq.Server
	baseCtx   context.Context
}

// NewServerRunner wires the hosted worker over the shared go-redis
// client with the demo's retry policy and queue weights. Construction
// dials nothing: a down Redis never blocks boot, and the broker's first
// contact happens inside Start's pollers.
func NewServerRunner(client *goredis.Client) *ServerRunner {
	return &ServerRunner{
		client:     client,
		retryDelay: adaptRetryDelay(DemoRetryDelay),
	}
}

// Start launches the worker: handlers' contexts derive from ctx (cancel
// it and the worker's polls stop), failures wait out DemoRetryDelay, and
// both weighted queues drain through NewDemoMux. Start is non-blocking —
// the server's pollers run on their own goroutines — and refuses a
// second start with ErrWorkerAlreadyStarted instead of silently hosting
// two workers on the same queues.
func (r *ServerRunner) Start(ctx context.Context) error {
	r.startOnce.Lock()
	defer r.startOnce.Unlock()
	if r.started {
		return ErrWorkerAlreadyStarted
	}
	if ctx == nil {
		return errors.New("jobs worker: Start needs a non-nil context")
	}
	r.baseCtx = ctx
	r.server = asynq.NewServerFromRedisClient(r.client, asynq.Config{
		Concurrency:    demoWorkerConcurrency,
		Queues:         DemoQueueWeights,
		RetryDelayFunc: r.retryDelay,
		// Canceling the base context unblocks in-flight handler
		// contexts; the shutdown timeout still bounds the drain.
		ShutdownTimeout: 5 * time.Second,
		BaseContext: func() context.Context {
			return r.baseCtx
		},
	})
	if err := r.server.Start(NewDemoMux(r.client)); err != nil {
		r.server = nil
		return fmt.Errorf("jobs worker start: %w", err)
	}
	r.started = true
	return nil
}

// Shutdown drains in-flight handlers and stops the pollers. It is
// idempotent (a second call is a no-op) and safe before Start — matching
// the graceful-shutdown shape the composition root needs at process end.
func (r *ServerRunner) Shutdown() {
	r.startOnce.Lock()
	defer r.startOnce.Unlock()
	if !r.started || r.server == nil {
		return
	}
	r.server.Shutdown()
	r.started = false
	r.server = nil
}

// adaptRetryDelay lifts the domain-shaped policy (attempt, error) onto
// asynq's signature: the broker hands the handler the RETRY COUNT so far
// (0 on the first failure), while the domain's vocabulary counts the
// attempt that just failed 1-based — the +1 is the whole translation,
// and without it a first-rung policy shifting by attempt-1 would shift
// by -1 and panic. The task parameter is dropped so the policy stays
// unit-testable with two numbers and an error, no broker.
func adaptRetryDelay(policy func(int, error) time.Duration) asynq.RetryDelayFunc {
	return func(n int, err error, _ *asynq.Task) time.Duration {
		return policy(n+1, err)
	}
}
