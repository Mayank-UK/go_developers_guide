// The topic 17 demo runner: ONE type over the shared pool implementing
// all six databases-sql demo ports (the payloads' pure builders live in
// internal/domain/demos). Every statement is parameterized; every
// choreography that writes runs on scratch rows and leaves the database
// exactly as it found it (transactions rolled back or their rows
// removed); the two-connection race dance is channel- and poll-driven,
// never sleep-driven, so its observed sequence is deterministic.
package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	postsdata "guide/internal/data/posts"
	"guide/internal/domain/demos"
)

// SQLDemoRunner runs the six databases-sql demos over the shared pool.
type SQLDemoRunner struct {
	pool *pgxpool.Pool
}

// NewSQLDemoRunner wires the runner over the process-wide pool.
func NewSQLDemoRunner(pool *pgxpool.Pool) *SQLDemoRunner {
	return &SQLDemoRunner{pool: pool}
}

// Compile-time proofs the runner satisfies every demo port.
var (
	_ demos.SQLSafetyRunner = (*SQLDemoRunner)(nil)
	_ demos.SQLTxRunner     = (*SQLDemoRunner)(nil)
	_ demos.SQLLabRunner    = (*SQLDemoRunner)(nil)
	_ demos.SQLSearchRunner = (*SQLDemoRunner)(nil)
	_ demos.SQLRaceRunner   = (*SQLDemoRunner)(nil)
	_ demos.SQLAuditRunner  = (*SQLDemoRunner)(nil)
)

// ObserveSQLSafety executes the parameterized lookup once per attack
// input ($1 carries the hostile string as DATA) and reports what the
// database saw. The concatenated form is never executed anywhere — the
// builder assembles it for display only.
func (r *SQLDemoRunner) ObserveSQLSafety(ctx context.Context, attacks []string) (demos.SQLSafetyObservation, error) {
	const lookupSQL = "SELECT count(*) FROM posts WHERE title = $1"

	attempts := make([]demos.SQLSafetyAttempt, 0, len(attacks))
	for _, attack := range attacks {
		var matched int
		if err := r.pool.QueryRow(ctx, lookupSQL, attack).Scan(&matched); err != nil {
			return demos.SQLSafetyObservation{}, fmt.Errorf("run the parameterized lookup: %w", err)
		}
		var total int
		if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM posts").Scan(&total); err != nil {
			return demos.SQLSafetyObservation{}, fmt.Errorf("count posts after the attempt: %w", err)
		}
		attempts = append(attempts, demos.SQLSafetyAttempt{
			Input:       attack,
			SQL:         lookupSQL,
			RowsMatched: matched,
			PostsTotal:  total,
		})
	}
	return demos.SQLSafetyObservation{Attempts: attempts}, nil
}

// RunTxStory executes the begin / insert / savepoint / rollback dance on
// two scratch posts inside ONE transaction, recording the count the
// transaction itself observes at every step, then the count after the
// final ROLLBACK — the database never saw any of it.
func (r *SQLDemoRunner) RunTxStory(ctx context.Context) (demos.TxStoryObservation, error) {
	var steps []demos.TxStoryRow
	count := func(qtx pgx.Tx) int {
		var n int
		if err := qtx.QueryRow(ctx, "SELECT count(*) FROM posts").Scan(&n); err != nil {
			return -1
		}
		return n
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return demos.TxStoryObservation{}, fmt.Errorf("begin the story transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	steps = append(steps, demos.TxStoryRow{Action: "begin", SQL: "BEGIN", Count: count(tx)})
	if _, err := tx.Exec(ctx,
		`INSERT INTO posts (title, body) VALUES ('tx story scratch one', 'rolled back with the unit')`); err != nil {
		return demos.TxStoryObservation{}, fmt.Errorf("insert the first scratch post: %w", err)
	}
	steps = append(steps, demos.TxStoryRow{Action: "insert scratch post",
		SQL: "INSERT INTO posts (title, body) VALUES ('tx story scratch one', …)", Count: count(tx)})
	if _, err := tx.Exec(ctx, "SAVEPOINT sp1"); err != nil {
		return demos.TxStoryObservation{}, fmt.Errorf("savepoint: %w", err)
	}
	steps = append(steps, demos.TxStoryRow{Action: "savepoint sp1", SQL: "SAVEPOINT sp1", Count: count(tx)})
	if _, err := tx.Exec(ctx,
		`INSERT INTO posts (title, body) VALUES ('tx story scratch two', 'undone by the savepoint')`); err != nil {
		return demos.TxStoryObservation{}, fmt.Errorf("insert the second scratch post: %w", err)
	}
	steps = append(steps, demos.TxStoryRow{Action: "insert second post",
		SQL: "INSERT INTO posts (title, body) VALUES ('tx story scratch two', …)", Count: count(tx)})
	if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT sp1"); err != nil {
		return demos.TxStoryObservation{}, fmt.Errorf("rollback to savepoint: %w", err)
	}
	steps = append(steps, demos.TxStoryRow{Action: "rollback to savepoint",
		SQL: "ROLLBACK TO SAVEPOINT sp1", Count: count(tx)})
	if err := tx.Rollback(ctx); err != nil {
		return demos.TxStoryObservation{}, fmt.Errorf("rollback the story: %w", err)
	}

	var after int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM posts").Scan(&after); err != nil {
		return demos.TxStoryObservation{}, fmt.Errorf("count after the rollback: %w", err)
	}
	steps = append(steps, demos.TxStoryRow{Action: "rollback", SQL: "ROLLBACK", Count: after})

	history, err := r.gooseHistory(ctx)
	if err != nil {
		return demos.TxStoryObservation{}, err
	}
	return demos.TxStoryObservation{Steps: steps, History: history}, nil
}

// gooseHistory reads goose's version ledger in apply order.
func (r *SQLDemoRunner) gooseHistory(ctx context.Context) ([]demos.MigrationRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT version_id, is_applied, tstamp::text FROM goose_db_version ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("read goose ledger: %w", err)
	}
	defer rows.Close()
	history := make([]demos.MigrationRow, 0, 4)
	for rows.Next() {
		var row demos.MigrationRow
		if err := rows.Scan(&row.Version, &row.Applied, &row.AppliedAt); err != nil {
			return nil, fmt.Errorf("scan goose ledger row: %w", err)
		}
		history = append(history, row)
	}
	return history, rows.Err()
}

// seededIDs shapes the exported seed literals into a driver value.
func seededIDs() [][16]byte {
	ids := make([][16]byte, len(postsdata.SeedPostIDs))
	for i, literal := range postsdata.SeedPostIDs {
		ids[i], _ = parseSeedUUID(literal)
	}
	return ids
}

// RunSQLLab executes the three exhibits, pinned to the six seeded ids
// ($1) so live-mode demo traffic cannot perturb the rows it reports.
func (r *SQLDemoRunner) RunSQLLab(ctx context.Context) (demos.SQLLabObservation, error) {
	ids := seededIDs()
	observation := demos.SQLLabObservation{}

	rows, err := r.pool.Query(ctx, demos.SQLTagTotals, ids)
	if err != nil {
		return demos.SQLLabObservation{}, fmt.Errorf("run the tag-totals exhibit: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var row demos.TagTotalRow
		if err := rows.Scan(&row.Tag, &row.Posts); err != nil {
			return demos.SQLLabObservation{}, fmt.Errorf("scan tag-totals row: %w", err)
		}
		observation.TagTotals = append(observation.TagTotals, row)
	}
	if err := rows.Err(); err != nil {
		return demos.SQLLabObservation{}, fmt.Errorf("iterate tag totals: %w", err)
	}

	rows2, err := r.pool.Query(ctx, demos.SQLRankByLength, ids)
	if err != nil {
		return demos.SQLLabObservation{}, fmt.Errorf("run the rank exhibit: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var row demos.LengthRankRow
		if err := rows2.Scan(&row.Title, &row.Runes, &row.Rank); err != nil {
			return demos.SQLLabObservation{}, fmt.Errorf("scan rank row: %w", err)
		}
		observation.LengthRanks = append(observation.LengthRanks, row)
	}
	if err := rows2.Err(); err != nil {
		return demos.SQLLabObservation{}, fmt.Errorf("iterate ranks: %w", err)
	}

	rows3, err := r.pool.Query(ctx, demos.SQLPublishGaps, ids)
	if err != nil {
		return demos.SQLLabObservation{}, fmt.Errorf("run the gaps exhibit: %w", err)
	}
	defer rows3.Close()
	for rows3.Next() {
		var row demos.PublishGapRow
		var days *int
		if err := rows3.Scan(&row.Title, &row.Created, &days); err != nil {
			return demos.SQLLabObservation{}, fmt.Errorf("scan gap row: %w", err)
		}
		if days != nil {
			row.Days = *days
			row.HasGap = true
		}
		observation.PublishGaps = append(observation.PublishGaps, row)
	}
	if err := rows3.Err(); err != nil {
		return demos.SQLLabObservation{}, fmt.Errorf("iterate gaps: %w", err)
	}
	return observation, nil
}

// RunSearch executes the fixed full-text query, the EXPLAIN that shows
// the chosen plan, and the catalog lookup of the GIN index's definition.
func (r *SQLDemoRunner) RunSearch(ctx context.Context) (demos.SQLSearchObservation, error) {
	observation := demos.SQLSearchObservation{}

	if err := r.pool.QueryRow(ctx,
		`SELECT websearch_to_tsquery('english', $1)::text`, demos.SearchDemoQuery).Scan(&observation.TsQuery); err != nil {
		return demos.SQLSearchObservation{}, fmt.Errorf("build the tsquery: %w", err)
	}

	rows, err := r.pool.Query(ctx, demos.SearchDemoSQL, demos.SearchDemoQuery)
	if err != nil {
		return demos.SQLSearchObservation{}, fmt.Errorf("run the search: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var row demos.SearchMatchRow
		var rank float32
		if err := rows.Scan(&row.Title, &rank); err != nil {
			return demos.SQLSearchObservation{}, fmt.Errorf("scan match row: %w", err)
		}
		row.Rank = strconv.FormatFloat(float64(rank), 'f', 4, 64)
		observation.Matches = append(observation.Matches, row)
	}
	if err := rows.Err(); err != nil {
		return demos.SQLSearchObservation{}, fmt.Errorf("iterate matches: %w", err)
	}

	planRows, err := r.pool.Query(ctx, "EXPLAIN "+demos.SearchDemoSQL, demos.SearchDemoQuery)
	if err != nil {
		return demos.SQLSearchObservation{}, fmt.Errorf("explain the search: %w", err)
	}
	defer planRows.Close()
	for planRows.Next() {
		var line string
		if err := planRows.Scan(&line); err != nil {
			return demos.SQLSearchObservation{}, fmt.Errorf("scan plan line: %w", err)
		}
		observation.Plan = append(observation.Plan, line)
	}
	if err := planRows.Err(); err != nil {
		return demos.SQLSearchObservation{}, fmt.Errorf("iterate plan: %w", err)
	}

	if err := r.pool.QueryRow(ctx,
		`SELECT pg_get_indexdef('posts_search_idx'::regclass)`).Scan(&observation.IndexDefinition); err != nil {
		return demos.SQLSearchObservation{}, fmt.Errorf("read the index definition: %w", err)
	}
	return observation, nil
}

// RunRace choreographs the deterministic two-transaction dance: the
// writer's uncommitted update is invisible to the reader, its commit
// makes it visible, and the FOR UPDATE exhibit blocks the reader until
// the writer lets go — every handoff signaled by channels or confirmed
// by pg_stat_activity polling, never by sleeping and hoping.
func (r *SQLDemoRunner) RunRace(ctx context.Context) (demos.RaceObservation, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	observation := demos.RaceObservation{}
	if err := r.pool.QueryRow(ctx, "SHOW transaction_isolation").Scan(&observation.Isolation); err != nil {
		return demos.RaceObservation{}, fmt.Errorf("read the isolation level: %w", err)
	}

	writer, err := r.pool.Acquire(ctx)
	if err != nil {
		return demos.RaceObservation{}, fmt.Errorf("acquire the writer connection: %w", err)
	}
	defer writer.Release()
	reader, err := r.pool.Acquire(ctx)
	if err != nil {
		return demos.RaceObservation{}, fmt.Errorf("acquire the reader connection: %w", err)
	}
	defer reader.Release()
	var readerPID int
	if err := reader.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&readerPID); err != nil {
		return demos.RaceObservation{}, fmt.Errorf("identify the reader: %w", err)
	}

	// The scratch row the dance happens on; removed before returning.
	var scratch string
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO posts (title, body) VALUES ('race demo scratch', 'the row two transactions fight over') RETURNING id::text`,
	).Scan(&scratch); err != nil {
		return demos.RaceObservation{}, fmt.Errorf("create the scratch row: %w", err)
	}
	defer func() {
		_, _ = r.pool.Exec(context.WithoutCancel(ctx), `DELETE FROM posts WHERE id::text = $1`, scratch)
	}()
	readTitle := func() string {
		var title string
		if err := reader.QueryRow(ctx, `SELECT title FROM posts WHERE id::text = $1`, scratch).Scan(&title); err != nil {
			return "error: " + err.Error()
		}
		return title
	}
	step := func(phase, actor, action, observed string) {
		observation.Steps = append(observation.Steps, demos.RaceStepRow{Phase: phase, Actor: actor, Action: action, Observed: observed})
	}

	// Phase one: the invisible uncommitted write.
	wtx, err := writer.Begin(ctx)
	if err != nil {
		return demos.RaceObservation{}, fmt.Errorf("begin the writer transaction: %w", err)
	}
	if _, err := wtx.Exec(ctx,
		`UPDATE posts SET title = 'race demo scratch (updated)' WHERE id::text = $1`, scratch); err != nil {
		return demos.RaceObservation{}, fmt.Errorf("the writer's update: %w", err)
	}
	step("uncommitted-write", "T1 (writer)", "BEGIN; UPDATE … (uncommitted)", "title updated inside T1 only")
	step("reader-looks", "T2 (reader)", "SELECT title …", readTitle())
	if err := wtx.Commit(ctx); err != nil {
		return demos.RaceObservation{}, fmt.Errorf("the writer commits: %w", err)
	}
	step("writer-commits", "T1 (writer)", "COMMIT", "the new version is THE committed truth")
	step("reader-looks-again", "T2 (reader)", "SELECT title … (again)", readTitle())

	// Phase two: the row lock. The reader's FOR UPDATE waits while the
	// writer holds it; the wait is CONFIRMED through pg_stat_activity.
	lockTx, err := writer.Begin(ctx)
	if err != nil {
		return demos.RaceObservation{}, fmt.Errorf("begin the lock transaction: %w", err)
	}
	if _, err := lockTx.Exec(ctx, `SELECT id FROM posts WHERE id::text = $1 FOR UPDATE`, scratch); err != nil {
		return demos.RaceObservation{}, fmt.Errorf("the writer takes the row lock: %w", err)
	}
	step("lock-acquired", "T1 (writer)", "BEGIN; SELECT … FOR UPDATE", "row lock held")

	type lockResult struct {
		title string
		err   error
	}
	locked := make(chan lockResult, 1)
	go func() {
		var title string
		err := reader.QueryRow(ctx, `SELECT title FROM posts WHERE id::text = $1 FOR UPDATE`, scratch).Scan(&title)
		locked <- lockResult{title: title, err: err}
	}()

	waiting := false
	for range 2000 {
		var blocked int
		if err := r.pool.QueryRow(ctx,
			`SELECT count(*) FROM pg_stat_activity WHERE pid = $1 AND wait_event_type = 'Lock'`, readerPID,
		).Scan(&blocked); err == nil && blocked == 1 {
			waiting = true
			break
		}
		select {
		case <-ctx.Done():
			return demos.RaceObservation{}, fmt.Errorf("the lock wait was never observed: %w", ctx.Err())
		case <-time.After(5 * time.Millisecond):
		}
	}
	if !waiting {
		return demos.RaceObservation{}, errors.New("the reader never waited for the row lock — choreography broken")
	}
	if err := lockTx.Commit(ctx); err != nil {
		return demos.RaceObservation{}, fmt.Errorf("release the row lock: %w", err)
	}
	result := <-locked
	if result.err != nil {
		return demos.RaceObservation{}, fmt.Errorf("the reader's locked read: %w", result.err)
	}
	step("lock-waited", "T2 (reader)", "SELECT … FOR UPDATE (waits)", "waited for the lock, then read "+strconv.Quote(result.title))

	return observation, nil
}

// RunAuditStory choreographs the audit pattern's two phases on a scratch
// post: a create-plus-two-audit-rows unit that COMMITS together, then an
// update-plus-audit-row unit that ROLLS BACK together — the counts prove
// both. The scratch rows are removed in a final cleanup transaction (a
// demo affordance: production audit trails are append-only).
func (r *SQLDemoRunner) RunAuditStory(ctx context.Context) (demos.AuditObservation, error) {
	var steps []demos.AuditStepRow
	var scratch string
	outside := func() (string, int) {
		var title *string
		var auditRows int
		if err := r.pool.QueryRow(ctx,
			`SELECT (SELECT title FROM posts WHERE id::text = $1),
			        (SELECT count(*) FROM audit_log WHERE post_id::text = $1)`, scratch,
		).Scan(&title, &auditRows); err != nil {
			return "error: " + err.Error(), -1
		}
		if title == nil {
			return "(no post yet)", auditRows
		}
		return *title, auditRows
	}

	// The scratch subject of the whole story, removed before returning.
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO posts (title, body) VALUES ('audit scratch', 'the row whose history must tell the truth') RETURNING id::text`,
	).Scan(&scratch); err != nil {
		return demos.AuditObservation{}, fmt.Errorf("create the scratch post: %w", err)
	}
	// Two statements, two calls: the extended protocol runs one statement
	// per Exec when parameters ride along.
	defer func() {
		cleanup := context.WithoutCancel(ctx)
		_, _ = r.pool.Exec(cleanup, `DELETE FROM audit_log WHERE post_id::text = $1`, scratch)
		_, _ = r.pool.Exec(cleanup, `DELETE FROM posts WHERE id::text = $1`, scratch)
	}()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return demos.AuditObservation{}, fmt.Errorf("begin the commit phase: %w", err)
	}
	defer tx.Rollback(ctx)

	title, auditRows := outside()
	steps = append(steps, demos.AuditStepRow{Phase: "commit-open", Action: "BEGIN", LiveTitle: title, AuditRows: auditRows})

	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_log (post_id, action, actor, summary) VALUES ($1::uuid, 'create', 'db:audit demo', 'created the scratch post')`, scratch); err != nil {
		return demos.AuditObservation{}, fmt.Errorf("audit the create: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE posts SET title = 'audit scratch (edited)' WHERE id::text = $1`, scratch); err != nil {
		return demos.AuditObservation{}, fmt.Errorf("edit inside the transaction: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_log (post_id, action, actor, summary) VALUES ($1::uuid, 'update', 'db:audit demo', 'edited the title inside the same transaction')`, scratch); err != nil {
		return demos.AuditObservation{}, fmt.Errorf("audit the edit: %w", err)
	}
	var insideTitle string
	var insideRows int
	if err := tx.QueryRow(ctx,
		`SELECT (SELECT title FROM posts WHERE id::text = $1),
		        (SELECT count(*) FROM audit_log WHERE post_id::text = $1)`, scratch,
	).Scan(&insideTitle, &insideRows); err != nil {
		return demos.AuditObservation{}, fmt.Errorf("observe inside the transaction: %w", err)
	}
	steps = append(steps, demos.AuditStepRow{Phase: "commit-change",
		Action: "INSERT post; INSERT audit_log; UPDATE title; INSERT audit_log", LiveTitle: insideTitle, AuditRows: insideRows})
	if err := tx.Commit(ctx); err != nil {
		return demos.AuditObservation{}, fmt.Errorf("commit both writes: %w", err)
	}
	title, auditRows = outside()
	steps = append(steps, demos.AuditStepRow{Phase: "commit-done", Action: "COMMIT", LiveTitle: title, AuditRows: auditRows})

	// The rollback phase: the same two-write unit, discarded.
	title, auditRows = outside()
	steps = append(steps, demos.AuditStepRow{Phase: "rollback-open", Action: "BEGIN", LiveTitle: title, AuditRows: auditRows})
	doomed, err := r.pool.Begin(ctx)
	if err != nil {
		return demos.AuditObservation{}, fmt.Errorf("begin the rollback phase: %w", err)
	}
	defer doomed.Rollback(ctx)
	if _, err := doomed.Exec(ctx, `UPDATE posts SET title = 'audit scratch (doomed)' WHERE id::text = $1`, scratch); err != nil {
		return demos.AuditObservation{}, fmt.Errorf("the doomed edit: %w", err)
	}
	if _, err := doomed.Exec(ctx,
		`INSERT INTO audit_log (post_id, action, actor, summary) VALUES ($1::uuid, 'update', 'db:audit demo', 'this history entry must never exist')`, scratch); err != nil {
		return demos.AuditObservation{}, fmt.Errorf("the doomed audit row: %w", err)
	}
	if err := doomed.QueryRow(ctx,
		`SELECT (SELECT title FROM posts WHERE id::text = $1),
		        (SELECT count(*) FROM audit_log WHERE post_id::text = $1)`, scratch,
	).Scan(&insideTitle, &insideRows); err != nil {
		return demos.AuditObservation{}, fmt.Errorf("observe inside the doomed transaction: %w", err)
	}
	steps = append(steps, demos.AuditStepRow{Phase: "rollback-inside",
		Action: "UPDATE title; INSERT audit_log", LiveTitle: insideTitle, AuditRows: insideRows})
	if err := doomed.Rollback(ctx); err != nil {
		return demos.AuditObservation{}, fmt.Errorf("roll both writes back: %w", err)
	}
	title, auditRows = outside()
	steps = append(steps, demos.AuditStepRow{Phase: "rollback-done", Action: "ROLLBACK", LiveTitle: title, AuditRows: auditRows})

	return demos.AuditObservation{Steps: steps}, nil
}

// parseSeedUUID parses a canonical uuid literal into its 16 bytes (the
// postsdata package owns the canonical parser; this local twin keeps the
// runner's dependency surface to the pool and the domain).
func parseSeedUUID(s string) ([16]byte, bool) {
	var key [16]byte
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return key, false
	}
	pos := 0
	for i := 0; i < len(s); i++ {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		hi, ok := hexNibbleOf(s[i])
		if !ok {
			return [16]byte{}, false
		}
		lo, ok := hexNibbleOf(s[i+1])
		if !ok {
			return [16]byte{}, false
		}
		key[pos] = hi<<4 | lo
		pos++
		i++
	}
	return key, true
}

// hexNibbleOf decodes one hex digit.
func hexNibbleOf(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}
