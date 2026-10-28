// Topic 43 advanced-architecture's event-sourcing demo (arch:eventstore).
// The machinery is the REAL event store from internal/domain/patterns,
// driven through a complete stream lifecycle: one inventory item's
// stream receives its events through the command path event sourcing
// prescribes — rebuild by replay, decide against the projection, append
// naming the version the decision was made against. Two writers then
// race: the second writer's stale append meets the store's optimistic
// concurrency check and dies with the typed conflict, its recovery
// (reload, revalidate, retry with the fresh version) shows BOTH halves
// of the loop — the stale intent itself is refused by the fresh state,
// and a genuinely valid intent lands at the new head. Finally the
// stream is rebuilt from nothing twice: the two rebuilds must be
// byte-identical and must agree with an independently computed naive
// ledger, and a stream that never received an event folds to the zero
// state.
//
// Determinism rules: no wall clock and no randomness anywhere — event
// occurrence times are FIXED DATA at a fixed exhibit clock (the store
// never reads a clock; that is part of its design), every version,
// count, state, and conflict number was observed from the real store
// during the run that served it, and a mismatched expectation is a
// script bug that panics — never a mislabeled row. Two GETs serve
// byte-identical bodies.
package demos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"guide/internal/domain/patterns"
)

// ArchEventstorePath is the demo area's endpoint — registered on the
// real mux by the composition root.
const ArchEventstorePath = "/api/demo/arch/eventstore"

// archEventstoreClockBase is the fixed exhibit clock every event's
// occurrence time derives from: step N is recorded at the base plus
// (N-1) fixed 30-second ticks. Fixed strings, no wall clock.
const (
	archEventstoreClockBase = "2026-10-28T12:00:00Z"
	archEventstoreTick      = 30 * time.Second
)

// archEventstoreEventClock parses the fixed exhibit clock once and
// advances it by the step's ticks; every value is a constant away from
// a constant, so a parse failure is a programmer error in this file.
func archEventstoreEventClock(step int) time.Time {
	base, err := time.Parse(time.RFC3339, archEventstoreClockBase)
	if err != nil {
		panic("arch eventstore: exhibit clock " + archEventstoreClockBase + " must parse")
	}
	return base.Add(time.Duration(step-1) * archEventstoreTick)
}

// The fixed exhibit quantities: what the inventory item's story moves.
const (
	archEventstoreStreamID = "inventory-tee-medium"

	archEventstoreStockQty     = 50
	archEventstoreReserveAQty  = 20
	archEventstoreReserveBQty  = 25
	archEventstorePickAQty     = 20
	archEventstoreRefuseQty    = 10 // wants 10 with only 5 available
	archEventstoreReleaseBQty  = 25
	archEventstoreAbsentStream = "inventory-never-stocked"
)

// ── the aggregate's rebuild: the event-sourced fold ─────────────────

// archEventstoreFold is the inventory item's state AS REBUILT FROM
// EVENTS — the only state event sourcing keeps. Nothing here is ever
// stored: every value is derived by replaying the stream.
type archEventstoreFold struct {
	onHand       int64
	reservations map[string]int64 // order ref → quantity held open
}

// archEventstoreNewFold is the zero state every replay starts from.
func archEventstoreNewFold() archEventstoreFold {
	return archEventstoreFold{reservations: make(map[string]int64)}
}

// reservedTotal sums the open reservations.
func (f archEventstoreFold) reservedTotal() int64 {
	total := int64(0)
	for _, qty := range f.reservations {
		total += qty
	}
	return total
}

// archEventstorePayloadOf decodes one event's payload — the script
// appends only well-formed payloads, so a decode failure is a
// programmer bug in this file.
func archEventstorePayloadOf(payload string) (qty int64, ref string) {
	var decoded struct {
		Qty int64  `json:"qty"`
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		panic("arch eventstore: script payload " + payload + " must decode: " + err.Error())
	}
	return decoded.Qty, decoded.Ref
}

// archEventstoreApply is the fold: one event applied to the rebuilt
// state, in version order. The reservations map is COPIED before it is
// touched — apply must leave the state it was handed intact, or the
// second rebuild would start from a map the first rebuild corrupted.
// An unknown event type is a programmer bug (the fold must be total):
// panics are for programmer bugs only.
func archEventstoreApply(state archEventstoreFold, event patterns.EventRecord) archEventstoreFold {
	qty, ref := archEventstorePayloadOf(event.Payload)
	next := archEventstoreFold{onHand: state.onHand, reservations: make(map[string]int64, len(state.reservations))}
	for held, heldQty := range state.reservations {
		next.reservations[held] = heldQty
	}
	switch event.Type {
	case "inventory.stocked":
		next.onHand += qty
	case "inventory.reserved":
		next.reservations[ref] += qty
	case "inventory.released":
		next.reservations[ref] -= qty
	case "inventory.picked":
		next.reservations[ref] -= qty
		next.onHand -= qty
	default:
		panic("arch eventstore: the fold met unknown event type " + event.Type + " — the fold must be total")
	}
	if next.reservations[ref] == 0 {
		delete(next.reservations, ref)
	}
	return next
}

// ArchEventstoreInventoryState is the rebuilt state as served: the two
// stored facts of the fold, the derived availability, and the open
// reservations per order ref.
type ArchEventstoreInventoryState struct {
	OnHand       int64            `json:"onHand"`
	Reserved     int64            `json:"reserved"`
	Available    int64            `json:"available"`
	Reservations map[string]int64 `json:"reservations"`
}

// archEventstoreStateOf shapes the fold as served.
func archEventstoreStateOf(fold archEventstoreFold) ArchEventstoreInventoryState {
	reservations := make(map[string]int64, len(fold.reservations))
	for ref, qty := range fold.reservations {
		reservations[ref] = qty
	}
	reserved := fold.reservedTotal()
	return ArchEventstoreInventoryState{
		OnHand:       fold.onHand,
		Reserved:     reserved,
		Available:    fold.onHand - reserved,
		Reservations: reservations,
	}
}

// archEventstoreFormatState renders one state for a row's detail line.
func archEventstoreFormatState(state ArchEventstoreInventoryState) string {
	refs := make([]string, 0, len(state.Reservations))
	for ref := range state.Reservations {
		refs = append(refs, fmt.Sprintf("%s:%d", ref, state.Reservations[ref]))
	}
	sort.Strings(refs)
	open := "{}"
	if len(refs) > 0 {
		open = "{" + strings.Join(refs, ", ") + "}"
	}
	return fmt.Sprintf("onHand %d · reserved %d · available %d · open %s",
		state.OnHand, state.Reserved, state.Available, open)
}

// ── the command path: rebuild → decide → append ─────────────────────

// archEventstoreDecision is one command's verdict against one
// projection: either the event to append, or the refusal. A refusal is
// NOT an event — the aggregate refuses before the store is ever
// touched, so refused commands leave no trace in history.
type archEventstoreDecision struct {
	eventType string // empty when refused
	payload   string
	refusal   string
}

// archEventstoreDecide applies the aggregate's rules for one command
// kind against one view of the stream. The rules are ordinary domain
// logic; what makes them event-sourced is WHERE the view came from —
// always a replay, never a stored row.
func archEventstoreDecide(kind string, qty int64, ref string, view archEventstoreFold) archEventstoreDecision {
	switch kind {
	case "stock":
		if qty <= 0 {
			return archEventstoreDecision{refusal: fmt.Sprintf("a stock of %d is not a quantity", qty)}
		}
		return archEventstoreDecision{eventType: "inventory.stocked", payload: fmt.Sprintf(`{"qty":%d}`, qty)}
	case "reserve":
		if qty <= 0 {
			return archEventstoreDecision{refusal: fmt.Sprintf("a reservation of %d is not a quantity", qty)}
		}
		if available := view.onHand - view.reservedTotal(); qty > available {
			return archEventstoreDecision{refusal: fmt.Sprintf("%d wanted, %d available — reserving more than the shelf can promise is refused", qty, available)}
		}
		return archEventstoreDecision{eventType: "inventory.reserved", payload: fmt.Sprintf(`{"qty":%d,"ref":%q}`, qty, ref)}
	case "release":
		if held := view.reservations[ref]; held < qty {
			return archEventstoreDecision{refusal: fmt.Sprintf("%s holds no reservation of %d (it holds %d) — releasing what was never reserved is refused", ref, qty, held)}
		}
		return archEventstoreDecision{eventType: "inventory.released", payload: fmt.Sprintf(`{"qty":%d,"ref":%q}`, qty, ref)}
	case "pick":
		if held := view.reservations[ref]; held < qty {
			return archEventstoreDecision{refusal: fmt.Sprintf("%s holds no reservation of %d (it holds %d) — picking ahead of a reservation is refused", ref, qty, held)}
		}
		return archEventstoreDecision{eventType: "inventory.picked", payload: fmt.Sprintf(`{"qty":%d,"ref":%q}`, qty, ref)}
	default:
		panic("arch eventstore: unknown command kind " + kind)
	}
}

// archEventstoreRunCommand is the full safe loop for one writer acting
// on FRESH history: rebuild the state by replay, decide against it,
// append naming the version the decision was made against. Every part
// of the verdict is observed, never assumed.
type archEventstoreCommandResult struct {
	accepted    bool
	version     int64 // stream version after the attempt (observed)
	eventType   string
	refusal     string
	conflict    *patterns.EventStoreConflict
	stateDetail string // the state after the attempt, when it appended
}

func archEventstoreRunCommand(ctx context.Context, store *patterns.EventStore, step int,
	kind string, qty int64, ref string) archEventstoreCommandResult {
	projection, err := patterns.EventProject(ctx, store, archEventstoreStreamID, archEventstoreNewFold(), archEventstoreApply)
	if err != nil {
		panic("arch eventstore: the command path's projection failed: " + err.Error())
	}
	decision := archEventstoreDecide(kind, qty, ref, projection.State)
	if decision.refusal != "" {
		return archEventstoreCommandResult{version: projection.Version, refusal: decision.refusal}
	}
	records, err := store.Append(ctx, archEventstoreStreamID, projection.Version, patterns.EventAppend{
		Type:       decision.eventType,
		Payload:    []byte(decision.payload),
		RecordedAt: archEventstoreEventClock(step),
	})
	var conflict *patterns.EventStoreConflict
	if errors.As(err, &conflict) {
		return archEventstoreCommandResult{version: conflict.ActualVersion, eventType: decision.eventType, conflict: conflict}
	}
	if err != nil {
		panic("arch eventstore: the append failed outside the conflict path: " + err.Error())
	}
	after, err := patterns.EventProject(ctx, store, archEventstoreStreamID, archEventstoreNewFold(), archEventstoreApply)
	if err != nil {
		panic("arch eventstore: the post-append projection failed: " + err.Error())
	}
	return archEventstoreCommandResult{
		accepted:    true,
		version:     records[len(records)-1].Version,
		eventType:   decision.eventType,
		stateDetail: archEventstoreFormatState(archEventstoreStateOf(after.State)),
	}
}

// ── the observed-run shapes ─────────────────────────────────────────

// ArchEventstoreRow is one observed step of the stream's story: what
// was attempted, with what input, what the machinery answered, and the
// stream version AFTER the step (always observed, 0 included — version
// 0 is real data: nothing has happened yet). ExpectedVersion and
// ActualVersion are carried only by the conflict step.
type ArchEventstoreRow struct {
	Step            int    `json:"step"`
	Call            string `json:"call"`
	Input           string `json:"input"`
	Outcome         string `json:"outcome"`
	Version         int64  `json:"version"`
	ExpectedVersion int64  `json:"expectedVersion,omitempty"`
	ActualVersion   int64  `json:"actualVersion,omitempty"`
	Detail          string `json:"detail"`
}

// ArchEventstoreCount is one counted observation — counted during the
// run, never asserted.
type ArchEventstoreCount struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

// ArchEventstorePanel is one panel as served: its framing, its rules,
// its observed rows, and its counted totals.
type ArchEventstorePanel struct {
	ID     string                `json:"id"`
	Title  string                `json:"title"`
	Lead   string                `json:"lead"`
	Config []string              `json:"config"`
	Rows   []ArchEventstoreRow   `json:"rows"`
	Counts []ArchEventstoreCount `json:"counts"`
}

// ArchEventstoreReplayEvidence is the rebuild's proof block: the
// events the fold consumed in order, the state it built, the naive
// ledger computed independently of the fold, the byte-identical double
// rebuild, and the absent stream's zero-state fold.
type ArchEventstoreReplayEvidence struct {
	FoldedEvents   int64                        `json:"foldedEvents"`
	AppliedInOrder []string                     `json:"appliedInOrder"`
	State          ArchEventstoreInventoryState `json:"state"`
	NaiveExpected  ArchEventstoreInventoryState `json:"naiveExpected"`
	RebuildsRun    int                          `json:"rebuildsRun"`
	ByteIdentical  bool                         `json:"byteIdentical"`
	AbsentStreamID string                       `json:"absentStreamId"`
	AbsentState    ArchEventstoreInventoryState `json:"absentState"`
	AbsentVersion  int64                        `json:"absentVersion"`
	Note           string                       `json:"note"`
}

// ArchEventstorePayload is the wire shape of
// GET /api/demo/arch/eventstore (camelCase JSON).
type ArchEventstorePayload struct {
	Mode        string                       `json:"mode"`
	HonestyNote string                       `json:"honestyNote"`
	StreamID    string                       `json:"streamId"`
	Store       []string                     `json:"store"`
	Panels      []ArchEventstorePanel        `json:"panels"`
	Replay      ArchEventstoreReplayEvidence `json:"replay"`
}

// BuildArchEventstorePayload drives the whole exhibit (pure over
// fixed inputs): a fresh REAL event store, the fixed three-act script
// against it, and every row shaped from what the machinery actually
// answered. No wall clock, no randomness — every GET serves identical
// bytes.
func BuildArchEventstorePayload() ArchEventstorePayload {
	ctx := context.Background()
	store := patterns.NewEventStore()

	lifecycle := archEventstoreRunLifecycleAct(ctx, store)
	safety := archEventstoreRunSafetyAct(ctx, store)
	replay := archEventstoreRunReplayAct(ctx, store)

	return ArchEventstorePayload{
		Mode: "event store lifecycle: one inventory stream driven through the real " +
			"internal/domain/patterns EventStore — commands rebuild by replay, decide against the projection, " +
			"and append naming their expected version; a second writer's stale append meets the typed conflict " +
			"and recovers by reloading; the stream is then rebuilt from nothing twice, byte-identically, and " +
			"checked against an independently computed ledger",
		HonestyNote: "No-invention pin: every version, count, state, and conflict number on this payload was " +
			"observed from the real in-process store during the run that served it. Event occurrence times are " +
			"FIXED DATA at the fixed exhibit clock " + archEventstoreClockBase + " (the store never reads a wall " +
			"clock — that is part of its design, and it is why two GETs serve byte-identical bodies). The " +
			"double-rebuild comparison ran encoding/json and compared the bytes; a drift would have panicked " +
			"this builder instead of serving a claim",
		StreamID: archEventstoreStreamID,
		Store: []string{
			"EventStore.Append(ctx, streamID, expectedVersion, events...) — the ONLY write path: batch-atomic, versions assigned 1-based by the store",
			"EventStore.Stream(ctx, streamID) — the replay source; an absent stream answers an EMPTY slice (no 'missing' state, only zero events)",
			"EventStore.Version(ctx, streamID) — the stream's head, i.e. the expectedVersion the next append may claim",
			"EventProject(ctx, store, streamID, initial, apply) → EventProjection{State, Version} — the rebuild, with the version the state provably reflects (a free generic function: Go methods cannot introduce type parameters)",
			"EventFold(events, initial, apply) — replay as a pure fold over the records in version order",
			"*EventStoreConflict{StreamID, ExpectedVersion, ActualVersion} — the typed optimistic-concurrency failure; EventStreamVersionEmpty=0 creates, EventStreamVersionAny=-1 skips the check",
			"no update, no delete: payloads are frozen into strings at append time and Stream returns copies — history is structurally immutable",
		},
		Panels: []ArchEventstorePanel{lifecycle, safety, replay.panel},
		Replay: replay.evidence,
	}
}

// ── act one: the lifecycle — one writer, four events ────────────────

// archEventstoreRunLifecycleAct drives writer one: stock the item,
// reserve for two orders. Every append runs the full command path and
// every row observes the store's own answer.
func archEventstoreRunLifecycleAct(ctx context.Context, store *patterns.EventStore) ArchEventstorePanel {
	rows := []ArchEventstoreRow{}
	counts := map[string]int{}
	step := 0

	run := func(call, input, kind string, qty int64, ref string) {
		step++
		result := archEventstoreRunCommand(ctx, store, step, kind, qty, ref)
		row := ArchEventstoreRow{Step: step, Call: call, Input: input, Version: result.version}
		switch {
		case result.conflict != nil:
			panic("arch eventstore: the lifecycle act runs alone — a conflict here is a script bug")
		case !result.accepted:
			row.Outcome = "refused-by-aggregate"
			row.Detail = result.refusal
		default:
			row.Outcome = "appended"
			row.Detail = fmt.Sprintf("%s landed as v%d; replay now says %s", result.eventType, result.version, result.stateDetail)
			counts["appends accepted"]++
			counts["events stored"]++
		}
		rows = append(rows, row)
	}

	run("Append (stock)", fmt.Sprintf("expected=EventStreamVersionEmpty (0), 1 event: inventory.stocked qty=%d", archEventstoreStockQty),
		"stock", archEventstoreStockQty, "")
	run("Append (reserve)", fmt.Sprintf("expected=1 (the projection's version), inventory.reserved qty=%d ref=order-A", archEventstoreReserveAQty),
		"reserve", archEventstoreReserveAQty, "order-A")
	run("Append (reserve)", fmt.Sprintf("expected=2, inventory.reserved qty=%d ref=order-B", archEventstoreReserveBQty),
		"reserve", archEventstoreReserveBQty, "order-B")

	if rows[len(rows)-1].Version != 3 {
		panic("arch eventstore: the lifecycle act must end at version 3 — the safety act's stale writer depends on it")
	}

	return ArchEventstorePanel{
		ID:    "lifecycle",
		Title: "Act one — one writer, the command path",
		Lead: "Every command rebuilds the item's state by replaying the stream, decides against that projection, " +
			"and appends naming the version it decided against. The state is never stored — it is derived, every time.",
		Config: []string{
			"the loop: Project (replay) → decide → Append(expected = projection.Version)",
			"versions: 1-based, dense, assigned by the store — a caller cannot forge one",
			fmt.Sprintf("the stream: %q; the aggregate: an inventory item with on-hand stock and per-order reservations", archEventstoreStreamID),
		},
		Rows:   rows,
		Counts: archEventstoreSortedCounts(counts),
	}
}

// ── act two: safety — two writers, one conflict, one recovery ───────

// archEventstoreRunSafetyAct drives the collision: a second writer
// loads the stream at version 3 and decides against that view; writer
// one picks order-A's reservation (v4) and a fresh reserve command is
// refused by the aggregate's own rules; the second writer's stale
// append then meets the typed conflict, its stale intent is refused by
// the reloaded state, and a genuinely valid intent lands at the new
// head.
func archEventstoreRunSafetyAct(ctx context.Context, store *patterns.EventStore) ArchEventstorePanel {
	rows := []ArchEventstoreRow{}
	counts := map[string]int{}
	step := 3 // the lifecycle act's rows own steps 1-3

	// The second writer loads the stream — an honest snapshot at v3.
	step++
	loaded, err := patterns.EventProject(ctx, store, archEventstoreStreamID, archEventstoreNewFold(), archEventstoreApply)
	if err != nil {
		panic("arch eventstore: the second writer's load failed: " + err.Error())
	}
	if loaded.Version != 3 {
		panic("arch eventstore: the second writer must load at v3 — the script's contract")
	}
	loadedDetail := archEventstoreFormatState(archEventstoreStateOf(loaded.State))
	rows = append(rows, ArchEventstoreRow{
		Step: step, Call: "Project (writer two loads)", Outcome: "loaded", Version: loaded.Version,
		Input:  "Stream → 3 events, folded from the zero state — the second writer's snapshot",
		Detail: fmt.Sprintf("loaded v%d; its view says %s", loaded.Version, loadedDetail),
	})
	counts["streams loaded by the second writer"] = 1

	// Writer one picks order-A's reservation — the event writer two has
	// not seen. The command path runs against FRESH history.
	step++
	pick := archEventstoreRunCommand(ctx, store, step, "pick", archEventstorePickAQty, "order-A")
	if !pick.accepted || pick.version != 4 {
		panic("arch eventstore: the pick must land as v4 — the script's contract")
	}
	rows = append(rows, ArchEventstoreRow{
		Step: step, Call: "Append (pick)", Outcome: "appended", Version: pick.version,
		Input:  fmt.Sprintf("expected=3, inventory.picked qty=%d ref=order-A — writer one, fresh history", archEventstorePickAQty),
		Detail: fmt.Sprintf("inventory.picked landed as v%d; replay now says %s", pick.version, pick.stateDetail),
	})
	counts["appends accepted"]++
	counts["events stored"]++

	// A fresh reserve that the aggregate's own rules refuse: 10 wanted,
	// 5 available. The refusal happens BEFORE the store is touched — no
	// event, no version movement.
	step++
	refused := archEventstoreRunCommand(ctx, store, step, "reserve", archEventstoreRefuseQty, "order-C")
	if refused.accepted || refused.version != 4 {
		panic("arch eventstore: the over-reserve must be refused without touching the stream")
	}
	rows = append(rows, ArchEventstoreRow{
		Step: step, Call: "reserve (decide only)", Outcome: "refused-by-aggregate", Version: refused.version,
		Input:  fmt.Sprintf("a command wanting to reserve qty=%d ref=order-C against the fresh projection", archEventstoreRefuseQty),
		Detail: refused.refusal + " — the command is refused before the store is touched: refusals are not events, history keeps no trace of them",
	})
	counts["commands refused by the aggregate (no event written)"] = 1

	// The second writer appends its decision — made against the v3
	// view — naming expected=3. The stream is at 4: the typed conflict.
	step++
	staleDecision := archEventstoreDecide("release", archEventstorePickAQty, "order-A", loaded.State)
	if staleDecision.refusal != "" {
		panic("arch eventstore: releasing order-A must be valid against the v3 view — the script's contract")
	}
	_, err = store.Append(ctx, archEventstoreStreamID, loaded.Version, patterns.EventAppend{
		Type:       staleDecision.eventType,
		Payload:    []byte(staleDecision.payload),
		RecordedAt: archEventstoreEventClock(step),
	})
	var conflict *patterns.EventStoreConflict
	if !errors.As(err, &conflict) || conflict.ExpectedVersion != 3 || conflict.ActualVersion != 4 {
		panic("arch eventstore: the stale append must meet the typed conflict (expected 3, actual 4)")
	}
	rows = append(rows, ArchEventstoreRow{
		Step: step, Call: "Append (writer two, stale)", Outcome: "refused-conflict", Version: conflict.ActualVersion,
		Input: fmt.Sprintf("expected=%d (its snapshot), inventory.released qty=%d ref=order-A — decided against the v3 view",
			loaded.Version, archEventstorePickAQty),
		ExpectedVersion: conflict.ExpectedVersion,
		ActualVersion:   conflict.ActualVersion,
		Detail: fmt.Sprintf("*EventStoreConflict observed via errors.As: expected %d, actual %d — the stream gained %d event(s) this writer never saw; nothing landed",
			conflict.ExpectedVersion, conflict.ActualVersion, conflict.ActualVersion-conflict.ExpectedVersion),
	})
	counts["optimistic conflicts (typed) observed"] = 1
	counts["events landed by stale appends"] = 0

	// Recovery, both halves. First: reload and revalidate the ORIGINAL
	// intent — the fresh state refuses it (order-A holds nothing; its
	// reservation was picked at v4). The conflict did not just delay a
	// bad write; it saved the ledger from a double release.
	step++
	reloaded := archEventstoreReload(ctx, store)
	recheck := archEventstoreDecide("release", archEventstorePickAQty, "order-A", reloaded.State)
	if recheck.refusal == "" {
		panic("arch eventstore: revalidating the stale intent against fresh history must refuse it")
	}
	rows = append(rows, ArchEventstoreRow{
		Step: step, Call: "Project + decide (revalidate)", Outcome: "refused-by-aggregate", Version: reloaded.Version,
		Input:  "the same release order-A intent, re-decided against the RELOADED state",
		Detail: recheck.refusal + " — the conflict saved the ledger: accepting the stale append would have released a reservation the pick already consumed",
	})
	counts["commands refused by the aggregate (no event written)"]++

	// Second half: a genuinely valid intent against fresh history —
	// release order-B's reservation at the true head.
	step++
	recovery := archEventstoreRunCommand(ctx, store, step, "release", archEventstoreReleaseBQty, "order-B")
	if !recovery.accepted || recovery.version != 5 {
		panic("arch eventstore: the recovery release must land as v5 — the script's contract")
	}
	rows = append(rows, ArchEventstoreRow{
		Step: step, Call: "Append (writer two, recovered)", Outcome: "appended-after-reload", Version: recovery.version,
		Input: fmt.Sprintf("expected=4 (the reloaded head), inventory.released qty=%d ref=order-B",
			archEventstoreReleaseBQty),
		Detail: fmt.Sprintf("inventory.released landed as v%d; replay now says %s", recovery.version, recovery.stateDetail),
	})
	counts["recovery appends accepted after reload"] = 1
	counts["appends accepted"]++
	counts["events stored"]++

	return ArchEventstorePanel{
		ID:    "safety",
		Title: "Act two — two writers, the conflict, the recovery",
		Lead: "Optimistic concurrency is the guardrail between a read and its follow-up write: the append must name " +
			"the head it decided against, and if history moved on, the store refuses with a typed error instead of " +
			"letting the stale decision through.",
		Config: []string{
			"expected-version: EventStreamVersionEmpty (0) creates a stream; any other value must equal the head; EventStreamVersionAny (-1) skips the check",
			"a refused append lands nothing — the whole batch is one transaction, checked and written under one lock",
			"recovery is the loop the conflict's own message teaches: reload, revalidate the intent against fresh history, retry with the fresh version",
		},
		Rows:   rows,
		Counts: archEventstoreSortedCounts(counts),
	}
}

// archEventstoreReload projects the stream fresh — the reload step of
// the recovery loop.
func archEventstoreReload(ctx context.Context, store *patterns.EventStore) patterns.EventProjection[archEventstoreFold] {
	projection, err := patterns.EventProject(ctx, store, archEventstoreStreamID, archEventstoreNewFold(), archEventstoreApply)
	if err != nil {
		panic("arch eventstore: the reload projection failed: " + err.Error())
	}
	return projection
}

// ── act three: replay — the rebuild proofs ──────────────────────────

// archEventstoreReplayAct carries the replay panel and its evidence.
type archEventstoreReplayAct struct {
	panel    ArchEventstorePanel
	evidence ArchEventstoreReplayEvidence
}

// archEventstoreRunReplayAct proves the rebuild: fold the whole stream
// from the zero state, do it again and compare the marshaled bytes,
// check the result against a naive ledger computed by plain arithmetic
// with no fold involved, and fold a stream that never received an
// event.
func archEventstoreRunReplayAct(ctx context.Context, store *patterns.EventStore) archEventstoreReplayAct {
	rows := []ArchEventstoreRow{}
	counts := map[string]int{}
	step := 9 // the safety act's rows own steps 4-9

	records, err := store.Stream(ctx, archEventstoreStreamID)
	if err != nil {
		panic("arch eventstore: the replay act's stream read failed: " + err.Error())
	}
	applied := make([]string, 0, len(records))
	for _, record := range records {
		applied = append(applied, fmt.Sprintf("v%d %s %s@%s", record.Version, record.Type,
			record.Payload, record.RecordedAt.Format(time.RFC3339)))
	}

	// Rebuild one: the fold from the zero state.
	step++
	first, err := patterns.EventProject(ctx, store, archEventstoreStreamID, archEventstoreNewFold(), archEventstoreApply)
	if err != nil {
		panic("arch eventstore: rebuild one failed: " + err.Error())
	}
	firstJSON, err := json.Marshal(archEventstoreStateOf(first.State))
	if err != nil {
		panic("arch eventstore: rebuild one must marshal: " + err.Error())
	}
	rows = append(rows, ArchEventstoreRow{
		Step: step, Call: "Project (rebuild)", Outcome: "rebuilt", Version: first.Version,
		Input:  fmt.Sprintf("%d events folded from the zero state, in version order", len(records)),
		Detail: fmt.Sprintf("%s — state is a DERIVATION: nothing like it was ever stored", archEventstoreFormatState(archEventstoreStateOf(first.State))),
	})
	counts["rebuilds run"] = 1

	// Rebuild two: byte-identical, or the whole discipline collapses.
	step++
	second, err := patterns.EventProject(ctx, store, archEventstoreStreamID, archEventstoreNewFold(), archEventstoreApply)
	if err != nil {
		panic("arch eventstore: rebuild two failed: " + err.Error())
	}
	secondJSON, err := json.Marshal(archEventstoreStateOf(second.State))
	if err != nil {
		panic("arch eventstore: rebuild two must marshal: " + err.Error())
	}
	byteIdentical := bytes.Equal(firstJSON, secondJSON)
	if !byteIdentical || second.Version != first.Version {
		panic("arch eventstore: the double rebuild must be byte-identical at the same version — drift is a bug, never a claim")
	}
	rows = append(rows, ArchEventstoreRow{
		Step: step, Call: "Project (rebuild again)", Outcome: "byte-identical", Version: second.Version,
		Input:  "the same fold, run again from the same zero state",
		Detail: fmt.Sprintf("encoding/json compared the bytes: identical=%t (%d bytes both times) — replay is deterministic or nothing holds", byteIdentical, len(firstJSON)),
	})
	counts["rebuilds run"]++
	counts["byte-identical rebuild pairs"] = 1

	// The naive ledger: plain arithmetic over the exhibit's raw
	// quantities, computed with NO fold, NO store, NO replay.
	// order-A: +20 reserved, −20 picked → gone; order-B: +25 reserved,
	// −25 released → gone; on hand: 50 stocked − 20 picked.
	naive := ArchEventstoreInventoryState{
		OnHand:       archEventstoreStockQty - archEventstorePickAQty,
		Reserved:     0,
		Available:    archEventstoreStockQty - archEventstorePickAQty,
		Reservations: map[string]int64{},
	}
	step++
	if !reflect.DeepEqual(archEventstoreStateOf(first.State), naive) || first.Version != 5 {
		panic("arch eventstore: the rebuilt state must equal the naive ledger at version 5")
	}
	rows = append(rows, ArchEventstoreRow{
		Step: step, Call: "naive ledger (independent)", Outcome: "matches-replay", Version: first.Version,
		Input:  "plain arithmetic over the exhibit's raw quantities — no fold, no store, no replay",
		Detail: fmt.Sprintf("50 stocked − 20 picked = 30 on hand; order-A 20−20 gone, order-B 25−25 gone → %s — and the replay agrees", archEventstoreFormatState(naive)),
	})
	counts["projection vs naive ledger agreement"] = 1

	// The absent stream: folds to the zero state at version 0.
	step++
	absent, err := patterns.EventProject(ctx, store, archEventstoreAbsentStream, archEventstoreNewFold(), archEventstoreApply)
	if err != nil {
		panic("arch eventstore: the absent-stream projection failed: " + err.Error())
	}
	absentState := archEventstoreStateOf(absent.State)
	if absent.Version != 0 || absentState.OnHand != 0 || absentState.Reserved != 0 || len(absentState.Reservations) != 0 {
		panic("arch eventstore: an absent stream must fold to the untouched zero state at version 0")
	}
	rows = append(rows, ArchEventstoreRow{
		Step: step, Call: "Project (absent stream)", Outcome: "zero-state-fold", Version: absent.Version,
		Input:  fmt.Sprintf("stream %q — never appended to", archEventstoreAbsentStream),
		Detail: "no 'missing' error, no nil: an absent stream and an empty stream both fold to the zero state — event sourcing has no 'does not exist', only 'no events yet'",
	})
	counts["absent-stream zero-state folds"] = 1

	return archEventstoreReplayAct{
		panel: ArchEventstorePanel{
			ID:    "replay",
			Title: "Act three — the rebuild proofs",
			Lead: "The state was never stored anywhere; these rows rebuild it from the events alone, prove the " +
				"rebuild is deterministic by running it twice, and cross-check it against arithmetic that never " +
				"touched the fold.",
			Config: []string{
				"replay = EventFold over the records in version order — occurrence times are data, never ordering",
				"determinism: same events + same zero state + same fold → byte-identical state, or the builder panics",
				"the zero-event fold: absent stream ⇒ initial state untouched, version 0",
			},
			Rows:   rows,
			Counts: archEventstoreSortedCounts(counts),
		},
		evidence: ArchEventstoreReplayEvidence{
			FoldedEvents:   int64(len(records)),
			AppliedInOrder: applied,
			State:          archEventstoreStateOf(first.State),
			NaiveExpected:  naive,
			RebuildsRun:    counts["rebuilds run"],
			ByteIdentical:  byteIdentical,
			AbsentStreamID: archEventstoreAbsentStream,
			AbsentState:    absentState,
			AbsentVersion:  absent.Version,
			Note: "the fold consumed the events above, in exactly that order, starting from the zero state; " +
				"the naive ledger was computed by hand-arithmetic over the same raw quantities and agreed exactly",
		},
	}
}

// archEventstoreSortedCounts renders counted observations as sorted
// rows — map iteration order never reaches the wire.
func archEventstoreSortedCounts(counts map[string]int) []ArchEventstoreCount {
	labels := make([]string, 0, len(counts))
	for label := range counts {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	rows := make([]ArchEventstoreCount, 0, len(labels))
	for _, label := range labels {
		rows = append(rows, ArchEventstoreCount{Label: label, Value: counts[label]})
	}
	return rows
}
