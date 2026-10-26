package demos

import (
	"strings"
	"testing"
)

// The redis-playground payload builder: pure shaping over a run — the
// tests feed hand-built runs and pin the mapping: rows pass through with
// nil slices materialized as [], and every proof sentence DERIVES from
// the run's own numbers (the counter total, the TTL countdown, the
// pub/sub match) so the payload can never claim a thing the store did
// not say.

func redisPlaygroundFixtureRun() RedisPlaygroundRun {
	return RedisPlaygroundRun{
		Namespace: "gdg:demo:redis:0123456789ab",
		Structures: []RedisStructureRun{
			{
				Kind:        "string",
				Key:         "gdg:demo:redis:0123456789ab:greeting",
				Description: "the workhorse",
				Commands:    []string{"SET … EX 30", "GET …"},
				Outcome:     "SET answered OK; GET read the value straight back.",
				Values:      []RedisValueRow{{Label: "GET", Value: RedisDemoGreeting}},
			},
			{
				Kind:     "hash",
				Key:      "gdg:demo:redis:0123456789ab:post",
				Commands: []string{"HSET …", "HGETALL …"},
				Outcome:  "HSET answered 4 new fields; HGETALL returned all 4.",
				Values: []RedisValueRow{
					{Label: "engine", Value: "in-memory"},
					{Label: "eviction", Value: "per-key TTL"},
				},
			},
		},
		Counter: RedisCounterRun{
			Key:          "gdg:demo:redis:0123456789ab:views",
			PipelineSize: RedisDemoCounterPipeline,
			Commands:     []string{"DEL …", "PIPELINE · 25× INCR …", "GET …"},
			Total:        25,
		},
		TTL: RedisTTLRun{
			Key:          "gdg:demo:redis:0123456789ab:flash",
			TTLSeconds:   RedisDemoTTLSeconds,
			TTLFirstRead: 30,
			NXFirstOK:    true,
			NXSecondOK:   false,
			Commands:     []string{"SET … EX 30", "TTL …", "SET … NX", "SET … NX"},
		},
		PubSub: RedisPubSubRun{
			Channel:         "gdg:demo:redis:0123456789ab:events",
			Payload:         RedisDemoPubSubPayload,
			Commands:        []string{"SUBSCRIBE …", "PUBLISH …"},
			Receivers:       1,
			ReceivedPayload: RedisDemoPubSubPayload,
		},
		Cleanup: RedisCleanupRun{
			KeysRemoved: 8,
			Commands:    []string{"SCAN 0 MATCH …", "DEL …"},
		},
	}
}

func TestRedisPlaygroundBuilderDerivesProofsFromTheRun(t *testing.T) {
	t.Parallel()

	payload := BuildRedisPlaygroundPayload(redisPlaygroundFixtureRun())

	if payload.Namespace != "gdg:demo:redis:0123456789ab" {
		t.Errorf("namespace = %q, want the run's own namespace", payload.Namespace)
	}
	if len(payload.Structures) != 2 || payload.Structures[0].Kind != "string" || payload.Structures[1].Kind != "hash" {
		t.Fatalf("structures drifted: %+v", payload.Structures)
	}
	if payload.Structures[0].Values[0].Value != RedisDemoGreeting {
		t.Errorf("string read-back = %q, want the greeting", payload.Structures[0].Values[0].Value)
	}

	// The counter proof derives from the LIVE total: 25 pipelined INCRs
	// and a GET that answered exactly 25.
	if payload.Counter.Total != 25 || payload.Counter.PipelineSize != RedisDemoCounterPipeline {
		t.Errorf("counter drifted: %+v", payload.Counter)
	}
	if !strings.Contains(payload.Counter.Proof, "one round trip") || !strings.Contains(payload.Counter.Proof, "answered 25") {
		t.Errorf("counter proof must derive from the run's total: %q", payload.Counter.Proof)
	}

	// The TTL proof carries the configured EX and the read countdown.
	if payload.TTL.TTLSeconds != 30 || payload.TTL.TTLFirstRead != 30 {
		t.Errorf("ttl proof inputs drifted: %+v", payload.TTL)
	}
	if !strings.Contains(payload.TTL.Proof, "EX 30") || !strings.Contains(payload.TTL.Proof, "read 30 back") {
		t.Errorf("ttl proof must derive from the run's values: %q", payload.TTL.Proof)
	}
	if !payload.TTL.NXFirstOK || payload.TTL.NXSecondOK {
		t.Errorf("nx answers drifted: %+v", payload.TTL)
	}

	// The pub/sub match derives from the two payload strings.
	if !payload.PubSub.Matched {
		t.Error("identical published and received payloads must derive matched=true")
	}
	if !strings.Contains(payload.PubSub.Proof, "1 receiver") {
		t.Errorf("pubsub proof must derive from the receiver count: %q", payload.PubSub.Proof)
	}
	if !strings.Contains(payload.Cleanup.Proof, "removed 8 keys") {
		t.Errorf("cleanup proof must derive from the removed count: %q", payload.Cleanup.Proof)
	}
	if !strings.Contains(payload.Reasoning, "gdg:demo:redis:0123456789ab") {
		t.Errorf("reasoning must name the run's namespace: %q", payload.Reasoning)
	}
}

func TestRedisPlaygroundBuilderReportsTheStoreVerdictsNotPromises(t *testing.T) {
	t.Parallel()

	// A run whose numbers DISAGREE with the happy story: the counter
	// short by one, the pub/sub payloads mismatched. The builder must
	// report those as what happened, never paper over them.
	run := redisPlaygroundFixtureRun()
	run.Counter.Total = 24
	run.PubSub.ReceivedPayload = "something else entirely"

	payload := BuildRedisPlaygroundPayload(run)

	if strings.Contains(payload.Counter.Proof, "the total is exact") {
		t.Errorf("a short total must not be called exact: %q", payload.Counter.Proof)
	}
	if !strings.Contains(payload.Counter.Proof, "answered 24") {
		t.Errorf("counter proof must report the store's arithmetic: %q", payload.Counter.Proof)
	}
	if payload.PubSub.Matched {
		t.Error("differing published/received payloads must derive matched=false")
	}
	if !strings.Contains(payload.PubSub.Proof, "mismatch") {
		t.Errorf("pubsub proof must report the mismatch: %q", payload.PubSub.Proof)
	}
}

func TestRedisPlaygroundBuilderNeverServesNilCollections(t *testing.T) {
	t.Parallel()

	// A run whose rows carry nil commands/values maps to [] on the wire,
	// never null — and an absent structure list still renders [].
	payload := BuildRedisPlaygroundPayload(RedisPlaygroundRun{
		Namespace: "gdg:demo:redis:ffffffffffff",
		Structures: []RedisStructureRun{
			{Kind: "set", Key: "k", Description: "d", Outcome: "o"},
		},
		Counter: RedisCounterRun{Key: "k", PipelineSize: 1, Total: 1},
		TTL:     RedisTTLRun{Key: "k", TTLSeconds: 30, TTLFirstRead: 30},
		PubSub:  RedisPubSubRun{Channel: "c", Payload: "p", Receivers: 1, ReceivedPayload: "p"},
		Cleanup: RedisCleanupRun{KeysRemoved: 1},
	})

	if payload.Structures[0].Commands == nil || len(payload.Structures[0].Commands) != 0 {
		t.Error("nil structure commands must map to a non-nil empty slice")
	}
	if payload.Structures[0].Values == nil || len(payload.Structures[0].Values) != 0 {
		t.Error("nil structure values must map to a non-nil empty slice")
	}
	if payload.Counter.Commands == nil || payload.TTL.Commands == nil ||
		payload.PubSub.Commands == nil || payload.Cleanup.Commands == nil {
		t.Error("nil proof commands must map to non-nil empty slices")
	}
}

func TestNewRedisDemoNamespaceMintsNamespacedUniqueHex(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool, 128)
	for range 128 {
		namespace, err := NewRedisDemoNamespace()
		if err != nil {
			t.Fatalf("mint namespace: %v", err)
		}
		if !strings.HasPrefix(namespace, RedisDemoKeyPrefix+":") {
			t.Fatalf("namespace %q must carry the %q prefix", namespace, RedisDemoKeyPrefix)
		}
		suffix := strings.TrimPrefix(namespace, RedisDemoKeyPrefix+":")
		if len(suffix) != 12 {
			t.Fatalf("namespace suffix %q must be 12 hex chars", suffix)
		}
		for _, ch := range suffix {
			if !strings.ContainsRune("0123456789abcdef", ch) {
				t.Fatalf("namespace suffix %q must be lowercase hex", suffix)
			}
		}
		if seen[namespace] {
			t.Fatalf("namespace %q minted twice — runs must be independent", namespace)
		}
		seen[namespace] = true
	}
}
