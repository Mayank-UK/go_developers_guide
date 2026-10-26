// Package ormprobe is topic 18's measured exhibit: one dataset (the
// ormprobe_* tables it creates and seeds idempotently on the guide's own
// PostgreSQL), one logical job — "the posts page with its tags" — run
// three ways: raw pgx (the baseline JOIN), sqlc (the same SQL as
// generated, typed code), and GORM (a per-post association loop that
// exhibits the N+1 query problem, then Preload, its structural fix).
// Every strategy's SQL statements are COUNTED, each through the
// observability seam its library actually offers: a pgx pool tracer, the
// explicit call sites generated code gives sqlc, and a GORM session
// logger. Counts are the payload's durable facts; durations are the live
// observation of the run that served them, never a promise.
//
// The probe owns the whole lifecycle of its tables (schema embedded from
// schema.sql — the same file sqlc type-checks against) and is constructed
// with a DSN, opening its pool per call: a database that is down degrades
// the demo (a typed 503 at the handler), never the boot.
package ormprobe

import (
	"context"
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"guide/internal/domain/demos"
)

//go:embed schema.sql
var schemaDDL string

// Runner runs the orm-probe against a PostgreSQL reachable at dsn.
type Runner struct {
	dsn string
}

// NewRunner wires the probe over a DSN (GDG_DATABASE_URL at the
// composition root). Construction touches nothing — the pool opens per
// Probe call — so an unreachable database is a per-request degradation,
// never a boot failure.
func NewRunner(dsn string) *Runner {
	return &Runner{dsn: dsn}
}

// Compile-time proof the runner satisfies the demo port.
var _ demos.ORMProbeRunner = (*Runner)(nil)

// sqlcMeasuredCallSites is the sqlc trial's statement count, derived from
// its code: the measured phase contains exactly one generated query call.
const sqlcMeasuredCallSites = 1

// probePingTimeout bounds the per-call pool's first contact: the demo must
// degrade in seconds when PostgreSQL is down, not hang a request.
const probePingTimeout = 10 * time.Second

// probe statementCounter is a pgx QueryTracer that records every statement
// crossing the connections it is attached to — the counting middleware the
// baseline reads and the tests cross-check the other libraries against.
// It is attached at pool construction, before any connection exists, so it
// is established race-free.
type statementCounter struct {
	mu         sync.Mutex
	statements []string
}

// TraceQueryStart implements pgx.QueryTracer: record the statement.
func (c *statementCounter) TraceQueryStart(
	ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData,
) context.Context {
	c.mu.Lock()
	c.statements = append(c.statements, normalizeSQL(data.SQL))
	c.mu.Unlock()
	return ctx
}

// TraceQueryEnd implements pgx.QueryTracer: nothing to record.
func (c *statementCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// Reset drops every recorded statement, starting a fresh measurement
// window. The probe runs its phases sequentially; resetting between phases
// makes the pool-wide tracer a per-strategy counter.
func (c *statementCounter) Reset() {
	c.mu.Lock()
	c.statements = nil
	c.mu.Unlock()
}

// Count reports how many statements the current window recorded.
func (c *statementCounter) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.statements)
}

// Samples copies the current window's statements — the receipts.
func (c *statementCounter) Samples() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.statements...)
}

// normalizeSQL collapses whitespace so measured samples render as one
// deterministic line regardless of how each library spells its SQL.
func normalizeSQL(sql string) string {
	return strings.Join(strings.Fields(sql), " ")
}

// openCountedPool dials the probe's own small pool with the counting
// tracer attached. It shares nothing with the process-wide pool (the probe
// is a short-lived per-call guest on the guide's PostgreSQL, and it must
// not inflate or wait behind the application's connections).
func openCountedPool(ctx context.Context, dsn string, tracer *statementCounter) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse the probe's dsn: %w", err)
	}
	config.ConnConfig.Tracer = tracer
	config.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open the probe's pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, probePingTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping the probe's database: %w", err)
	}
	return pool, nil
}

// ensureSchema creates the ormprobe_* tables from the embedded schema.sql
// — the same file sqlc reads, so generated code and live tables cannot
// drift. IF NOT EXISTS makes every call after the first a no-op.
func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schemaDDL); err != nil {
		return fmt.Errorf("create the probe's tables: %w", err)
	}
	return nil
}

// The deterministic seed: twelve posts, a five-tag vocabulary, two tags
// per post — small enough to read, large enough that the N+1 loop's
// statement count (13) is unmistakably not the baseline's (1).
const (
	ormProbePostCount = 12
	ormProbeTagCount  = 5
)

// ormProbeTagVocabulary is the seed's closed tag vocabulary.
var ormProbeTagVocabulary = [ormProbeTagCount]string{"go", "sql", "orm", "sqlc", "gorm"}

// seedBaseTime anchors the seeded timeline (and every created_at after
// it, stepped 36 hours per post) — fixed literals, so the dataset is
// byte-identical wherever the probe runs.
var seedBaseTime = time.Date(2026, 2, 2, 9, 0, 0, 0, time.UTC)

// ensureSeed inserts the fixed dataset if it is not present — every
// statement parameterized and idempotent (ON CONFLICT DO NOTHING), the
// same discipline the posts store's Seed runs on the guide's real tables.
func ensureSeed(ctx context.Context, pool *pgxpool.Pool) error {
	vocabulary := ormProbeTagVocabulary[:]
	if _, err := pool.Exec(ctx, `
		INSERT INTO ormprobe_tags (name)
		SELECT * FROM unnest($1::text[])
		ON CONFLICT (name) DO NOTHING`, vocabulary); err != nil {
		return fmt.Errorf("seed the tag vocabulary: %w", err)
	}

	ids := make([]int64, ormProbePostCount)
	titles := make([]string, ormProbePostCount)
	bodies := make([]string, ormProbePostCount)
	created := make([]time.Time, ormProbePostCount)
	for i := range ids {
		ids[i] = int64(i + 1)
		titles[i] = fmt.Sprintf("ORM probe post %02d", i+1)
		bodies[i] = fmt.Sprintf(
			"Seeded post %02d of the orm-probe dataset: twelve posts, two tags each, one honest measurement.", i+1)
		created[i] = seedBaseTime.Add(time.Duration(i) * 36 * time.Hour)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO ormprobe_posts (id, title, body, created_at)
		SELECT * FROM unnest($1::bigint[], $2::text[], $3::text[], $4::timestamptz[])
		ON CONFLICT (id) DO NOTHING`, ids, titles, bodies, created); err != nil {
		return fmt.Errorf("seed the probe posts: %w", err)
	}

	postIDs := make([]int64, 0, 2*ormProbePostCount)
	tagNames := make([]string, 0, 2*ormProbePostCount)
	for i := range ids {
		for _, offset := range [2]int{0, 2} {
			postIDs = append(postIDs, ids[i])
			tagNames = append(tagNames, ormProbeTagVocabulary[(i+offset)%ormProbeTagCount])
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO ormprobe_post_tags (post_id, tag_name)
		SELECT * FROM unnest($1::bigint[], $2::text[])
		ON CONFLICT DO NOTHING`, postIDs, tagNames); err != nil {
		return fmt.Errorf("seed the tag edges: %w", err)
	}

	// Explicit ids do not advance a GENERATED BY DEFAULT identity
	// sequence — the next insert the database itself ids would mint 1 and
	// collide with the seed. Sync the sequence to the seeded maximum (the
	// textbook companion to explicit-id seeding), so the GORM trial's
	// inserts — which omit the id and let the identity mint it — land past
	// the dataset. Idempotent by nature: setval to the same maximum again.
	if _, err := pool.Exec(ctx, `
		SELECT setval(pg_get_serial_sequence('ormprobe_posts', 'id'),
		              COALESCE((SELECT max(id) FROM ormprobe_posts), 1))`); err != nil {
		return fmt.Errorf("sync the posts identity sequence: %w", err)
	}
	return nil
}

// readCounts reports the dataset the trials are about to measure.
func readCounts(ctx context.Context, pool *pgxpool.Pool) (demos.ORMProbeDatasetFacts, error) {
	var dataset demos.ORMProbeDatasetFacts
	if err := pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM ormprobe_posts),
		       (SELECT count(*) FROM ormprobe_tags),
		       (SELECT count(*) FROM ormprobe_post_tags)`).Scan(
		&dataset.Posts, &dataset.Tags, &dataset.Edges); err != nil {
		return demos.ORMProbeDatasetFacts{}, fmt.Errorf("count the probe dataset: %w", err)
	}
	return dataset, nil
}

// baselineJoinSQL is the raw-pgx baseline's ONE statement: the page AND
// its tags in a single round trip — deliberately the same statement the
// sqlc query file carries, so the comparison is SQL-identical before it
// is library-different.
const baselineJoinSQL = `
SELECT p.id, p.title, p.body, p.created_at,
       (COALESCE(array_agg(pt.tag_name ORDER BY pt.tag_name)
                 FILTER (WHERE pt.tag_name IS NOT NULL), '{}'))::text[] AS tags
FROM ormprobe_posts p
LEFT JOIN ormprobe_post_tags pt ON pt.post_id = p.id
GROUP BY p.id, p.title, p.body, p.created_at
ORDER BY p.id`

// probeRow is the strategy-neutral shape every trial normalizes to for
// the apples-to-apples comparison.
type probeRow struct {
	ID    int64
	Title string
	Tags  []string
}

// runBaseline executes the one-statement read the way topic 17 taught it.
func runBaseline(ctx context.Context, pool *pgxpool.Pool) ([]probeRow, error) {
	rows, err := pool.Query(ctx, baselineJoinSQL)
	if err != nil {
		return nil, fmt.Errorf("run the pgx baseline: %w", err)
	}
	defer rows.Close()
	results := make([]probeRow, 0, ormProbePostCount)
	for rows.Next() {
		var row probeRow
		var body string
		var createdAt time.Time
		if err := rows.Scan(&row.ID, &row.Title, &body, &createdAt, &row.Tags); err != nil {
			return nil, fmt.Errorf("scan a baseline row: %w", err)
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// Probe implements the demo port: open, ensure schema and seed, run the
// four measured trials, run the unit-of-work exhibit, and report whether
// every strategy answered with the same rows.
func (r *Runner) Probe(ctx context.Context) (demos.ORMProbeObservation, error) {
	counter := &statementCounter{}
	pool, err := openCountedPool(ctx, r.dsn, counter)
	if err != nil {
		return demos.ORMProbeObservation{}, err
	}
	defer pool.Close()

	if err := ensureSchema(ctx, pool); err != nil {
		return demos.ORMProbeObservation{}, err
	}
	if err := ensureSeed(ctx, pool); err != nil {
		return demos.ORMProbeObservation{}, err
	}
	dataset, err := readCounts(ctx, pool)
	if err != nil {
		return demos.ORMProbeObservation{}, err
	}

	// Baseline: raw pgx, one explicit statement, counted by the tracer.
	counter.Reset()
	started := time.Now()
	baselineRows, err := runBaseline(ctx, pool)
	if err != nil {
		return demos.ORMProbeObservation{}, err
	}
	baselineFacts := demos.ORMProbeTrialFacts{
		Strategy:         demos.ORMProbeStrategyPgx,
		Statements:       counter.Count(),
		Rows:             len(baselineRows),
		DurationMicros:   time.Since(started).Microseconds(),
		SampleStatements: counter.Samples(),
	}

	// sqlc: the generated call — the measured phase's ONLY statement call
	// site, which is why its count is a code fact rather than a metered
	// one (the tracer window, reset here, records the same single
	// statement crossing the pool).
	counter.Reset()
	started = time.Now()
	sqlcRows, err := New(pool).ListPostsWithTags(ctx)
	if err != nil {
		return demos.ORMProbeObservation{}, fmt.Errorf("run the sqlc trial: %w", err)
	}
	sqlcFacts := demos.ORMProbeTrialFacts{
		Strategy:         demos.ORMProbeStrategySQLC,
		Statements:       sqlcMeasuredCallSites,
		Rows:             len(sqlcRows),
		DurationMicros:   time.Since(started).Microseconds(),
		SampleStatements: []string{normalizeSQL(listPostsWithTags)},
	}

	// GORM: same pool through pgx's database/sql adapter — one pool, three
	// APIs. Closing the adapter leaves the pool alive for its own Close.
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	gormDB, err := openGORM(sqlDB)
	if err != nil {
		return demos.ORMProbeObservation{}, err
	}
	lazyRows, lazyFacts, err := runGormLazy(ctx, gormDB)
	if err != nil {
		return demos.ORMProbeObservation{}, err
	}
	eagerRows, eagerFacts, err := runGormEager(ctx, gormDB)
	if err != nil {
		return demos.ORMProbeObservation{}, err
	}
	unitFacts, err := runUnitOfWork(ctx, gormDB)
	if err != nil {
		return demos.ORMProbeObservation{}, err
	}

	sqlcRowsNeutral := make([]probeRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		sqlcRowsNeutral = append(sqlcRowsNeutral, probeRow{ID: row.ID, Title: row.Title, Tags: row.Tags})
	}
	return demos.ORMProbeObservation{
		Dataset: dataset,
		Trials:  []demos.ORMProbeTrialFacts{baselineFacts, sqlcFacts, lazyFacts, eagerFacts},
		ResultsAgree: rowsAgree(
			baselineRows, sqlcRowsNeutral, lazyRows, eagerRows),
		UnitOfWork: unitFacts,
	}, nil
}

// rowsAgree reports whether every run answered the same rows in the same
// order — the apples-to-apples proof that the three styles measured are
// solving the same problem. Tags compare as SETS: each API returns them
// in its own order (the SQL aggregates alphabetically; GORM returns rows
// in scan order), and tag ORDER is not part of the logical answer.
func rowsAgree(runs ...[]probeRow) bool {
	if len(runs) < 2 {
		return true
	}
	first := runs[0]
	for _, run := range runs[1:] {
		if len(run) != len(first) {
			return false
		}
		for i := range first {
			if first[i].ID != run[i].ID || first[i].Title != run[i].Title {
				return false
			}
			if tagSet(first[i].Tags) != tagSet(run[i].Tags) {
				return false
			}
		}
	}
	return true
}

// tagSet renders a tag collection order-insensitively for comparison.
func tagSet(tags []string) string {
	sorted := append([]string(nil), tags...)
	sort.Strings(sorted)
	return strings.Join(sorted, "\x00")
}
