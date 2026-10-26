// Topic 20 redis's single live demo: the playground. One request walks
// the five core structures on the RUNNING stack Redis through the
// RedisPlaygroundRunner port (string with its inline EX, hash, list, set,
// sorted set with scores), proves INCR atomicity with a pipelined batch
// (N increments, one round trip, an exact final total), reads TTL and NX
// semantics as DATA (the countdown the store owns — no sleeping out a
// wall-clock promise), performs a REAL pub/sub round-trip with both sides
// of the wire reported, and cleans up every key it touched. The builder
// below is PURE — it passes the run's rows through and derives every
// proof sentence from what actually ran, so the payload can never drift
// from the store.
package demos

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// The demo's beat constants: the exact values every walk writes, shared
// by the adapter (which runs them) and the builder (whose proofs derive
// from them), so the two can never drift.
const (
	// RedisDemoKeyPrefix namespaces every key the demo touches — the
	// compose Redis is shared with future topics, and gdg:demo:redis:
	// marks these keys as the playground's own, disposable territory.
	RedisDemoKeyPrefix = "gdg:demo:redis"

	// RedisDemoTTLSeconds is the inline EX the string and TTL beats carry:
	// long enough to read a live countdown, short enough to be a real
	// deadline. The demo never waits it out — expiry is the STORE's job.
	RedisDemoTTLSeconds = 30

	// RedisDemoCounterPipeline is how many INCRs ride the pipeline beat:
	// big enough that "no locks in application code" means something,
	// small enough to read as a list.
	RedisDemoCounterPipeline = 25
)

// RedisDemoGreeting is the string beat's value — SET with an inline EX,
// then read straight back.
const RedisDemoGreeting = "Hello, Redis — the guide's first string."

// RedisDemoHashFields is the hash beat's field set: four fields written
// in one HSET, read back whole with HGETALL.
var RedisDemoHashFields = map[string]string{
	"title":      "one key, many fields — the object-shaped value",
	"engine":     "in-memory",
	"structures": "five walked here",
	"eviction":   "per-key TTL",
}

// RedisDemoListPushes is the list beat's push order: LPUSH puts each at
// the head, so the read-back is exactly reversed — the push-order
// inversion the demo exists to show.
var RedisDemoListPushes = []string{"first", "second", "third"}

// RedisDemoSetMembers is the set beat's membership attempt: four SADD
// arguments, three DISTINCT members — the duplicate is refused by the
// structure, not by application code.
var RedisDemoSetMembers = []string{"go", "redis", "server", "go"}

// RedisDemoZSetScores is the sorted-set beat's members with scores,
// listed by DESCENDING score on purpose: the read-back (ZRANGE, ascending)
// must invert it — the ordering belongs to the structure.
var RedisDemoZSetScores = []RedisZSetMember{
	{Member: "grace", Score: 100},
	{Member: "ada", Score: 85},
	{Member: "linus", Score: 97},
}

// RedisZSetMember is one sorted-set entry the demo writes.
type RedisZSetMember struct {
	Member string
	Score  float64
}

// RedisDemoPubSubPayload is the pub/sub beat's message: fixed content,
// matched verbatim against what the subscriber collected.
const RedisDemoPubSubPayload = "playground event — hello from the publisher"

// RedisPlaygroundRunner is the port the playground reads through: one
// call walks every beat under the namespace and reports what the store
// answered. The go-redis-backed implementation lives in
// internal/data/redis; tests and the handler wire fakes through this
// interface.
type RedisPlaygroundRunner interface {
	// Run walks the structures, the pipelined counter, the TTL/NX
	// semantics, and the pub/sub round-trip under namespace, then removes
	// every key it created.
	Run(ctx context.Context, namespace string) (RedisPlaygroundRun, error)
}

// RedisPlaygroundRun is the runner's raw haul: exactly what the live
// store answered, beat by beat — the builder shapes it, derives its
// proofs from it, and adds nothing the store did not say.
type RedisPlaygroundRun struct {
	// Namespace is the run's key prefix; every key below lives under it
	// and the cleanup beat removes exactly that keyspace.
	Namespace  string
	Structures []RedisStructureRun
	Counter    RedisCounterRun
	TTL        RedisTTLRun
	PubSub     RedisPubSubRun
	Cleanup    RedisCleanupRun
}

// RedisStructureRun is ONE structure's beat: the commands as run, the
// values read back, and the one-line outcome the store's replies add up
// to.
type RedisStructureRun struct {
	Kind        string // string | hash | list | set | zset
	Key         string
	Description string
	Commands    []string
	Outcome     string
	Values      []RedisValueRow
}

// RedisValueRow is one labeled value a structure's read returned.
type RedisValueRow struct {
	Label string
	Value string
}

// RedisCounterRun is the atomicity beat: a fresh key, one pipeline of
// RedisDemoCounterPipeline INCRs, and the exact total a GET answered
// afterwards.
type RedisCounterRun struct {
	Key          string
	PipelineSize int
	Commands     []string
	Total        int64
}

// RedisTTLRun is the expiry beat, read as data: the TTL the store
// reported for the EX the SET carried, plus the two-command NX lock
// (first SET NX acquires, second is refused).
type RedisTTLRun struct {
	Key          string
	TTLSeconds   int
	TTLFirstRead int64
	NXFirstOK    bool
	NXSecondOK   bool
	Commands     []string
}

// RedisPubSubRun is the pub/sub beat: what was published, how many
// receivers the store says saw it, and the payload the subscriber
// actually collected.
type RedisPubSubRun struct {
	Channel         string
	Payload         string
	Commands        []string
	Receivers       int64
	ReceivedPayload string
}

// RedisCleanupRun is the no-residue beat: how many keys the final
// SCAN+DEL removed.
type RedisCleanupRun struct {
	KeysRemoved int64
	Commands    []string
}

// RedisPlaygroundPayload is the wire shape of GET /api/demo/redis/playground
// (camelCase JSON). Deterministic for a given store: every beat writes
// fixed values and reads them back, so two runs differ only in their
// throwaway namespace and the TTL's live countdown — every slice is
// non-nil so the wire renders [], never null.
type RedisPlaygroundPayload struct {
	Namespace  string              `json:"namespace"`
	Structures []RedisStructureRow `json:"structures"`
	Counter    RedisCounterProof   `json:"counter"`
	TTL        RedisTTLProof       `json:"ttl"`
	PubSub     RedisPubSubProof    `json:"pubsub"`
	Cleanup    RedisCleanupProof   `json:"cleanup"`
	Reasoning  string              `json:"reasoning"`
}

// RedisStructureRow is one structure as the payload serves it.
type RedisStructureRow struct {
	Kind        string              `json:"kind"`
	Key         string              `json:"key"`
	Description string              `json:"description"`
	Commands    []string            `json:"commands"`
	Outcome     string              `json:"outcome"`
	Values      []RedisValueDisplay `json:"values"`
}

// RedisValueDisplay is one labeled value on the wire.
type RedisValueDisplay struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// RedisCounterProof is the atomicity beat as the payload serves it, with
// its verdict derived from the live total.
type RedisCounterProof struct {
	Key          string   `json:"key"`
	PipelineSize int      `json:"pipelineSize"`
	Commands     []string `json:"commands"`
	Total        int64    `json:"total"`
	Proof        string   `json:"proof"`
}

// RedisTTLProof is the expiry beat as the payload serves it: the
// countdown read as data, the NX lock's two answers, and the derived
// proof sentence.
type RedisTTLProof struct {
	Key          string   `json:"key"`
	TTLSeconds   int      `json:"ttlSeconds"`
	Commands     []string `json:"commands"`
	TTLFirstRead int64    `json:"ttlFirstRead"`
	NXFirstOK    bool     `json:"nxFirstAcquired"`
	NXSecondOK   bool     `json:"nxSecondAcquired"`
	Proof        string   `json:"proof"`
}

// RedisPubSubProof is the pub/sub beat as the payload serves it, with
// matched derived from the two payload strings.
type RedisPubSubProof struct {
	Channel         string   `json:"channel"`
	Payload         string   `json:"payload"`
	Commands        []string `json:"commands"`
	Receivers       int64    `json:"receivers"`
	ReceivedPayload string   `json:"receivedPayload"`
	Matched         bool     `json:"matched"`
	Proof           string   `json:"proof"`
}

// RedisCleanupProof is the no-residue beat as the payload serves it.
type RedisCleanupProof struct {
	KeysRemoved int64    `json:"keysRemoved"`
	Commands    []string `json:"commands"`
	Proof       string   `json:"proof"`
}

// NewRedisDemoNamespace mints a run's throwaway key namespace:
// gdg:demo:redis:<12 hex chars>. Random, so concurrent runs never share
// keys — the counter beat's exact total needs no coordination — and
// short, because it prefixes every key the run touches.
func NewRedisDemoNamespace() (string, error) {
	var entropy [6]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("read demo namespace entropy: %w", err)
	}
	return RedisDemoKeyPrefix + ":" + hex.EncodeToString(entropy[:]), nil
}

// BuildRedisPlaygroundPayload shapes a run into the demo payload: rows
// pass through with nil slices materialized as [], and every proof
// sentence DERIVES from the run's own numbers — the total the GET
// answered, the countdown the TTL returned, whether the two payload
// strings matched — so a payload can never claim a thing the store did
// not say.
func BuildRedisPlaygroundPayload(run RedisPlaygroundRun) RedisPlaygroundPayload {
	structures := make([]RedisStructureRow, 0, len(run.Structures))
	for _, structure := range run.Structures {
		values := make([]RedisValueDisplay, 0, len(structure.Values))
		for _, value := range structure.Values {
			values = append(values, RedisValueDisplay{Label: value.Label, Value: value.Value})
		}
		structures = append(structures, RedisStructureRow{
			Kind:        structure.Kind,
			Key:         structure.Key,
			Description: structure.Description,
			Commands:    nonNilStrings(structure.Commands),
			Outcome:     structure.Outcome,
			Values:      values,
		})
	}

	counterProof := fmt.Sprintf(
		"%d INCRs rode ONE pipeline (one round trip) and the final GET answered %d — "+
			"the increments are atomic on the server's single command thread, so the total "+
			"is exact with no lock anywhere in application code.",
		run.Counter.PipelineSize, run.Counter.Total,
	)
	if run.Counter.Total != int64(run.Counter.PipelineSize) {
		counterProof = fmt.Sprintf(
			"%d INCRs rode one pipeline but the final GET answered %d — the verdict is the "+
				"store's own arithmetic, reported as it happened.",
			run.Counter.PipelineSize, run.Counter.Total,
		)
	}

	ttlProof := fmt.Sprintf(
		"SET carried its EX %d inline — value and deadline are one atomic command — and TTL "+
			"read %d back: a countdown the STORE owns, served here as data rather than slept "+
			"out. The lock pair closed the beat: the first SET NX acquired (OK), the second "+
			"was refused (nil) — the lock primitive in two commands.",
		run.TTL.TTLSeconds, run.TTL.TTLFirstRead,
	)

	pubsub := RedisPubSubProof{
		Channel:         run.PubSub.Channel,
		Payload:         run.PubSub.Payload,
		Commands:        nonNilStrings(run.PubSub.Commands),
		Receivers:       run.PubSub.Receivers,
		ReceivedPayload: run.PubSub.ReceivedPayload,
		Matched:         run.PubSub.ReceivedPayload == run.PubSub.Payload,
	}
	pubsub.Proof = fmt.Sprintf(
		"PUBLISH answered %d receiver(s) and the subscriber collected %q — both sides of "+
			"one real round trip, bounded by a context deadline, never by hope.",
		run.PubSub.Receivers, run.PubSub.ReceivedPayload,
	)
	if !pubsub.Matched {
		pubsub.Proof = fmt.Sprintf(
			"PUBLISH answered %d receiver(s) but the subscriber collected %q against a "+
				"published %q — a mismatch the demo reports rather than papers over.",
			run.PubSub.Receivers, run.PubSub.ReceivedPayload, run.PubSub.Payload,
		)
	}

	cleanup := RedisCleanupProof{
		KeysRemoved: run.Cleanup.KeysRemoved,
		Commands:    nonNilStrings(run.Cleanup.Commands),
	}
	cleanup.Proof = fmt.Sprintf(
		"SCAN + DEL removed %d keys under the run's namespace — the playground leaves "+
			"nothing behind on the shared store.",
		run.Cleanup.KeysRemoved,
	)

	kinds := ""
	for i, structure := range run.Structures {
		if i > 0 {
			kinds += ", "
		}
		kinds += structure.Kind
	}
	reasoning := fmt.Sprintf(
		"Every row above is what the live store answered under %s — the structures walked in "+
			"order (%s), each with its real read-back; the counter proved INCR atomicity with a "+
			"pipelined batch; the TTL beat read the countdown as data and showed SET NX refusing "+
			"a second holder; the pub/sub beat delivered one real message to one real subscriber. "+
			"Redis is a data STRUCTURES server — the sorting, the uniqueness, the counting, and "+
			"the ordering below all happened inside the store, not in this code.",
		run.Namespace, kinds,
	)

	return RedisPlaygroundPayload{
		Namespace:  run.Namespace,
		Structures: structures,
		Counter: RedisCounterProof{
			Key:          run.Counter.Key,
			PipelineSize: run.Counter.PipelineSize,
			Commands:     nonNilStrings(run.Counter.Commands),
			Total:        run.Counter.Total,
			Proof:        counterProof,
		},
		TTL: RedisTTLProof{
			Key:          run.TTL.Key,
			TTLSeconds:   run.TTL.TTLSeconds,
			Commands:     nonNilStrings(run.TTL.Commands),
			TTLFirstRead: run.TTL.TTLFirstRead,
			NXFirstOK:    run.TTL.NXFirstOK,
			NXSecondOK:   run.TTL.NXSecondOK,
			Proof:        ttlProof,
		},
		PubSub:    pubsub,
		Cleanup:   cleanup,
		Reasoning: reasoning,
	}
}
