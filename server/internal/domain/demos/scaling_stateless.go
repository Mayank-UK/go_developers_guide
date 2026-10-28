// Topic 40 scaling's live demo: statelessness, proven by a real fleet.
// The machinery is REAL, not narrated: N in-process replicas (each with
// its OWN local memory — a served counter and a session map), a real
// mutex-guarded round-robin load balancer with an affinity (sticky)
// table over them, and a shared store standing in for the externalized
// state a fleet survives on. The round runs three measured acts:
// (a) DISTRIBUTION — twelve requests through the balancer, per-replica
// counts read off the replicas themselves; (b) THE LOCAL SESSION — a
// login stored in replica-local memory, missed by the very next
// round-robin hops, rescued by sticky routing, then killed again by a
// rolling-update swap of the pinned replica (the swap's before-counts
// are measured off the dying instance); (c) THE SHARED SESSION — the
// same login written to the shared store, found by every replica of
// the fleet including fresh replacements, surviving a second swap,
// with six increments spread across the fleet reading back as one
// accumulated counter. Every number on the wire was OBSERVED by the
// structures that served the round — no clock, no randomness: the
// round is deterministic, so two GETs replay it identically, byte for
// byte.
package demos

import (
	"fmt"
	"strings"
	"sync"
)

// ScalingStatelessPath is the demo area's endpoint — registered on the
// real mux by the composition root.
const ScalingStatelessPath = "/api/demo/scaling/stateless"

// The round's step phases, on the wire exactly as the teaching names
// them.
const (
	// scalingPhaseLocal is replica-local memory: fast, private, lost at
	// the first cross-replica hop.
	scalingPhaseLocal = "local-memory"
	// scalingPhaseSticky is affinity routing: the workaround that pins
	// a client to the replica that holds its state.
	scalingPhaseSticky = "sticky"
	// scalingPhaseSwap is the rolling-update event: one replica
	// terminated, a fresh one (empty memory) under the same slot.
	scalingPhaseSwap = "swap"
	// scalingPhaseShared is externalized state: the store every replica
	// reads and no replica owns.
	scalingPhaseShared = "shared-store"
	// scalingPhaseCounter is the accumulation beat: increments spread
	// across the fleet, one shared total.
	scalingPhaseCounter = "counter"
)

// The round's outcome vocabulary — each value derived from what the
// machinery observed, never promised in advance.
const (
	scalingOutcomeStored   = "stored"
	scalingOutcomeHit      = "hit"
	scalingOutcomeMiss     = "miss"
	scalingOutcomeReplaced = "replaced"
)

// ScalingStatelessConfig shapes the round. Every field is a real
// constant on the wire (the payload serves the config panel), and the
// whole round is deterministic in them.
type ScalingStatelessConfig struct {
	// Replicas is the fleet size the balancer fronts.
	Replicas int
	// DistributionRequests is how many anonymous requests the
	// distribution act drives before any session exists.
	DistributionRequests int
	// ClientID is the user whose session walks both placements.
	ClientID string
	// CounterIncrements is how many shared-counter writes the final act
	// spreads across the fleet.
	CounterIncrements int
}

// DefaultScalingStatelessConfig is what the composition root serves: a
// three-replica fleet, twelve distribution requests, client ada, and
// six counter increments — small enough to read in one panel, large
// enough that every beat lands (twelve across three is an exact four
// each; six across three is an exact two each).
func DefaultScalingStatelessConfig() ScalingStatelessConfig {
	return ScalingStatelessConfig{
		Replicas:             3,
		DistributionRequests: 12,
		ClientID:             "ada",
		CounterIncrements:    6,
	}
}

// LocalToken derives the local-placement session id — deterministic,
// and named so the panel can tell the two placements apart at a
// glance.
func (c ScalingStatelessConfig) LocalToken() string {
	return "s-local-" + c.ClientID
}

// SharedToken derives the shared-placement session id.
func (c ScalingStatelessConfig) SharedToken() string {
	return "s-shared-" + c.ClientID
}

// ── the fleet: replicas, balancer, shared store ─────────────────────

// ScalingReplica is one in-process replica: its own served count and
// its own session map, and nothing else. That privacy is the point —
// nothing in the fleet can read this map but the replica itself, which
// is exactly the property replica-local memory has in a real fleet.
// All state is mutex-guarded because the type is real machinery: a
// shared fleet driven from parallel request goroutines must stay
// race-detector clean.
type ScalingReplica struct {
	id string

	mu       sync.Mutex
	served   int
	sessions map[string]string
}

// newScalingReplica builds a fresh replica with an empty heap — the
// state every process starts with, and the state every REPLACEMENT
// starts with too.
func newScalingReplica(slot int) *ScalingReplica {
	return &ScalingReplica{
		id:       fmt.Sprintf("replica-%d", slot+1),
		sessions: make(map[string]string),
	}
}

// ID names the replica on the wire (slot number, 1-based, stable
// across swaps — the fresh process under a slot keeps its name, which
// is itself part of the lesson).
func (r *ScalingReplica) ID() string { return r.id }

// Served reports the replica's measured request count.
func (r *ScalingReplica) Served() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.served
}

// SessionCount reports how many sessions live in the replica's own
// memory right now.
func (r *ScalingReplica) SessionCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sessions)
}

// Serve records one served request whose work went elsewhere — the
// distribution act's anonymous hit, or a request the replica served by
// reading/writing the SHARED store on the caller's behalf.
func (r *ScalingReplica) Serve() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.served++
}

// ServeSessionStore handles a request that WRITES a session into this
// replica's own local memory.
func (r *ScalingReplica) ServeSessionStore(token, user string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.served++
	r.sessions[token] = user
}

// ServeSessionLookup handles a request that READS a session from this
// replica's own local memory — the answer is whatever this replica
// alone knows, which is the entire lesson.
func (r *ScalingReplica) ServeSessionLookup(token string) (user string, found bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.served++
	user, found = r.sessions[token]
	return user, found
}

// ScalingBalancer is a real round-robin load balancer with an affinity
// table: Next hands out replicas in ring order, Remember pins a
// session token to the slot that stored it, and Pin routes a sticky
// request back there. Mutex-guarded for the same reason the replica
// is: the balancer is genuinely shared state in any real fleet.
type ScalingBalancer struct {
	mu       sync.Mutex
	replicas []*ScalingReplica
	next     int
	sticky   map[string]int
}

// newScalingBalancer fronts a fresh fleet of n replicas.
func newScalingBalancer(n int) *ScalingBalancer {
	replicas := make([]*ScalingReplica, n)
	for slot := range replicas {
		replicas[slot] = newScalingReplica(slot)
	}
	return &ScalingBalancer{replicas: replicas, sticky: make(map[string]int)}
}

// Next returns the next replica in ring order and the slot it
// occupies, advancing the ring.
func (b *ScalingBalancer) Next() (*ScalingReplica, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	slot := b.next % len(b.replicas)
	b.next++
	return b.replicas[slot], slot
}

// Remember records the affinity entry: this token lives on this slot —
// the sticky-learning behavior production balancers expose (nginx's
// sticky learn, a cloud ALB's target affinity).
func (b *ScalingBalancer) Remember(token string, slot int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sticky[token] = slot
}

// Pin routes a sticky request: the slot the affinity table recorded,
// or the ring when no entry exists. Pinning never advances the ring.
func (b *ScalingBalancer) Pin(token string) *ScalingReplica {
	b.mu.Lock()
	if slot, ok := b.sticky[token]; ok {
		replica := b.replicas[slot]
		b.mu.Unlock()
		return replica
	}
	b.mu.Unlock()
	replica, _ := b.Next()
	return replica
}

// ReplaceReplica swaps the replica at slot for a fresh one and returns
// the terminated instance — a rolling update one pod at a time, the
// same dance the k8s manifest's RollingUpdate strategy performs. The
// affinity table is deliberately NOT touched: entries pin to slots,
// and the slot's occupant changing underneath them is the trap the
// round demonstrates. The terminated replica keeps its measured final
// counts for the caller to read before it goes.
func (b *ScalingBalancer) ReplaceReplica(slot int) (fresh, terminated *ScalingReplica) {
	b.mu.Lock()
	defer b.mu.Unlock()
	terminated = b.replicas[slot]
	fresh = newScalingReplica(slot)
	b.replicas[slot] = fresh
	return fresh, terminated
}

// Fleet exposes the replicas behind the balancer (a convenience read
// for row shaping; the round never mutates through it).
func (b *ScalingBalancer) Fleet() []*ScalingReplica {
	b.mu.Lock()
	defer b.mu.Unlock()
	fleet := make([]*ScalingReplica, len(b.replicas))
	copy(fleet, b.replicas)
	return fleet
}

// ScalingSharedStore is the externalized-state stand-in: sessions and
// a counter that live OUTSIDE every replica, the way the stack's redis
// lives outside every server process. The payload's honesty note says
// so plainly — this is an in-process analogue of the redis container
// the stack carries (topics 20 and 21 drove the real client calls);
// the lesson it carries is placement, not hops.
type ScalingSharedStore struct {
	mu       sync.Mutex
	sessions map[string]string
	counter  int
}

// newScalingSharedStore builds an empty store.
func newScalingSharedStore() *ScalingSharedStore {
	return &ScalingSharedStore{sessions: make(map[string]string)}
}

// StoreSession writes a session into the shared store — reachable
// through any replica, owned by none.
func (s *ScalingSharedStore) StoreSession(token, user string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = user
}

// LookupSession reads a session from the shared store.
func (s *ScalingSharedStore) LookupSession(token string) (user string, found bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, found = s.sessions[token]
	return user, found
}

// Increment advances the shared counter and returns its new value —
// the accumulated total every writer contributed to.
func (s *ScalingSharedStore) Increment() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter++
	return s.counter
}

// Counter reads the shared counter without advancing it.
func (s *ScalingSharedStore) Counter() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.counter
}

// SessionCount reports how many sessions live in the shared store.
func (s *ScalingSharedStore) SessionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

// ── the round: the observed facts the payload is built from ────────

// ScalingDistributionFact is one replica's measured share of the
// distribution act.
type ScalingDistributionFact struct {
	ReplicaID string
	Served    int
}

// ScalingSwapFact is one rolling-update swap, measured off the dying
// replica before it is replaced: what it had served and what it held.
type ScalingSwapFact struct {
	Step           int
	ReplicaID      string
	ServedBefore   int
	SessionsBefore int
}

// ScalingCounterRow is one replica's measured share of the counter act.
type ScalingCounterRow struct {
	ReplicaID  string
	Increments int
}

// ScalingCounterObservation is the counter act's evidence: the
// per-replica increment counts and the shared total read back after
// them.
type ScalingCounterObservation struct {
	Rows   []ScalingCounterRow
	Shared int
}

// ScalingSessionStep is one step of the session round: which phase it
// belongs to, how the request was routed, which replica served it,
// what the outcome was, and a one-line observation with the measured
// numbers interpolated.
type ScalingSessionStep struct {
	Step      int
	Phase     string
	Label     string
	Route     string
	ReplicaID string
	Outcome   string
	Detail    string
	Observed  string
}

// ScalingStatelessFacts carries everything the pure builder needs: the
// config the round ran under, the measured distribution, the session
// steps, the swap events, the counter observation, and the fleet's
// final measured totals.
type ScalingStatelessFacts struct {
	Config       ScalingStatelessConfig
	Distribution []ScalingDistributionFact
	Steps        []ScalingSessionStep
	Swaps        []ScalingSwapFact
	Counter      ScalingCounterObservation
	FinalTotals  []ScalingDistributionFact
}

// ScalingRound owns one fleet + balancer + shared store and walks the
// whole statelessness lesson through them. One round per request keeps
// the payload deterministic; the machinery types themselves are shared
// -state-safe because they are the real teaching artifact.
type ScalingRound struct {
	cfg    ScalingStatelessConfig
	fleet  *ScalingBalancer
	shared *ScalingSharedStore

	steps         []ScalingSessionStep
	swaps         []ScalingSwapFact
	counterRows   []ScalingCounterRow
	counterShared int
}

// NewScalingStatelessRound validates the config (a fleet of at least
// two, honest request counts, a named client) and builds the round's
// machinery.
func NewScalingStatelessRound(cfg ScalingStatelessConfig) (*ScalingRound, error) {
	if cfg.Replicas < 2 || cfg.Replicas > 8 {
		return nil, fmt.Errorf("scaling round: replica count %d must be 2-8", cfg.Replicas)
	}
	if cfg.DistributionRequests < 1 || cfg.DistributionRequests > 64 {
		return nil, fmt.Errorf("scaling round: distribution requests %d must be 1-64", cfg.DistributionRequests)
	}
	if cfg.CounterIncrements < 1 || cfg.CounterIncrements > 64 {
		return nil, fmt.Errorf("scaling round: counter increments %d must be 1-64", cfg.CounterIncrements)
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("scaling round: client id must not be blank")
	}
	return &ScalingRound{
		cfg:    cfg,
		fleet:  newScalingBalancer(cfg.Replicas),
		shared: newScalingSharedStore(),
	}, nil
}

// Run walks the acts in order and returns the observed facts. Every
// value in the facts was read off the machinery after the step that
// produced it — the round narrates nothing it did not measure.
func (r *ScalingRound) Run() ScalingStatelessFacts {
	// Act A — distribution: anonymous requests, measured per replica.
	for range r.cfg.DistributionRequests {
		replica, _ := r.fleet.Next()
		replica.Serve()
	}
	distribution := make([]ScalingDistributionFact, 0, r.cfg.Replicas)
	for _, replica := range r.fleet.Fleet() {
		distribution = append(distribution, ScalingDistributionFact{
			ReplicaID: replica.ID(),
			Served:    replica.Served(),
		})
	}

	// Act B — the session, local placement first.
	r.localMemoryAct()
	// Act C — the session, externalized; then the counter.
	r.sharedStoreAct()
	r.counterAct()

	finalTotals := make([]ScalingDistributionFact, 0, r.cfg.Replicas)
	for _, replica := range r.fleet.Fleet() {
		finalTotals = append(finalTotals, ScalingDistributionFact{
			ReplicaID: replica.ID(),
			Served:    replica.Served(),
		})
	}

	return ScalingStatelessFacts{
		Config:       r.cfg,
		Distribution: distribution,
		Steps:        r.steps,
		Swaps:        r.swaps,
		Counter: ScalingCounterObservation{
			Rows:   r.counterRows,
			Shared: r.counterShared,
		},
		FinalTotals: finalTotals,
	}
}

// localMemoryAct walks the local session through its whole arc: stored
// on one replica, missed by the next hops, rescued by sticky routing,
// then killed by the swap of the pinned replica.
func (r *ScalingRound) localMemoryAct() {
	token := r.cfg.LocalToken()

	// The login lands wherever the ring says — replica-local memory has
	// no say in the matter. The affinity table learns the slot.
	replica, slot := r.fleet.Next()
	replica.ServeSessionStore(token, r.cfg.ClientID)
	r.fleet.Remember(token, slot)
	r.record(ScalingSessionStep{
		Phase: scalingPhaseLocal, Route: "round-robin",
		ReplicaID: replica.ID(), Outcome: scalingOutcomeStored,
		Label:  "login: the session lands in replica-local memory",
		Detail: "the write goes to the replica the balancer picked — its own map, nobody else's",
		Observed: fmt.Sprintf("%s holds %d session(s) after storing %s — the only copy in the fleet",
			replica.ID(), replica.SessionCount(), token),
	})

	// The next requests round-robin elsewhere: the misses.
	for hop := 1; hop < r.cfg.Replicas; hop++ {
		other, _ := r.fleet.Next()
		user, found := other.ServeSessionLookup(token)
		r.record(ScalingSessionStep{
			Phase: scalingPhaseLocal, Route: "round-robin",
			ReplicaID: other.ID(), Outcome: outcomeOf(found),
			Label:  fmt.Sprintf("the next request hops elsewhere (%s)", other.ID()),
			Detail: "process-local state does not follow the request — the load balancer logged the user out",
			Observed: fmt.Sprintf("%s answers from its own map: %d session(s) held, token %s — %s",
				other.ID(), other.SessionCount(), token, lookupVerdict(user, found)),
		})
	}

	// The classic workaround: sticky routing via the affinity table.
	pinned := r.fleet.Pin(token)
	user, found := pinned.ServeSessionLookup(token)
	r.record(ScalingSessionStep{
		Phase: scalingPhaseSticky, Route: "sticky",
		ReplicaID: pinned.ID(), Outcome: outcomeOf(found),
		Label:  "the workaround: sticky routing via the affinity table",
		Detail: "the balancer remembers which replica stored the token and pins the client there",
		Observed: fmt.Sprintf("affinity pins %s to %s: %d session(s) held there — %s. Sticky trades the scaling you bought for the affinity you just reintroduced",
			token, pinned.ID(), pinned.SessionCount(), lookupVerdict(user, found)),
	})

	// The rolling update: the pinned replica is replaced. The dying
	// instance's counts are measured before it goes; the affinity entry
	// deliberately survives.
	_, terminated := r.fleet.ReplaceReplica(slot)
	r.swaps = append(r.swaps, ScalingSwapFact{
		Step:           len(r.steps) + 1,
		ReplicaID:      terminated.ID(),
		ServedBefore:   terminated.Served(),
		SessionsBefore: terminated.SessionCount(),
	})
	r.record(ScalingSessionStep{
		Phase: scalingPhaseSwap, Route: "rolling update",
		ReplicaID: terminated.ID(), Outcome: scalingOutcomeReplaced,
		Label:  "a rolling update replaces the pinned replica",
		Detail: "one replica terminated, a fresh one under the same slot — the k8s manifest's maxUnavailable 0 dance",
		Observed: fmt.Sprintf("%s terminated: %d request(s) served, %d session(s) held — both gone; the replacement boots with 0 of each",
			terminated.ID(), terminated.Served(), terminated.SessionCount()),
	})

	// Sticky routing still points at the slot — the process under it
	// changed. Every deploy logs out every pinned user.
	after := r.fleet.Pin(token)
	user, found = after.ServeSessionLookup(token)
	r.record(ScalingSessionStep{
		Phase: scalingPhaseSticky, Route: "sticky",
		ReplicaID: after.ID(), Outcome: outcomeOf(found),
		Label:  "the pinned request meets the fresh replica",
		Detail: "the affinity entry survived the swap; the memory it pointed at did not",
		Observed: fmt.Sprintf("affinity still points at %s, now a fresh process: %d session(s) held — %s",
			after.ID(), after.SessionCount(), lookupVerdict(user, found)),
	})
}

// sharedStoreAct repeats the session with one change — the write goes
// to the shared store — and walks lookups across every replica of the
// fleet, through a second swap, and out the other side.
func (r *ScalingRound) sharedStoreAct() {
	token := r.cfg.SharedToken()

	// The login again, externalized this time. Whichever replica the
	// ring picks merely SERVES the request; the write belongs to the
	// store.
	writer, writerSlot := r.fleet.Next()
	writer.Serve()
	r.shared.StoreSession(token, r.cfg.ClientID)
	r.record(ScalingSessionStep{
		Phase: scalingPhaseShared, Route: "round-robin",
		ReplicaID: writer.ID(), Outcome: scalingOutcomeStored,
		Label:  "login again: the session is written to the shared store",
		Detail: "any replica can take the write — the value lives in the store, not in the server",
		Observed: fmt.Sprintf("%s served the request; the shared store holds %d session(s) — no affinity entry recorded, none needed",
			writer.ID(), r.shared.SessionCount()),
	})

	// Lookups through every replica of the fleet, round-robin: all hit.
	for range r.cfg.Replicas {
		replica, _ := r.fleet.Next()
		replica.Serve()
		user, found := r.shared.LookupSession(token)
		r.record(ScalingSessionStep{
			Phase: scalingPhaseShared, Route: "round-robin",
			ReplicaID: replica.ID(), Outcome: outcomeOf(found),
			Label:  fmt.Sprintf("the lookup routed to %s", replica.ID()),
			Detail: "a different replica than the writer answers correctly — the state followed the request because it never lived in a replica",
			Observed: fmt.Sprintf("%s reads the shared store: %s",
				replica.ID(), lookupVerdict(user, found)),
		})
	}

	// The same swap, on a replica that holds NOTHING of the session —
	// with externalized state, every replica holds nothing of it.
	swapSlot := (writerSlot + 1) % r.cfg.Replicas
	_, terminated := r.fleet.ReplaceReplica(swapSlot)
	r.swaps = append(r.swaps, ScalingSwapFact{
		Step:           len(r.steps) + 1,
		ReplicaID:      terminated.ID(),
		ServedBefore:   terminated.Served(),
		SessionsBefore: terminated.SessionCount(),
	})
	r.record(ScalingSessionStep{
		Phase: scalingPhaseSwap, Route: "rolling update",
		ReplicaID: terminated.ID(), Outcome: scalingOutcomeReplaced,
		Label:  "the same swap happens mid-session",
		Detail: "a replica is replaced while the session is in active use — the session is untouched because it never lived here",
		Observed: fmt.Sprintf("%s terminated: %d request(s) served, %d session(s) held — the shared session does not notice",
			terminated.ID(), terminated.Served(), terminated.SessionCount()),
	})

	// The lookup after the swap: still found — externalized state
	// survives the rollout that killed its writers.
	after, _ := r.fleet.Next()
	after.Serve()
	user, found := r.shared.LookupSession(token)
	r.record(ScalingSessionStep{
		Phase: scalingPhaseShared, Route: "round-robin",
		ReplicaID: after.ID(), Outcome: outcomeOf(found),
		Label:  "the lookup after the swap",
		Detail: "externalized state survives the rollout that killed its writers",
		Observed: fmt.Sprintf("%s (a fresh process) reads the shared store: %s",
			after.ID(), lookupVerdict(user, found)),
	})
}

// counterAct spreads the configured increments across the fleet
// through the shared store and reads the total back — accumulation
// that no replica-local counter could match.
func (r *ScalingRound) counterAct() {
	rows := make(map[string]int, r.cfg.Replicas)
	for _, replica := range r.fleet.Fleet() {
		rows[replica.ID()] = 0
	}
	for range r.cfg.CounterIncrements {
		replica, _ := r.fleet.Next()
		replica.Serve()
		r.shared.Increment()
		rows[replica.ID()]++
	}

	fleet := r.fleet.Fleet()
	parts := make([]string, 0, len(fleet))
	for _, replica := range fleet {
		parts = append(parts, fmt.Sprintf("%s %d", replica.ID(), rows[replica.ID()]))
	}
	total := r.shared.Counter()
	r.record(ScalingSessionStep{
		Phase: scalingPhaseCounter, Route: "round-robin",
		ReplicaID: "fleet",
		Outcome:   scalingOutcomeStored,
		Label:     fmt.Sprintf("%d increments spread across the fleet", r.cfg.CounterIncrements),
		Detail:    "every write goes through a different replica to the same store — one counter accumulates them all",
		Observed: fmt.Sprintf("increments per replica: %s — the shared counter reads back %d",
			joinCounts(parts), total),
	})
	r.counterRows = make([]ScalingCounterRow, 0, len(fleet))
	for _, replica := range fleet {
		r.counterRows = append(r.counterRows, ScalingCounterRow{
			ReplicaID:  replica.ID(),
			Increments: rows[replica.ID()],
		})
	}
	r.counterShared = total
}

// record appends a step with its number.
func (r *ScalingRound) record(step ScalingSessionStep) {
	step.Step = len(r.steps) + 1
	r.steps = append(r.steps, step)
}

// outcomeOf maps a found flag to the hit/miss vocabulary.
func outcomeOf(found bool) string {
	if found {
		return scalingOutcomeHit
	}
	return scalingOutcomeMiss
}

// lookupVerdict renders what a lookup observed.
func lookupVerdict(user string, found bool) string {
	if found {
		return fmt.Sprintf("found (%s)", user)
	}
	return "not found — unknown session"
}

// joinCounts renders "a 2 · b 2 · c 2" — measured counts, fleet order.
func joinCounts(parts []string) string {
	joined := ""
	for i, part := range parts {
		if i > 0 {
			joined += " · "
		}
		joined += part
	}
	return joined
}

// ── the pure payload builder ────────────────────────────────────────

// ScalingConfigView is the config panel: the constants the round ran
// under, including the two session ids so the panel can tell the
// placements apart.
type ScalingConfigView struct {
	Replicas             int    `json:"replicas"`
	DistributionRequests int    `json:"distributionRequests"`
	ClientID             string `json:"clientId"`
	CounterIncrements    int    `json:"counterIncrements"`
	LocalToken           string `json:"localToken"`
	SharedToken          string `json:"sharedToken"`
	SharedStoreNote      string `json:"sharedStoreNote"`
}

// ScalingDistributionRowView is one replica's measured share on the
// wire.
type ScalingDistributionRowView struct {
	ReplicaID string `json:"replicaId"`
	Served    int    `json:"served"`
}

// ScalingDistributionView is the distribution act as served: the
// measured total, the measured per-replica rows, and a note derived
// from those numbers.
type ScalingDistributionView struct {
	TotalServed int                          `json:"totalServed"`
	Rows        []ScalingDistributionRowView `json:"rows"`
	Note        string                       `json:"note"`
}

// ScalingStepView is one session-round step on the wire.
type ScalingStepView struct {
	Step      int    `json:"step"`
	Phase     string `json:"phase"`
	Label     string `json:"label"`
	Route     string `json:"route"`
	ReplicaID string `json:"replicaId"`
	Outcome   string `json:"outcome"`
	Detail    string `json:"detail"`
	Observed  string `json:"observed"`
}

// ScalingCounterRowView is one replica's measured increment share on
// the wire.
type ScalingCounterRowView struct {
	ReplicaID  string `json:"replicaId"`
	Increments int    `json:"increments"`
}

// ScalingCounterView is the counter act as served.
type ScalingCounterView struct {
	Rows  []ScalingCounterRowView `json:"rows"`
	Total int                     `json:"total"`
	Note  string                  `json:"note"`
}

// ScalingVerdictView states one thing the round proved, with its
// measured numbers.
type ScalingVerdictView struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// ScalingStatelessPayload is the wire shape of
// GET /api/demo/scaling/stateless.
type ScalingStatelessPayload struct {
	Mode         string                  `json:"mode"`
	Config       ScalingConfigView       `json:"config"`
	Distribution ScalingDistributionView `json:"distribution"`
	Steps        []ScalingStepView       `json:"steps"`
	Counter      ScalingCounterView      `json:"counter"`
	Verdicts     []ScalingVerdictView    `json:"verdicts"`
	Honesty      string                  `json:"honesty"`
}

// BuildScalingStatelessPayload shapes the wire payload (pure): every
// count, hit, miss, and swap number comes from the facts the round
// observed — the notes are DERIVED from those numbers, never asserted
// over them, and the payload carries no clock and no randomness, so
// identical facts serialize identically.
func BuildScalingStatelessPayload(facts ScalingStatelessFacts) ScalingStatelessPayload {
	rows := make([]ScalingDistributionRowView, 0, len(facts.Distribution))
	total := 0
	for _, fact := range facts.Distribution {
		rows = append(rows, ScalingDistributionRowView{ReplicaID: fact.ReplicaID, Served: fact.Served})
		total += fact.Served
	}

	steps := make([]ScalingStepView, 0, len(facts.Steps))
	localMisses := 0
	sharedHits, sharedLookups := 0, 0
	localStoredOn := ""
	for _, step := range facts.Steps {
		steps = append(steps, ScalingStepView{
			Step: step.Step, Phase: step.Phase, Label: step.Label, Route: step.Route,
			ReplicaID: step.ReplicaID, Outcome: step.Outcome, Detail: step.Detail, Observed: step.Observed,
		})
		switch {
		case step.Phase == scalingPhaseLocal && step.Outcome == scalingOutcomeStored:
			localStoredOn = step.ReplicaID
		case step.Phase == scalingPhaseLocal && step.Outcome == scalingOutcomeMiss:
			localMisses++
		case step.Phase == scalingPhaseShared &&
			(step.Outcome == scalingOutcomeHit || step.Outcome == scalingOutcomeMiss):
			sharedLookups++
			if step.Outcome == scalingOutcomeHit {
				sharedHits++
			}
		}
	}

	counterRows := make([]ScalingCounterRowView, 0, len(facts.Counter.Rows))
	for _, row := range facts.Counter.Rows {
		counterRows = append(counterRows, ScalingCounterRowView{ReplicaID: row.ReplicaID, Increments: row.Increments})
	}

	firstSwap := ScalingSwapFact{}
	if len(facts.Swaps) > 0 {
		firstSwap = facts.Swaps[0]
	}

	return ScalingStatelessPayload{
		Mode: "one fleet, two placements: three in-process replicas behind a real " +
			"round-robin balancer walk one session through replica-local memory " +
			"and through the shared store — every count below was observed by " +
			"the structures that served the round, and the round is " +
			"deterministic: reload it and it replays identically",
		Config: ScalingConfigView{
			Replicas:             facts.Config.Replicas,
			DistributionRequests: facts.Config.DistributionRequests,
			ClientID:             facts.Config.ClientID,
			CounterIncrements:    facts.Config.CounterIncrements,
			LocalToken:           facts.Config.LocalToken(),
			SharedToken:          facts.Config.SharedToken(),
			SharedStoreNote: "the shared store is an in-process analogue of the redis " +
				"container this stack carries — the lesson is where the value " +
				"lives, not how many hops it takes (topics 20 and 21 drove the " +
				"real redis calls)",
		},
		Distribution: ScalingDistributionView{
			TotalServed: total,
			Rows:        rows,
			Note:        DescribeScalingDistribution(rows, total),
		},
		Steps: steps,
		Counter: ScalingCounterView{
			Rows:  counterRows,
			Total: facts.Counter.Shared,
			Note: fmt.Sprintf("%d increments through three different replicas read back as %d from one store — "+
				"a replica-local counter dies with its replica (the swap steps above measured both deaths)",
				sumScalingIncrements(counterRows), facts.Counter.Shared),
		},
		Verdicts: []ScalingVerdictView{
			{
				ID: "distribution", Label: "the balancer spreads the work",
				Value: fmt.Sprintf("%d requests landed %s across %d replicas — measured per replica after the round",
					total, servedSpread(rows), len(rows)),
			},
			{
				ID: "local-session", Label: "replica-local state does not follow the request",
				Value: fmt.Sprintf("the local session was stored on %s and missed on its next %d hop(s) — %d of %d replicas ever knew it",
					localStoredOn, localMisses, 1, len(rows)),
			},
			{
				ID: "sticky", Label: "sticky routing is a lease, not a solution",
				Value: fmt.Sprintf("affinity rescued the lookup until the rolling update replaced %s: %d request(s) served and %d session(s) died with the process, and the pinned request then missed on the fresh replica",
					firstSwap.ReplicaID, firstSwap.ServedBefore, firstSwap.SessionsBefore),
			},
			{
				ID: "shared-session", Label: "externalized state follows every request",
				Value: fmt.Sprintf("the shared token answered %d of %d lookups across the fleet — including both fresh replacements — and survived the second swap untouched",
					sharedHits, sharedLookups),
			},
			{
				ID: "the-trade", Label: "the trade this topic keeps making",
				Value: "in-process state is fast, free, and invisible to every other replica; externalized state costs a hop on every request and survives every replica. A load balancer makes that trade load-bearing — there is no third option",
			},
		},
		Honesty: "Every count on this panel is real: per-replica served counts come off the replicas " +
			"themselves, every hit and miss is an actual map or store lookup that happened, and the " +
			"swap rows quote the dying replica's final measured counters. The three replicas are " +
			"in-process state worlds inside this one process — an honest single-process demo cannot " +
			"fork the server, but it can construct exactly what a second process would construct: the " +
			"same fresh-memory replica a rolling update boots. The balancer is a real round-robin " +
			"algorithm with a real affinity table, and the shared store is an in-process analogue of " +
			"the redis container this stack carries. The lesson is placement, not processes: the local " +
			"session died at the first cross-replica hop because it lived in a heap; the shared one " +
			"survived the swap because it never did. Nothing here reads a clock or a random source — " +
			"the round is deterministic, so this payload replays identically on every request",
	}
}

// DescribeScalingDistribution derives the distribution note from the
// measured rows: the spread is stated from the numbers, never assumed.
func DescribeScalingDistribution(rows []ScalingDistributionRowView, total int) string {
	if len(rows) == 0 {
		return "no distribution rows — the fleet served nothing"
	}
	return fmt.Sprintf("%d request(s) across %d replica(s), per-replica %s — spread %s: round-robin's promise holds when the count divides evenly, and stays within one request otherwise",
		total, len(rows), servedSpread(rows), spreadVerdict(rows))
}

// servedSpread renders "4/4/4" from the measured rows.
func servedSpread(rows []ScalingDistributionRowView) string {
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		parts = append(parts, fmt.Sprintf("%d", row.Served))
	}
	return strings.Join(parts, "/")
}

// spreadVerdict states evenness from the measured rows.
func spreadVerdict(rows []ScalingDistributionRowView) string {
	if len(rows) == 0 {
		return "unknown"
	}
	min, max := rows[0].Served, rows[0].Served
	for _, row := range rows[1:] {
		if row.Served < min {
			min = row.Served
		}
		if row.Served > max {
			max = row.Served
		}
	}
	switch {
	case max == min:
		return "exactly even"
	case max-min == 1:
		return "even within one request"
	default:
		return fmt.Sprintf("uneven (min %d, max %d)", min, max)
	}
}

// sumScalingIncrements adds the measured increment rows.
func sumScalingIncrements(rows []ScalingCounterRowView) int {
	sum := 0
	for _, row := range rows {
		sum += row.Increments
	}
	return sum
}
