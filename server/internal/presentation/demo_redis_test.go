package presentation

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The redis topic's demo endpoint, exercised through the same middleware
// chain production runs behind (the family rule that every new demo
// endpoint ships with a Go test). The runner port is faked here — the
// go-redis-backed adapter has its own tests against the running stack
// Redis in internal/data/redis — so both the happy path and the honest
// 503 degradation are pinned without a store.

// fakeRedisRunner stands in for the go-redis-backed runner: it returns a
// canned run shaped like a real walk, or the failure its field carries.
type fakeRedisRunner struct {
	run demos.RedisPlaygroundRun
	err error
}

func (f fakeRedisRunner) Run(_ context.Context, _ string) (demos.RedisPlaygroundRun, error) {
	return f.run, f.err
}

// redisPlaygroundFixtureRun mirrors a real walk's shape closely enough to
// pin the handler's mapping: two structures, the counter, TTL, pub/sub,
// and cleanup proofs.
func redisPlaygroundFixtureRun() demos.RedisPlaygroundRun {
	return demos.RedisPlaygroundRun{
		Namespace: "gdg:demo:redis:0123456789ab",
		Structures: []demos.RedisStructureRun{
			{
				Kind:        "string",
				Key:         "gdg:demo:redis:0123456789ab:greeting",
				Description: "the workhorse",
				Commands:    []string{"SET … EX 30", "GET …"},
				Outcome:     "SET answered OK; GET read the value straight back.",
				Values:      []demos.RedisValueRow{{Label: "GET", Value: demos.RedisDemoGreeting}},
			},
			{
				Kind:     "zset",
				Key:      "gdg:demo:redis:0123456789ab:leaderboard",
				Commands: []string{"ZADD …", "ZRANGE … WITHSCORES"},
				Outcome:  "ZADD placed 3 members; ZRANGE read them back in score order.",
				Values: []demos.RedisValueRow{
					{Label: "ada", Value: "score 85"},
					{Label: "grace", Value: "score 100"},
				},
			},
		},
		Counter: demos.RedisCounterRun{
			Key:          "gdg:demo:redis:0123456789ab:views",
			PipelineSize: demos.RedisDemoCounterPipeline,
			Commands:     []string{"DEL …", "PIPELINE · 25× INCR …", "GET …"},
			Total:        25,
		},
		TTL: demos.RedisTTLRun{
			Key:          "gdg:demo:redis:0123456789ab:flash",
			TTLSeconds:   demos.RedisDemoTTLSeconds,
			TTLFirstRead: 30,
			NXFirstOK:    true,
			Commands:     []string{"SET … EX 30", "TTL …", "SET … NX", "SET … NX"},
		},
		PubSub: demos.RedisPubSubRun{
			Channel:         "gdg:demo:redis:0123456789ab:events",
			Payload:         demos.RedisDemoPubSubPayload,
			Commands:        []string{"SUBSCRIBE …", "PUBLISH …"},
			Receivers:       1,
			ReceivedPayload: demos.RedisDemoPubSubPayload,
		},
		Cleanup: demos.RedisCleanupRun{
			KeysRemoved: 8,
			Commands:    []string{"SCAN 0 MATCH …", "DEL …"},
		},
	}
}

func newRedisDemoServer(runner demos.RedisPlaygroundRunner, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/redis/playground", NewRedisPlaygroundHandler(runner, logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestRedisPlaygroundDemoServesTheShapedRun(t *testing.T) {
	var buf strings.Builder
	server := newRedisDemoServer(fakeRedisRunner{run: redisPlaygroundFixtureRun()}, newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/redis/playground", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q, want application/json", ct)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	decoded := struct {
		Namespace  string `json:"namespace"`
		Structures []struct {
			Kind   string `json:"kind"`
			Key    string `json:"key"`
			Values []struct {
				Label string `json:"label"`
				Value string `json:"value"`
			} `json:"values"`
		} `json:"structures"`
		Counter struct {
			PipelineSize int    `json:"pipelineSize"`
			Total        int64  `json:"total"`
			Proof        string `json:"proof"`
		} `json:"counter"`
		TTL struct {
			TTLSeconds   int   `json:"ttlSeconds"`
			TTLFirstRead int64 `json:"ttlFirstRead"`
			NXFirstOK    bool  `json:"nxFirstAcquired"`
			NXSecondOK   bool  `json:"nxSecondAcquired"`
		} `json:"ttl"`
		PubSub struct {
			Receivers int64 `json:"receivers"`
			Matched   bool  `json:"matched"`
		} `json:"pubsub"`
		Cleanup struct {
			KeysRemoved int64 `json:"keysRemoved"`
		} `json:"cleanup"`
		Reasoning string `json:"reasoning"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.Namespace != "gdg:demo:redis:0123456789ab" {
		t.Errorf("namespace = %q", decoded.Namespace)
	}
	if len(decoded.Structures) != 2 || decoded.Structures[0].Kind != "string" || decoded.Structures[1].Kind != "zset" {
		t.Errorf("structures drifted: %+v", decoded.Structures)
	}
	if decoded.Structures[0].Values[0].Value != demos.RedisDemoGreeting {
		t.Errorf("string read-back drifted: %+v", decoded.Structures[0].Values)
	}
	if decoded.Counter.PipelineSize != 25 || decoded.Counter.Total != 25 {
		t.Errorf("counter drifted: %+v", decoded.Counter)
	}
	if !strings.Contains(decoded.Counter.Proof, "answered 25") {
		t.Errorf("counter proof must derive from the total: %q", decoded.Counter.Proof)
	}
	if decoded.TTL.TTLSeconds != 30 || decoded.TTL.TTLFirstRead != 30 || !decoded.TTL.NXFirstOK || decoded.TTL.NXSecondOK {
		t.Errorf("ttl proof drifted: %+v", decoded.TTL)
	}
	if decoded.PubSub.Receivers != 1 || !decoded.PubSub.Matched {
		t.Errorf("pubsub proof drifted: %+v", decoded.PubSub)
	}
	if decoded.Cleanup.KeysRemoved != 8 {
		t.Errorf("cleanup drifted: %+v", decoded.Cleanup)
	}
	if !strings.Contains(decoded.Reasoning, "data STRUCTURES server") {
		t.Errorf("reasoning must carry the topic's thesis: %q", decoded.Reasoning)
	}
}

func TestRedisPlaygroundDemoDegradesTo503WhenTheStoreIsUnreachable(t *testing.T) {
	var buf strings.Builder
	server := newRedisDemoServer(
		fakeRedisRunner{err: errors.New("dial tcp: connection refused")},
		newTestLogger(&buf),
	)

	rec := doRequest(server, http.MethodGet, "/api/demo/redis/playground", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"service_unavailable"`) {
		t.Errorf("503 body is not the honest service_unavailable envelope: %s", body)
	}
	if !strings.Contains(body, "not reachable right now") {
		t.Errorf("503 body must say what is wrong in plain words: %s", body)
	}
	if rid := rec.Header().Get(core.RequestIDHeader); rid == "" {
		t.Error("503 must still carry x-request-id")
	}
}

func TestRedisPlaygroundDemoRejectsNonGet(t *testing.T) {
	var buf strings.Builder
	server := newRedisDemoServer(fakeRedisRunner{run: redisPlaygroundFixtureRun()}, newTestLogger(&buf))

	rec := doRequest(server, http.MethodPost, "/api/demo/redis/playground", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow header lost: %q", allow)
	}
}
