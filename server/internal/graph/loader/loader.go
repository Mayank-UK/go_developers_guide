// Package loader is the guide's hand-rolled, request-scoped batch loader:
// the answer to GraphQL's N+1 problem, written out in full because the
// moving parts are few and worth knowing (coalescing window, dedupe,
// futures) — no graph-gophers/dataloader dependency needed. A loader is
// created per request, handed the batch seam (ONE call resolving many
// ids), and Loaded concurrently by sibling field resolvers; every Load
// that lands inside the coalescing window shares a single batch fetch.
package loader

import (
	"context"
	"sync"
	"time"

	"guide/internal/core"
	"guide/internal/domain/posts"
)

// BatchFetch is the batching seam: ONE call that resolves many post ids at
// once, returning the posts it found keyed by id. Absent ids are not the
// fetch's business — the loader turns an absence into the typed
// core.NotFound error so callers see the same miss the point-read path
// serves.
type BatchFetch func(ctx context.Context, ids []string) (map[string]posts.Post, error)

// loaderWindow is how long the dispatcher waits for stragglers after the
// last Load before firing one batch. Long enough to collect a page's worth
// of concurrent field resolutions (each a few microseconds), short enough
// that no request notices it.
const loaderWindow = 5 * time.Millisecond

// PostsLoader coalesces concurrent Load calls into one BatchFetch. It is
// request-scoped by design: build one per request, Load it from sibling
// resolvers, let it retire — it carries per-request bookkeeping (the
// pending map) that must never be shared across requests.
type PostsLoader struct {
	fetch  BatchFetch
	window time.Duration
	// batchLimit is the closed-batch switch: zero (the default) dispatches
	// on the window; positive dispatches the instant exactly batchLimit
	// Load calls are pending — the count, not the clock, draws the batch
	// boundary.
	batchLimit int

	mu      sync.Mutex
	waiters map[string][]chan result
	order   []string
	started bool
	pending int

	wake chan struct{}
	full chan struct{}
}

// result is one Load's answer, delivered on the waiter's channel.
type result struct {
	post posts.Post
	err  error
}

// Option configures a loader at construction; NewPostsLoader applies
// them left to right.
type Option func(*PostsLoader)

// WithClosedBatch switches dispatch from timing to COUNT: the batch fires
// the instant exactly `loads` Load calls are pending, and the coalescing
// window NEVER fires it early. That is the contract a known fan-out wants
// — the issuer knows how many sibling lookups ride one batch — and the one
// a pinned, byte-identical experiment needs: no amount of machine load can
// split the batch, and no window latency is paid. The caller owes the
// loader exactly that many Loads (or a context cancellation to unwind the
// wait): fewer leave the batch pending; any beyond the count overflow into
// a second batch cycle.
func WithClosedBatch(loads int) Option {
	return func(l *PostsLoader) {
		if loads > 0 {
			l.batchLimit = loads
		}
	}
}

// NewPostsLoader builds a loader over the batch seam. The context is
// captured on first Load and carries cancellation into the batch fetch.
func NewPostsLoader(fetch BatchFetch, opts ...Option) *PostsLoader {
	l := &PostsLoader{
		fetch:   fetch,
		window:  loaderWindow,
		waiters: make(map[string][]chan result),
		wake:    make(chan struct{}, 1),
		full:    make(chan struct{}, 1),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Load registers an id and blocks until its batch resolves — a future,
// collapsed into the call. Concurrent Loads batch; duplicate ids are
// deduplicated (both waiters share one entry and one answer). Calling Load
// after the loader's context died returns the typed cancellation error
// rather than hanging forever.
func (l *PostsLoader) Load(ctx context.Context, id string) (posts.Post, error) {
	// A buffered channel of one: the dispatcher delivers without holding
	// a goroutine hostage, and the receiver may arrive late or never
	// (context canceled) without blocking delivery.
	ch := make(chan result, 1)

	l.mu.Lock()
	if _, seen := l.waiters[id]; !seen {
		l.order = append(l.order, id)
	}
	l.waiters[id] = append(l.waiters[id], ch)
	l.pending++
	batchFull := l.batchLimit > 0 && l.pending >= l.batchLimit
	// Spawn the single dispatcher under the same lock that guards the
	// waiters: either this Load sees started=false and spawns it, or a
	// previous dispatcher is still alive to serve this waiter — the
	// retire path below clears the flag only when the map is empty, so
	// the two can never cross.
	if !l.started {
		l.started = true
		go l.dispatchLoop(ctx)
	}
	l.mu.Unlock()
	// Nudge the dispatcher: non-blocking — it only needs to know work
	// exists, and a dropped nudge is caught by the window timer.
	select {
	case l.wake <- struct{}{}:
	default:
	}
	if batchFull {
		// Closed batch filled: tell the dispatcher to fire NOW — the
		// count, not the clock, drew this batch's boundary.
		select {
		case l.full <- struct{}{}:
		default:
		}
	}

	select {
	case got := <-ch:
		return got.post, got.err
	case <-ctx.Done():
		return posts.Post{}, core.Wrapf(ctx.Err(), core.KindInternal,
			"Load of post %q canceled before its batch resolved.", id)
	}
}

// dispatchLoop is the loader's one background goroutine: after every nudge
// it waits out the coalescing window, fires the batch when the window
// closes with work pending, and retires one window after the last dispatch
// — no goroutine leak, no shutdown API to forget to call. In closed-batch
// mode the count fires the batch instead, and the window keeps only its
// retirement duty.
func (l *PostsLoader) dispatchLoop(ctx context.Context) {
	idle := time.NewTimer(l.window)
	defer idle.Stop()

	for {
		select {
		case <-l.wake:
			// Straggler arrived: extend the window so it joins this batch.
			if !idle.Stop() {
				drainTimer(idle)
			}
			idle.Reset(l.window)
		case <-l.full:
			// Closed batch filled: fire at once. The re-armed timer is
			// liveness only — it retires the dispatcher one quiet window
			// after its work is done.
			l.fire(ctx)
			idle.Reset(l.window)
		case <-idle.C:
			if l.retireIfQuiet() {
				return
			}
			if l.batchLimit > 0 {
				// Closed batch: the timer is liveness, not a trigger —
				// it must never fire a batch early; only the count does.
				idle.Reset(l.window)
				continue
			}
			l.fire(ctx)
			idle.Reset(l.window)
		case <-ctx.Done():
			l.retireIfQuiet()
			l.failAll(ctx.Err())
			return
		}
	}
}

// retireIfQuiet flips the dispatcher-off switch when no Load is pending
// and reports whether the caller should exit. The flag and the waiters
// share one mutex, so a Load can never register against a dispatcher that
// just decided to leave: either it arrives first (waiters non-empty, no
// retirement) or it sees started=false and spawns a fresh dispatcher.
func (l *PostsLoader) retireIfQuiet() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.waiters) > 0 {
		return false
	}
	l.started = false
	return true
}

// fire runs the batch: one snapshot of the pending ids (in first-request
// order — deterministic), one BatchFetch, one typed answer per waiter.
func (l *PostsLoader) fire(ctx context.Context) {
	l.mu.Lock()
	ids := append([]string(nil), l.order...)
	waiters := l.waiters
	l.order = nil
	l.waiters = make(map[string][]chan result)
	l.pending = 0
	l.mu.Unlock()

	fetched, err := l.fetch(ctx, ids)
	for _, id := range ids {
		answer := result{}
		switch {
		case err != nil:
			answer.err = err
		default:
			if post, ok := fetched[id]; ok {
				answer.post = post
			} else {
				answer.err = core.Newf(core.KindNotFound, "Post %q not found.", id)
			}
		}
		for _, ch := range waiters[id] {
			ch <- answer
		}
	}
}

// failAll answers every pending Load with the cancellation cause so a
// canceled request's resolvers all unwind instead of hanging.
func (l *PostsLoader) failAll(cause error) {
	l.mu.Lock()
	waiters := l.waiters
	l.order = nil
	l.waiters = make(map[string][]chan result)
	l.pending = 0
	l.mu.Unlock()

	for _, queue := range waiters {
		for _, ch := range queue {
			ch <- result{err: core.Wrapf(cause, core.KindInternal,
				"Batch canceled before it resolved.")}
		}
	}
}

// drainTimer empties a timer's channel after a failed Stop raced the
// timer firing — the standard-library dance that keeps Reset honest.
func drainTimer(timer *time.Timer) {
	select {
	case <-timer.C:
	default:
	}
}

// PortBatch adapts the posts port into a BatchFetch: ONE unfiltered List
// page read (the whole collection while it fits a page, in deterministic
// oldest-first order), indexed by id — one port call serving every id in
// the batch. A database adapter grows a real GetByIDs port method
// (`WHERE id = ANY($1)`); against this in-memory substrate, the single
// page read IS the batch, and the counting wrapper below proves it: one
// counted call, no matter how many ids ride along.
func PortBatch(repo posts.PostsRepository) BatchFetch {
	return func(ctx context.Context, ids []string) (map[string]posts.Post, error) {
		page, err := repo.List(ctx, posts.ListQuery{
			Page:  1,
			Limit: posts.MaxPageLimit,
			Sort:  posts.SortOldest,
		})
		if err != nil {
			return nil, err
		}
		byID := make(map[string]posts.Post, len(page.Items))
		for _, post := range page.Items {
			byID[post.ID] = post
		}
		return byID, nil
	}
}
