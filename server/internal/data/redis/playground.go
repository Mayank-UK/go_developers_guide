// Package redis is the guide's Redis data layer: the go-redis v9 client
// factory (one place for timeout policy) and the playground adapter that
// runs topic 20's beats against the RUNNING stack Redis. The domain port
// (demos.RedisPlaygroundRunner in internal/domain/demos) speaks plain
// result rows; an unreachable Redis surfaces here as an ordinary error
// that the handler maps to the typed service_unavailable envelope — the
// optional-service discipline, never a crash.
package redis

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"guide/internal/domain/demos"
)

// Command timeouts keep the optional-service discipline FAST: when the
// address points at nothing, the first command fails within these bounds
// instead of the TCP stack's defaults, and the request degrades to the
// 503 envelope in seconds.
const (
	dialTimeout  = 2 * time.Second
	readTimeout  = 2 * time.Second
	writeTimeout = 2 * time.Second

	// pubsubWait bounds how long the subscriber waits for the published
	// message: a healthy store delivers in milliseconds, so this is only
	// ever the honest failure path for a store that stopped answering.
	pubsubWait = 2 * time.Second
)

// NewClient builds the process's go-redis client for an address — the
// compose network's "redis:6379" from inside the stack (the host's
// published slot is 6380). Construction is LAZY: no connection is made,
// so the app boots with Redis down and the first COMMAND pays the
// connect, failing inside the timeouts above rather than hanging a
// boot or a worker.
func NewClient(addr string) *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr:         addr,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	})
}

// PlaygroundRunner walks topic 20's demo beats over one client. It is a
// thin adapter: every method runs the commands the topic teaches and
// returns what the store answered; the payload builder (in the domain)
// derives every proof sentence from these rows.
type PlaygroundRunner struct {
	client *goredis.Client
}

// NewPlaygroundRunner wires the runner over its client.
func NewPlaygroundRunner(client *goredis.Client) *PlaygroundRunner {
	return &PlaygroundRunner{client: client}
}

// Run walks every beat under namespace and reports the store's answers:
// the five structures, the pipelined INCR counter, the TTL/NX semantics
// read as data, and one real pub/sub round-trip — then SCAN+DELs every
// key it created. Beats run in teaching order; the first failure aborts
// the walk with the command's context wrapped (%w), for the handler to
// map onto the honest 503.
func (r *PlaygroundRunner) Run(ctx context.Context, namespace string) (demos.RedisPlaygroundRun, error) {
	if strings.TrimSpace(namespace) == "" {
		return demos.RedisPlaygroundRun{}, fmt.Errorf("redis playground: namespace must not be blank")
	}
	key := func(suffix string) string { return namespace + ":" + suffix }

	stringRow, err := r.runString(ctx, key("greeting"))
	if err != nil {
		return demos.RedisPlaygroundRun{}, err
	}
	hashRow, err := r.runHash(ctx, key("post"))
	if err != nil {
		return demos.RedisPlaygroundRun{}, err
	}
	listRow, err := r.runList(ctx, key("timeline"))
	if err != nil {
		return demos.RedisPlaygroundRun{}, err
	}
	setRow, err := r.runSet(ctx, key("tags"))
	if err != nil {
		return demos.RedisPlaygroundRun{}, err
	}
	zsetRow, err := r.runZSet(ctx, key("leaderboard"))
	if err != nil {
		return demos.RedisPlaygroundRun{}, err
	}
	counter, err := r.runCounter(ctx, key("views"))
	if err != nil {
		return demos.RedisPlaygroundRun{}, err
	}
	ttl, err := r.runTTL(ctx, key("flash"), key("lock"))
	if err != nil {
		return demos.RedisPlaygroundRun{}, err
	}
	pubsub, err := r.runPubSub(ctx, namespace+":events", demos.RedisDemoPubSubPayload)
	if err != nil {
		return demos.RedisPlaygroundRun{}, err
	}
	cleanup, err := r.cleanup(ctx, namespace)
	if err != nil {
		return demos.RedisPlaygroundRun{}, err
	}

	return demos.RedisPlaygroundRun{
		Namespace:  namespace,
		Structures: []demos.RedisStructureRun{stringRow, hashRow, listRow, setRow, zsetRow},
		Counter:    counter,
		TTL:        ttl,
		PubSub:     pubsub,
		Cleanup:    cleanup,
	}, nil
}

// runString sets the greeting with its EX inline — value and deadline in
// ONE atomic command — and reads it straight back.
func (r *PlaygroundRunner) runString(ctx context.Context, key string) (demos.RedisStructureRun, error) {
	if err := r.client.Set(ctx, key, demos.RedisDemoGreeting,
		time.Duration(demos.RedisDemoTTLSeconds)*time.Second).Err(); err != nil {
		return demos.RedisStructureRun{}, fmt.Errorf("SET %s: %w", key, err)
	}
	value, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return demos.RedisStructureRun{}, fmt.Errorf("GET %s: %w", key, err)
	}
	return demos.RedisStructureRun{
		Kind:        "string",
		Key:         key,
		Description: "the workhorse: one binary-safe value up to 512 MB — caches, counters, whole serialized payloads",
		Commands: []string{
			fmt.Sprintf("SET %s %q EX %d", key, demos.RedisDemoGreeting, demos.RedisDemoTTLSeconds),
			fmt.Sprintf("GET %s", key),
		},
		Outcome: "SET answered OK (value and TTL in one command); GET read the value straight back.",
		Values:  []demos.RedisValueRow{{Label: "GET", Value: value}},
	}, nil
}

// runHash writes the whole object in one HSET and reads it back whole
// with HGETALL — fields sorted for display, because a hash's field order
// is the store's business, not a promise.
func (r *PlaygroundRunner) runHash(ctx context.Context, key string) (demos.RedisStructureRun, error) {
	added, err := r.client.HSet(ctx, key, demos.RedisDemoHashFields).Result()
	if err != nil {
		return demos.RedisStructureRun{}, fmt.Errorf("HSET %s: %w", key, err)
	}
	whole, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return demos.RedisStructureRun{}, fmt.Errorf("HGETALL %s: %w", key, err)
	}
	fields := make([]string, 0, len(whole))
	for field := range whole {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	values := make([]demos.RedisValueRow, 0, len(fields))
	for _, field := range fields {
		values = append(values, demos.RedisValueRow{Label: field, Value: whole[field]})
	}
	return demos.RedisStructureRun{
		Kind:        "hash",
		Key:         key,
		Description: "one key, many field→value pairs — the object-shaped value, addressed per field or read whole",
		Commands: []string{
			fmt.Sprintf("HSET %s title … eviction …", key),
			fmt.Sprintf("HGETALL %s", key),
		},
		Outcome: fmt.Sprintf("HSET answered %d new fields; HGETALL returned all %d — the object came back whole.", added, len(whole)),
		Values:  values,
	}, nil
}

// runList pushes three elements (each lands at the HEAD) and reads the
// range back — the push-order inversion IS the lesson.
func (r *PlaygroundRunner) runList(ctx context.Context, key string) (demos.RedisStructureRun, error) {
	length, err := r.client.LPush(ctx, key, demos.RedisDemoListPushes).Result()
	if err != nil {
		return demos.RedisStructureRun{}, fmt.Errorf("LPUSH %s: %w", key, err)
	}
	elements, err := r.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return demos.RedisStructureRun{}, fmt.Errorf("LRANGE %s: %w", key, err)
	}
	values := make([]demos.RedisValueRow, 0, len(elements))
	for i, element := range elements {
		values = append(values, demos.RedisValueRow{Label: fmt.Sprintf("LRANGE %d", i), Value: element})
	}
	return demos.RedisStructureRun{
		Kind:        "list",
		Key:         key,
		Description: "an ordered sequence addressed from both ends — feeds, timelines, queues",
		Commands: []string{
			fmt.Sprintf("LPUSH %s %s", key, strings.Join(demos.RedisDemoListPushes, " ")),
			fmt.Sprintf("LRANGE %s 0 -1", key),
		},
		Outcome: fmt.Sprintf("LPUSH answered %d; LRANGE read them back newest-first — lists push at the head, so the order inverts.", length),
		Values:  values,
	}, nil
}

// runSet adds four arguments of which one is a duplicate: SADD answers 3
// because uniqueness is the structure's job, and SMEMBERS (sorted here
// for a deterministic display; the store makes no order promise) returns
// the distinct members.
func (r *PlaygroundRunner) runSet(ctx context.Context, key string) (demos.RedisStructureRun, error) {
	added, err := r.client.SAdd(ctx, key, demos.RedisDemoSetMembers).Result()
	if err != nil {
		return demos.RedisStructureRun{}, fmt.Errorf("SADD %s: %w", key, err)
	}
	members, err := r.client.SMembers(ctx, key).Result()
	if err != nil {
		return demos.RedisStructureRun{}, fmt.Errorf("SMEMBERS %s: %w", key, err)
	}
	sort.Strings(members)
	values := make([]demos.RedisValueRow, 0, len(members))
	for _, member := range members {
		values = append(values, demos.RedisValueRow{Label: "member", Value: member})
	}
	return demos.RedisStructureRun{
		Kind:        "set",
		Key:         key,
		Description: "an unordered, unique membership — tags, presence, set algebra (SINTER/SUNION/SDIFF)",
		Commands: []string{
			fmt.Sprintf("SADD %s %s", key, strings.Join(demos.RedisDemoSetMembers, " ")),
			fmt.Sprintf("SMEMBERS %s", key),
		},
		Outcome: fmt.Sprintf("SADD answered %d of 4 arguments — the duplicate was refused by the structure; SMEMBERS returned the %d distinct members.", added, len(members)),
		Values:  values,
	}, nil
}

// runZSet writes members with scores in DESCENDING score order on
// purpose, then ZRANGE WITHSCORES reads them back ascending — the sort
// belongs to the structure, not to the query.
func (r *PlaygroundRunner) runZSet(ctx context.Context, key string) (demos.RedisStructureRun, error) {
	scored := make([]goredis.Z, 0, len(demos.RedisDemoZSetScores))
	for _, member := range demos.RedisDemoZSetScores {
		scored = append(scored, goredis.Z{Member: member.Member, Score: member.Score})
	}
	added, err := r.client.ZAdd(ctx, key, scored...).Result()
	if err != nil {
		return demos.RedisStructureRun{}, fmt.Errorf("ZADD %s: %w", key, err)
	}
	ascending, err := r.client.ZRangeWithScores(ctx, key, 0, -1).Result()
	if err != nil {
		return demos.RedisStructureRun{}, fmt.Errorf("ZRANGE %s WITHSCORES: %w", key, err)
	}
	values := make([]demos.RedisValueRow, 0, len(ascending))
	for _, member := range ascending {
		values = append(values, demos.RedisValueRow{
			// Member is interface{} on the wire; the demo only ever writes
			// strings, and %v renders one without a panic-capable assertion.
			Label: fmt.Sprintf("%v", member.Member),
			Value: fmt.Sprintf("score %v", member.Score),
		})
	}
	return demos.RedisStructureRun{
		Kind:        "zset",
		Key:         key,
		Description: "members carrying scores, kept ordered by them — leaderboards, ranking, sliding windows",
		Commands: []string{
			fmt.Sprintf("ZADD %s 100 grace 85 ada 97 linus", key),
			fmt.Sprintf("ZRANGE %s 0 -1 WITHSCORES", key),
		},
		Outcome: fmt.Sprintf("ZADD placed %d members; ZRANGE WITHSCORES read them back in SCORE order — written 100, 85, 97, returned 85, 97, 100.", added),
		Values:  values,
	}, nil
}

// runCounter proves INCR atomicity with a pipeline: DEL for a fresh key,
// then RedisDemoCounterPipeline INCRs queued and executed as ONE round
// trip, then a GET whose answer must be exactly the pipeline's size —
// the increments are atomic on the server's single command thread, so no
// application-side lock exists to get wrong.
func (r *PlaygroundRunner) runCounter(ctx context.Context, key string) (demos.RedisCounterRun, error) {
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return demos.RedisCounterRun{}, fmt.Errorf("DEL %s: %w", key, err)
	}
	pipe := r.client.Pipeline()
	for range demos.RedisDemoCounterPipeline {
		pipe.Incr(ctx, key)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return demos.RedisCounterRun{}, fmt.Errorf("pipeline %d INCRs on %s: %w", demos.RedisDemoCounterPipeline, key, err)
	}
	total, err := r.client.Get(ctx, key).Int64()
	if err != nil {
		return demos.RedisCounterRun{}, fmt.Errorf("GET %s: %w", key, err)
	}
	return demos.RedisCounterRun{
		Key:          key,
		PipelineSize: demos.RedisDemoCounterPipeline,
		Commands: []string{
			fmt.Sprintf("DEL %s", key),
			fmt.Sprintf("PIPELINE · %d× INCR %s", demos.RedisDemoCounterPipeline, key),
			fmt.Sprintf("GET %s", key),
		},
		Total: total,
	}, nil
}

// runTTL reads expiry semantics as DATA: the SET carries its EX inline,
// TTL reports the live countdown right back, and the SET NX pair on the
// lock key shows the two-command lock — first acquire OK, second refused
// nil. Nothing sleeps: eviction is the store's clock, not this code's.
func (r *PlaygroundRunner) runTTL(ctx context.Context, flashKey, lockKey string) (demos.RedisTTLRun, error) {
	flash := "here for 30 seconds — the store's clock evicts me"
	if err := r.client.Set(ctx, flashKey, flash,
		time.Duration(demos.RedisDemoTTLSeconds)*time.Second).Err(); err != nil {
		return demos.RedisTTLRun{}, fmt.Errorf("SET %s: %w", flashKey, err)
	}
	firstRead, err := r.client.TTL(ctx, flashKey).Result()
	if err != nil {
		return demos.RedisTTLRun{}, fmt.Errorf("TTL %s: %w", flashKey, err)
	}
	if firstRead <= 0 {
		// -1 means no expiry, -2 means no key — both contradict the SET
		// that just succeeded, so the beat refuses to lie about either.
		return demos.RedisTTLRun{}, fmt.Errorf("TTL %s answered %v, want a positive countdown", flashKey, firstRead)
	}
	nxFirst, err := r.client.SetNX(ctx, lockKey, "owner-1", 0).Result()
	if err != nil {
		return demos.RedisTTLRun{}, fmt.Errorf("SET %s NX: %w", lockKey, err)
	}
	nxSecond, err := r.client.SetNX(ctx, lockKey, "owner-2", 0).Result()
	if err != nil {
		return demos.RedisTTLRun{}, fmt.Errorf("SET %s NX (second holder): %w", lockKey, err)
	}
	return demos.RedisTTLRun{
		Key:          flashKey,
		TTLSeconds:   demos.RedisDemoTTLSeconds,
		TTLFirstRead: int64(firstRead.Seconds()),
		NXFirstOK:    nxFirst,
		NXSecondOK:   nxSecond,
		Commands: []string{
			fmt.Sprintf("SET %s %q EX %d", flashKey, flash, demos.RedisDemoTTLSeconds),
			fmt.Sprintf("TTL %s", flashKey),
			fmt.Sprintf("SET %s \"owner-1\" NX", lockKey),
			fmt.Sprintf("SET %s \"owner-2\" NX", lockKey),
		},
	}, nil
}

// runPubSub performs one REAL round-trip: subscribe on a dedicated
// connection, wait for the store's subscription confirmation BEFORE
// publishing (a publish that races ahead of the subscribe reaches
// nobody), publish from the pooled connection, and collect what the
// subscriber received — bounded by a context deadline, with the
// goroutine always released by the same cancel.
func (r *PlaygroundRunner) runPubSub(ctx context.Context, channel, payload string) (demos.RedisPubSubRun, error) {
	subscriber := r.client.Subscribe(ctx, channel)
	defer subscriber.Close()

	ack, err := subscriber.Receive(ctx)
	if err != nil {
		return demos.RedisPubSubRun{}, fmt.Errorf("SUBSCRIBE %s: %w", channel, err)
	}
	confirmation, ok := ack.(*goredis.Subscription)
	if !ok || confirmation.Kind != "subscribe" {
		return demos.RedisPubSubRun{}, fmt.Errorf("SUBSCRIBE %s: unexpected confirmation %T", channel, ack)
	}

	msgCtx, cancel := context.WithTimeout(ctx, pubsubWait)
	defer cancel()

	var received string
	var receiveErr error
	receivedMessage := make(chan struct{})
	go func() {
		defer close(receivedMessage)
		message, err := subscriber.ReceiveMessage(msgCtx)
		if err != nil {
			receiveErr = err
			return
		}
		received = message.Payload
	}()

	receivers, err := r.client.Publish(ctx, channel, payload).Result()
	if err != nil {
		cancel()
		<-receivedMessage
		return demos.RedisPubSubRun{}, fmt.Errorf("PUBLISH %s: %w", channel, err)
	}

	select {
	case <-receivedMessage:
	case <-msgCtx.Done():
		return demos.RedisPubSubRun{}, fmt.Errorf("wait for the published message on %s: %w", channel, msgCtx.Err())
	}
	if receiveErr != nil {
		return demos.RedisPubSubRun{}, fmt.Errorf("receive the published message on %s: %w", channel, receiveErr)
	}
	return demos.RedisPubSubRun{
		Channel:         channel,
		Payload:         payload,
		Commands:        []string{fmt.Sprintf("SUBSCRIBE %s", channel), fmt.Sprintf("PUBLISH %s %q", channel, payload)},
		Receivers:       receivers,
		ReceivedPayload: received,
	}, nil
}

// cleanup removes every key the run created: SCAN (never KEYS — SCAN
// streams in batches and does not block the store) under the namespace's
// pattern, then one DEL for the whole batch. The channel is not a key,
// so the subscriber's close in runPubSub is its whole cleanup.
func (r *PlaygroundRunner) cleanup(ctx context.Context, namespace string) (demos.RedisCleanupRun, error) {
	match := namespace + ":*"
	var keys []string
	iterator := r.client.Scan(ctx, 0, match, 100).Iterator()
	for iterator.Next(ctx) {
		keys = append(keys, iterator.Val())
	}
	if err := iterator.Err(); err != nil {
		return demos.RedisCleanupRun{}, fmt.Errorf("SCAN %s: %w", match, err)
	}
	var removed int64
	if len(keys) > 0 {
		deleted, err := r.client.Del(ctx, keys...).Result()
		if err != nil {
			return demos.RedisCleanupRun{}, fmt.Errorf("DEL %s keys: %w", namespace, err)
		}
		removed = deleted
	}
	return demos.RedisCleanupRun{
		KeysRemoved: removed,
		Commands: []string{
			fmt.Sprintf("SCAN 0 MATCH %q COUNT 100", match),
			fmt.Sprintf("DEL %s …", namespace),
		},
	}, nil
}
