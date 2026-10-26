package redis

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"guide/internal/domain/demos"
)

// The playground adapter's tests run against the RUNNING stack Redis
// (go_developers_guide_redis_dev) — the compose network's in-network
// address by default, GDG_REDIS_ADDR to override (e.g. the host's
// published slot 6380). Every walk rides a THROWAWY namespace minted per
// test; the runner's own cleanup removes its keys, and a defensive
// t.Cleanup force-clears the prefix even when an assertion fails
// mid-walk. The degradation cases use a constructed bad address and a
// canceled context — no store required.

// testRedisAddr resolves the store under test.
func testRedisAddr() string {
	if addr := os.Getenv("GDG_REDIS_ADDR"); addr != "" {
		return addr
	}
	return "redis:6379"
}

// newTestRunner builds a runner over a fresh client and clears the
// namespace prefix on test end, whatever happened.
func newTestRunner(t *testing.T) (*PlaygroundRunner, string) {
	t.Helper()
	client := NewClient(testRedisAddr())
	namespace, err := demos.NewRedisDemoNamespace()
	if err != nil {
		client.Close()
		t.Fatalf("mint throwaway namespace: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		clearPattern(ctx, t, client, namespace+":*")
		client.Close()
	})
	return NewPlaygroundRunner(client), namespace
}

// clearPattern is the defensive belt: SCAN+DEL everything under pattern,
// used only by test cleanup, never by the beats themselves.
func clearPattern(ctx context.Context, t *testing.T, client *goredis.Client, pattern string) {
	t.Helper()
	var keys []string
	iterator := client.Scan(ctx, 0, pattern, 100).Iterator()
	for iterator.Next(ctx) {
		keys = append(keys, iterator.Val())
	}
	if err := iterator.Err(); err != nil {
		t.Logf("defensive clear %s: %v", pattern, err)
		return
	}
	if len(keys) > 0 {
		if _, err := client.Del(ctx, keys...).Result(); err != nil {
			t.Logf("defensive clear %s: %v", pattern, err)
		}
	}
}

func TestRedisPlaygroundRunnerWalksEveryBeatAgainstTheLiveStore(t *testing.T) {
	runner, namespace := newTestRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	run, err := runner.Run(ctx, namespace)
	if err != nil {
		t.Fatalf("the walk must succeed against the running store at %s: %v", testRedisAddr(), err)
	}

	// The five structures, in teaching order, each with its real
	// round-trip values.
	if len(run.Structures) != 5 {
		t.Fatalf("structures = %d, want the five beats", len(run.Structures))
	}
	wantKinds := []string{"string", "hash", "list", "set", "zset"}
	for i, want := range wantKinds {
		if run.Structures[i].Kind != want {
			t.Errorf("structure %d = %q, want %q", i, run.Structures[i].Kind, want)
		}
		if !strings.HasPrefix(run.Structures[i].Key, namespace+":") {
			t.Errorf("structure %d key %q must live under the namespace", i, run.Structures[i].Key)
		}
	}

	if got := run.Structures[0].Values[0].Value; got != demos.RedisDemoGreeting {
		t.Errorf("string GET round-trip = %q, want the greeting", got)
	}

	hash := run.Structures[1]
	if len(hash.Values) != len(demos.RedisDemoHashFields) {
		t.Fatalf("hash HGETALL returned %d fields, want %d", len(hash.Values), len(demos.RedisDemoHashFields))
	}
	hashFields := make(map[string]string, len(hash.Values))
	for _, row := range hash.Values {
		hashFields[row.Label] = row.Value
	}
	for field, want := range demos.RedisDemoHashFields {
		if hashFields[field] != want {
			t.Errorf("hash field %s = %q, want %q", field, hashFields[field], want)
		}
	}
	for i := 1; i < len(hash.Values); i++ {
		if hash.Values[i-1].Label >= hash.Values[i].Label {
			t.Errorf("hash fields must render sorted: %+v", hash.Values)
		}
	}

	list := run.Structures[2]
	if len(list.Values) != 3 ||
		list.Values[0].Value != "third" || list.Values[1].Value != "second" || list.Values[2].Value != "first" {
		t.Errorf("list LRANGE = %+v, want the push order inverted", list.Values)
	}

	set := run.Structures[3]
	if len(set.Values) != 3 {
		t.Fatalf("set SMEMBERS returned %d members, want the 3 distinct", len(set.Values))
	}
	for i, want := range []string{"go", "redis", "server"} {
		if set.Values[i].Value != want {
			t.Errorf("set member %d = %q, want %q (sorted)", i, set.Values[i].Value, want)
		}
	}

	zset := run.Structures[4]
	if len(zset.Values) != 3 ||
		zset.Values[0].Label != "ada" || zset.Values[1].Label != "linus" || zset.Values[2].Label != "grace" {
		t.Fatalf("zset ZRANGE order = %+v, want ascending by score", zset.Values)
	}
	if !strings.Contains(zset.Values[0].Value, "85") || !strings.Contains(zset.Values[2].Value, "100") {
		t.Errorf("zset scores drifted: %+v", zset.Values)
	}

	// The pipelined counter: 25 INCRs, one round trip, an exact total.
	if run.Counter.PipelineSize != demos.RedisDemoCounterPipeline {
		t.Errorf("pipeline size = %d, want %d", run.Counter.PipelineSize, demos.RedisDemoCounterPipeline)
	}
	if run.Counter.Total != int64(demos.RedisDemoCounterPipeline) {
		t.Errorf("counter total = %d, want exactly %d", run.Counter.Total, demos.RedisDemoCounterPipeline)
	}

	// The TTL beat as data: the configured EX, a live positive countdown
	// read back (asserting the store's answer, never a wall clock), and
	// the NX pair's two answers.
	if run.TTL.TTLSeconds != demos.RedisDemoTTLSeconds {
		t.Errorf("ttl configured = %d, want %d", run.TTL.TTLSeconds, demos.RedisDemoTTLSeconds)
	}
	if run.TTL.TTLFirstRead <= 0 || run.TTL.TTLFirstRead > int64(demos.RedisDemoTTLSeconds) {
		t.Errorf("ttl first read = %d, want within (0, %d]", run.TTL.TTLFirstRead, demos.RedisDemoTTLSeconds)
	}
	if !run.TTL.NXFirstOK {
		t.Error("the first SET NX must acquire the lock")
	}
	if run.TTL.NXSecondOK {
		t.Error("the second SET NX must be refused — one holder only")
	}

	// The pub/sub round-trip: one receiver, identical payloads.
	if run.PubSub.Receivers != 1 {
		t.Errorf("publish receivers = %d, want 1", run.PubSub.Receivers)
	}
	if run.PubSub.ReceivedPayload != demos.RedisDemoPubSubPayload {
		t.Errorf("received %q, want the published %q", run.PubSub.ReceivedPayload, demos.RedisDemoPubSubPayload)
	}

	// The cleanup beat: 8 keys (five structures + counter + flash + lock)
	// removed, and the keyspace provably empty afterwards.
	if run.Cleanup.KeysRemoved != 8 {
		t.Errorf("cleanup removed %d keys, want the 8 the walk created", run.Cleanup.KeysRemoved)
	}
	client := NewClient(testRedisAddr())
	defer client.Close()
	var leftover []string
	iterator := client.Scan(ctx, 0, namespace+":*", 100).Iterator()
	for iterator.Next(ctx) {
		leftover = append(leftover, iterator.Val())
	}
	if err := iterator.Err(); err != nil {
		t.Fatalf("verify cleanup: %v", err)
	}
	if len(leftover) != 0 {
		t.Errorf("leftover keys after cleanup: %v", leftover)
	}
}

func TestRedisPlaygroundRunnerIsRepeatableUnderASecondNamespace(t *testing.T) {
	runner, namespace := newTestRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	first, err := runner.Run(ctx, namespace)
	if err != nil {
		t.Fatalf("first walk: %v", err)
	}
	secondNamespace, err := demos.NewRedisDemoNamespace()
	if err != nil {
		t.Fatalf("mint second namespace: %v", err)
	}
	secondClient := NewClient(testRedisAddr())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		clearPattern(ctx, t, secondClient, secondNamespace+":*")
		secondClient.Close()
	})
	second, err := runner.Run(ctx, secondNamespace)
	if err != nil {
		t.Fatalf("second walk: %v", err)
	}

	// Namespaces are independent: the second walk's counter starts from
	// zero (the DEL + pipeline discipline), not from the first walk's
	// residue.
	if first.Counter.Total != second.Counter.Total {
		t.Errorf("totals drifted between walks: %d then %d", first.Counter.Total, second.Counter.Total)
	}
	if second.Counter.Total != int64(demos.RedisDemoCounterPipeline) {
		t.Errorf("second walk total = %d, want the exact pipeline size", second.Counter.Total)
	}
}

func TestRedisPlaygroundRunnerDegradesFastOnABadAddress(t *testing.T) {
	// A constructed bad address: nothing listens there, and the client's
	// dial timeout turns refusal into an error in seconds — the honest
	// service_unavailable path, never a hang.
	runner := NewPlaygroundRunner(NewClient("127.0.0.1:1"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	started := time.Now()
	_, err := runner.Run(ctx, "gdg:demo:redis:degradation")
	if err == nil {
		t.Fatal("a bad address must fail the walk, not serve a stale answer")
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Errorf("degradation took %v, want it inside the command timeouts", elapsed)
	}
	if !strings.Contains(err.Error(), "SET") {
		t.Errorf("the error should name the first failing command: %v", err)
	}
}

func TestRedisPlaygroundRunnerPropagatesContextCancellation(t *testing.T) {
	runner, namespace := newTestRunner(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runner.Run(ctx, namespace)
	if err == nil {
		t.Fatal("a canceled context must fail the walk")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error should unwrap to context.Canceled, got: %v", err)
	}
}

func TestRedisPlaygroundRunnerRefusesABlankNamespace(t *testing.T) {
	client := NewClient(testRedisAddr())
	defer client.Close()
	runner := NewPlaygroundRunner(client)
	if _, err := runner.Run(context.Background(), "  "); err == nil {
		t.Fatal("a blank namespace must be refused before any command runs")
	}
}
