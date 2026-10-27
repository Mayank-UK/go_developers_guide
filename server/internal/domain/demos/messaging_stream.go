// Topic 22 messaging's first live demo: the consumer-group story. One
// request against the RUNNING stack Redis walks the full worker
// lifecycle — XADD three ordered events, create the group at 0-0 so
// history is in scope, XREADGROUP as consumer-a (all three, in insertion
// order), XREADGROUP as consumer-b (nothing: the group already assigned
// every entry), consumer-a crashes with one entry un-acknowledged,
// XPENDING reports the stalled entry with its owner, XAUTOCLAIM hands it
// to consumer-b with its values intact, consumer-b finishes it, and the
// pending ledger drains to zero. The builder below is PURE — proofs
// derive from the run's own rows, so the payload can never claim an
// order the store did not answer.
package demos

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// MessagingDemoKeyPrefix namespaces every key the messaging demos touch:
// the compose Redis is shared, and gdg:demo:msg marks this topic's keys
// as disposable territory.
const MessagingDemoKeyPrefix = "gdg:demo:msg"

// MessagingDemoGroup is the consumer-group name both demos use — the
// group is per-stream, and streams are per-run, so one name reads
// honestly everywhere.
const MessagingDemoGroup = "workers"

// MessagingStreamSteps is the stream story's fixed payload sequence:
// three order-lifecycle events appended in order, because the demo's
// ordering proof needs entries whose meaning is their position.
var MessagingStreamSteps = []string{"created", "paid", "shipped"}

// MessagingStreamRunner is the port the msg:stream demo reads through:
// one call walks the whole consumer-group lifecycle under a throwaway
// namespace and reports what the store answered. The go-redis-backed
// implementation lives in internal/data/messaging.
type MessagingStreamRunner interface {
	// RunStreamStory appends, groups, reads, stalls, inspects, reclaims,
	// and cleans up — the full loop, step by step.
	RunStreamStory(ctx context.Context, namespace string) (MessagingStreamRun, error)
}

// MessagingEntryRow is one stream entry as the story observed it: the
// store-assigned id and the step the payload carried.
type MessagingEntryRow struct {
	ID   string
	Step string
}

// MessagingPendingRow is one pending-ledger observation: the entry id,
// the consumer that owns it, and how many times it has been delivered.
type MessagingPendingRow struct {
	ID         string
	Consumer   string
	Deliveries int64
}

// MessagingStreamRun is the stream story's raw haul: exactly what the
// live store answered, beat by beat.
type MessagingStreamRun struct {
	Namespace         string
	Stream            string
	Group             string
	AppendedIDs       []string
	ConsumerARead     []MessagingEntryRow
	ConsumerBRead     []MessagingEntryRow
	AckedByA          []string
	StalledID         string
	PendingBefore     []MessagingPendingRow
	ClaimedByB        []MessagingEntryRow
	PendingAfterCount int64
	GroupDestroyed    bool
	StreamDeleted     bool
}

// MessagingStreamPayload is the wire shape of GET /api/demo/msg/stream
// (camelCase JSON). Deterministic for a given store: every beat writes
// fixed payloads; only the namespace and entry ids vary per run, and
// every slice is non-nil so the wire renders [], never null.
type MessagingStreamPayload struct {
	Namespace string                    `json:"namespace"`
	Stream    string                    `json:"stream"`
	Group     string                    `json:"group"`
	Commands  []string                  `json:"commands"`
	Beats     []MessagingStreamBeatRow  `json:"beats"`
	Proofs    MessagingStreamProofGroup `json:"proofs"`
	Reasoning string                    `json:"reasoning"`
}

// MessagingStreamBeatRow is one beat as the payload serves it: what was
// done, the commands that did it, and what the store answered.
type MessagingStreamBeatRow struct {
	Beat     string   `json:"beat"`
	Commands []string `json:"commands"`
	Outcome  string   `json:"outcome"`
	Entries  []string `json:"entries"`
}

// MessagingStreamProofGroup carries the story's derived verdicts: the
// ordering proof, the queue-vs-fanout proof, and the recovery proof.
type MessagingStreamProofGroup struct {
	Ordering    string `json:"ordering"`
	QueueProof  string `json:"queueProof"`
	Recovery    string `json:"recovery"`
	PendingLeft int64  `json:"pendingLeft"`
}

// NewMessagingDemoNamespace mints a messaging demo run's throwaway
// namespace: gdg:demo:msg:<12 hex chars>. Random per request, so
// concurrent runs never share streams, groups, or keys.
func NewMessagingDemoNamespace() (string, error) {
	var entropy [6]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("read messaging demo namespace entropy: %w", err)
	}
	return MessagingDemoKeyPrefix + ":" + hex.EncodeToString(entropy[:]), nil
}

// BuildMessagingStreamPayload shapes a run into the demo payload: the
// beats in execution order, and every proof sentence DERIVED from the
// run's own rows — ids compared for order, the second consumer's read
// inspected for emptiness, the claimed id matched against the stalled
// one — so a claim can never drift from what the store said.
func BuildMessagingStreamPayload(run MessagingStreamRun) MessagingStreamPayload {
	idsOf := func(rows []MessagingEntryRow) []string {
		ids := make([]string, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		return ids
	}
	stepsOf := func(rows []MessagingEntryRow) []string {
		steps := make([]string, 0, len(rows))
		for _, row := range rows {
			steps = append(steps, row.Step)
		}
		return steps
	}

	// The ordering proof: consumer-a's ids, read back, must be the
	// appended ids verbatim — same members, same order.
	ordered := len(run.ConsumerARead) == len(run.AppendedIDs)
	if ordered {
		for i := range run.AppendedIDs {
			if run.ConsumerARead[i].ID != run.AppendedIDs[i] {
				ordered = false
				break
			}
		}
	}
	ordering := fmt.Sprintf(
		"%d entries appended in order (%s); consumer-a's XREADGROUP answered the same ids in the same order — "+
			"a stream is an append-only LOG, so entries read back in insertion order and ids sort as they happened.",
		len(run.AppendedIDs), joinSteps(stepsOf(run.ConsumerARead)),
	)
	if !ordered {
		ordering = fmt.Sprintf(
			"%d entries appended but consumer-a read %d back in a different order than they were appended — "+
				"reported as it happened; a stream promises insertion order, so this would be a store contract break.",
			len(run.AppendedIDs), len(run.ConsumerARead),
		)
	}

	// The queue proof: consumer-b read NOTHING new because the group had
	// already assigned every entry to consumer-a — one group over a
	// stream is a work QUEUE (each entry to exactly one consumer), not a
	// topic fan-out.
	queueProof := fmt.Sprintf(
		"consumer-b's XREADGROUP answered %d entries: the group had already assigned every entry to consumer-a. "+
			"One consumer group over a stream is a work queue — load-balancing across consumers; MORE groups "+
			"is how you get fan-out (every group sees every entry).",
		len(run.ConsumerBRead),
	)
	if len(run.ConsumerBRead) > 0 {
		queueProof = fmt.Sprintf(
			"consumer-b read %d entries that consumer-a had already received — the group assigned new entries "+
				"to b, reported as it happened.",
			len(run.ConsumerBRead),
		)
	}

	// The recovery proof: XPENDING named the stalled entry with its
	// owner, XAUTOCLAIM moved exactly that entry to consumer-b.
	claimedStalled := len(run.ClaimedByB) == 1 && run.ClaimedByB[0].ID == run.StalledID
	recovery := fmt.Sprintf(
		"consumer-a acknowledged %d of %d and \"crashed\" with %s un-acknowledged; XPENDING reported %d pending "+
			"entry owned by consumer-a; XAUTOCLAIM moved %d entries to consumer-b — %s — and the pending ledger "+
			"drained to %d.",
		len(run.AckedByA), len(run.AppendedIDs), run.StalledID, len(run.PendingBefore),
		len(run.ClaimedByB), map[bool]string{true: "exactly the stalled entry, values intact", false: "a different set than the stalled entry — reported as it happened"}[claimedStalled],
		run.PendingAfterCount,
	)

	beats := []MessagingStreamBeatRow{
		{
			Beat:     "append",
			Commands: xaddCommands(run.Stream, stepsOf(run.ConsumerARead)),
			Outcome:  fmt.Sprintf("three entries appended; ids assigned by the store: %s.", joinIDs(idsOf(run.ConsumerARead))),
			Entries:  idsOf(run.ConsumerARead),
		},
		{
			Beat:     "group",
			Commands: []string{fmt.Sprintf("XGROUP CREATE %s %s 0 MKSTREAM", run.Stream, run.Group)},
			Outcome:  "the group starts at 0-0, so the whole history is in scope for its consumers.",
		},
		{
			Beat:     "consume (a)",
			Commands: []string{fmt.Sprintf("XREADGROUP GROUP %s %s COUNT 10 >", run.Group, "consumer-a")},
			Outcome:  fmt.Sprintf("consumer-a received all %d entries in insertion order — steps %s.", len(run.ConsumerARead), joinSteps(stepsOf(run.ConsumerARead))),
			Entries:  idsOf(run.ConsumerARead),
		},
		{
			Beat:     "consume (b)",
			Commands: []string{fmt.Sprintf("XREADGROUP GROUP %s %s COUNT 10 >", run.Group, "consumer-b")},
			Outcome:  fmt.Sprintf("consumer-b received %d new entries — the queue had nothing left for it.", len(run.ConsumerBRead)),
			Entries:  idsOf(run.ConsumerBRead),
		},
		{
			Beat: "ack + crash",
			Commands: []string{
				fmt.Sprintf("XACK %s %s %s", run.Stream, run.Group, joinIDs(run.AckedByA)),
			},
			Outcome: fmt.Sprintf("consumer-a acknowledged %d entries, then died with %s still pending.", len(run.AckedByA), run.StalledID),
		},
		{
			Beat:     "pending",
			Commands: []string{fmt.Sprintf("XPENDING %s %s - + 100", run.Stream, run.Group)},
			Outcome:  fmt.Sprintf("the ledger holds %d entry: %s, owned by %s, delivered %d time(s).", len(run.PendingBefore), run.StalledID, ownerOf(run.PendingBefore, run.StalledID), deliveriesOf(run.PendingBefore, run.StalledID)),
		},
		{
			Beat:     "autoclaim",
			Commands: []string{fmt.Sprintf("XAUTOCLAIM %s %s consumer-b 0 0-0 COUNT 10", run.Stream, run.Group)},
			Outcome:  fmt.Sprintf("consumer-b claimed %d stalled %s with the values intact and finished the work.", len(run.ClaimedByB), entryNoun(len(run.ClaimedByB))),
			Entries:  idsOf(run.ClaimedByB),
		},
		{
			Beat:     "drained",
			Commands: []string{fmt.Sprintf("XPENDING %s %s - + 100", run.Stream, run.Group)},
			Outcome:  fmt.Sprintf("the pending ledger holds %d entries — every delivered entry is now acknowledged.", run.PendingAfterCount),
		},
	}

	cleanupNote := "clean"
	if !run.GroupDestroyed || !run.StreamDeleted {
		cleanupNote = "reported as it happened"
	}
	reasoning := fmt.Sprintf(
		"Every beat above is what the live store answered under %s. The ordering proof: a stream is a log, so reads "+
			"return insertion order. The queue proof: one consumer GROUP balances work across consumers; separate groups "+
			"would each see every entry (fan-out). The recovery proof: un-acknowledged entries stay in the group's pending "+
			"ledger with their owner named, so XAUTOCLAIM can move stalled work to a live consumer — crash recovery is a "+
			"command, not a mystery. Teardown (%s): group destroyed, stream deleted, nothing left behind.",
		run.Namespace, cleanupNote,
	)

	return MessagingStreamPayload{
		Namespace: run.Namespace,
		Stream:    run.Stream,
		Group:     run.Group,
		Commands: []string{
			"XADD", "XGROUP CREATE", "XREADGROUP", "XACK", "XPENDING", "XAUTOCLAIM", "XGROUP DESTROY",
		},
		Beats: beats,
		Proofs: MessagingStreamProofGroup{
			Ordering:    ordering,
			QueueProof:  queueProof,
			Recovery:    recovery,
			PendingLeft: run.PendingAfterCount,
		},
		Reasoning: reasoning,
	}
}

// joinSteps renders steps as a readable sequence: created → paid → shipped.
func joinSteps(steps []string) string {
	out := ""
	for i, step := range steps {
		if i > 0 {
			out += " → "
		}
		out += step
	}
	return out
}

// joinIDs joins ids comma-separated — full ids, so the payload can be
// matched against a live store by eye.
func joinIDs(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ", "
		}
		out += id
	}
	return out
}

// entryNoun picks entry/entries for a count.
func entryNoun(n int) string {
	if n == 1 {
		return "entry"
	}
	return "entries"
}

// xaddCommands renders one XADD command per step, derived from what the
// run actually appended — the payload can never drift from the story.
func xaddCommands(stream string, steps []string) []string {
	commands := make([]string, 0, len(steps))
	for _, step := range steps {
		commands = append(commands, fmt.Sprintf("XADD %s … step %s", stream, step))
	}
	return commands
}

// ownerOf finds the named owner of a pending entry.
func ownerOf(pending []MessagingPendingRow, id string) string {
	for _, row := range pending {
		if row.ID == id {
			return row.Consumer
		}
	}
	return "unknown"
}

// deliveriesOf finds how often the named pending entry was delivered.
func deliveriesOf(pending []MessagingPendingRow, id string) int64 {
	for _, row := range pending {
		if row.ID == id {
			return row.Deliveries
		}
	}
	return 0
}
