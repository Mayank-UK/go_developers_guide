package security

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RatePolicy is a token-bucket contract: a sustained RATE of tokens
// per second and a BURST ceiling — the bucket size the client may
// spend instantly. The two numbers answer different questions: burst
// is "how many may land at once", rate is "how fast the allowance
// refills", and honest rate limiting states both.
type RatePolicy struct {
	Rate  rate.Limit // sustained allowance, tokens per second (fractional allowed)
	Burst int        // bucket size — the instant ceiling
}

// String renders the policy the way the demos teach it.
func (p RatePolicy) String() string {
	return fmt.Sprintf("%s sustained, burst %d", rateText(p.Rate), p.Burst)
}

// rateText renders a rate as a per-second fraction when it is not
// whole ("0.5/s" rather than a float soup).
func rateText(limit rate.Limit) string {
	perSecond := float64(limit)
	if perSecond == float64(int(perSecond)) {
		return fmt.Sprintf("%d/s", int(perSecond))
	}
	return fmt.Sprintf("%g/s", perSecond)
}

// RateDecision is one Allow verdict. RetryAfter is the honest delay
// until the NEXT token would be available — zero when allowed, and the
// value the 429 surfaces as its retry hint.
type RateDecision struct {
	Allowed    bool
	RetryAfter time.Duration
}

// RetryAfterText renders the retry hint for display, truncated to the
// millisecond (the token bucket's own math returns durations with
// nanosecond tails no human needs to read).
func (d RateDecision) RetryAfterText() string {
	if d.RetryAfter <= 0 {
		return "now"
	}
	return d.RetryAfter.Truncate(time.Millisecond).String()
}

// ClientRateLimiter is a per-client token-bucket gate over
// golang.org/x/time/rate: one rate.Limiter per client key, created on
// first sight and shared by every request that key makes. The
// x/time/rate Limiter is a TOKEN BUCKET: it starts full (burst tokens),
// every allowed request spends one, and tokens refill continuously at
// rate — not at window boundaries — so an exhausted client recovers
// one request's worth of allowance every 1/rate seconds. The map grows
// one entry per distinct key; the demo keys are bounded by client
// count, and a production deployment facing unbounded keys (open
// signup) pairs this with idle-bucket eviction or a shared store — a
// deliberate simplification, named rather than hidden.
type ClientRateLimiter struct {
	policy  RatePolicy
	mu      sync.Mutex
	buckets map[string]*rate.Limiter
}

// NewClientRateLimiter builds the gate. A burst below one or a
// non-positive rate is a configuration bug that would make the
// limiter refuse or allow everything, so construction refuses it
// loudly instead.
func NewClientRateLimiter(policy RatePolicy) (*ClientRateLimiter, error) {
	if policy.Burst < 1 || policy.Rate <= 0 {
		return nil, fmt.Errorf("rate policy %s: burst must be >= 1 and rate > 0", policy)
	}
	return &ClientRateLimiter{policy: policy, buckets: make(map[string]*rate.Limiter)}, nil
}

// Policy answers the policy every bucket was built from.
func (l *ClientRateLimiter) Policy() RatePolicy { return l.policy }

// Size answers how many per-client buckets currently exist — one per
// distinct key seen. The demo uses it to prove the buckets are
// per-client, and nothing else grows the number here.
func (l *ClientRateLimiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// bucket resolves (creating on first sight) the key's limiter.
func (l *ClientRateLimiter) bucket(key string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket := l.buckets[key]
	if bucket == nil {
		bucket = rate.NewLimiter(l.policy.Rate, l.policy.Burst)
		l.buckets[key] = bucket
	}
	return bucket
}

// Allow answers one non-blocking request against the key's bucket.
// The Reserve/Cancel pair is the honest way to refuse: Reserve claims
// a token and reports the delay it would cost; a zero delay means the
// token was available NOW (allowed), and any other delay means the
// request is over budget — so the reservation is cancelled, returning
// the token to the bucket for the next requester, and the decision
// carries that delay as the RetryAfter hint. Nothing is consumed by a
// refused request and nothing blocks.
func (l *ClientRateLimiter) Allow(key string) RateDecision {
	reservation := l.bucket(key).Reserve()
	if !reservation.OK() {
		// OK=false means a zero burst — construction forbids it; this
		// branch is defensive, and refuses rather than trusts.
		return RateDecision{Allowed: false}
	}
	delay := reservation.Delay()
	if delay == 0 {
		return RateDecision{Allowed: true}
	}
	reservation.Cancel()
	return RateDecision{Allowed: false, RetryAfter: delay}
}

// Wait blocks until the key's bucket can afford one token (or the
// context is cancelled) — the COOPERATIVE counterpart to Allow, for
// clients that would rather slow down than be refused. Context first:
// the call can block, so cancellation must reach it.
func (l *ClientRateLimiter) Wait(ctx context.Context, key string) error {
	if err := l.bucket(key).Wait(ctx); err != nil {
		return fmt.Errorf("wait for rate token: %w", err)
	}
	return nil
}
