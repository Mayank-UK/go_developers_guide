package db

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"guide/internal/domain/demos"
)

// These tests run against the REAL PostgreSQL of the dev stack (Protocol
// v2: the long-lived compose containers, no ephemeral churn) — but never
// against its dev database. Every test draws a DEDICATED throwaway
// database: created through an admin connection to the `postgres`
// maintenance database, named with a `gdg_test_` prefix plus pid and
// timestamp, and dropped in t.Cleanup. The schema the migrations claim and
// the catalog's own introspection must agree — these tests assert the
// introspected truth, so a migration that does not do what it teaches
// fails here first.

// defaultAdminDSN points at the compose stack's PostgreSQL through its
// in-network service name, at the `postgres` maintenance database — NOT
// the dev `guide` database. GDG_PG_ADMIN_DSN overrides it (e.g. port 5433
// when running tests from the host).
const defaultAdminDSN = "postgres://postgres:guide@postgres:5432/postgres?sslmode=disable"

// throwawaySuffix identifies every database these tests create.
const throwawayPrefix = "gdg_test_"

// newThrowawayPool creates a fresh database on the live server, connects a
// pool to it, and schedules its destruction. The admin DSN is refused if
// it targets the dev database: creating and dropping tables there would
// corrupt the shared state every other consumer sees.
func newThrowawayPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	adminDSN := os.Getenv("GDG_PG_ADMIN_DSN")
	if adminDSN == "" {
		adminDSN = defaultAdminDSN
	}
	parsed, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatalf("parse admin DSN: %v", err)
	}
	if strings.TrimPrefix(parsed.Path, "/") == "guide" {
		t.Fatalf("admin DSN must not target the dev database (got %q) — tests create their own throwaway databases", parsed.Path)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	admin, err := pgxpool.New(ctx, adminDSN)
	if err != nil {
		t.Fatalf("admin connect: %v (is the dev stack's PostgreSQL up?)", err)
	}
	if err := admin.Ping(ctx); err != nil {
		t.Fatalf("admin ping: %v (is the dev stack's PostgreSQL up?)", err)
	}

	name := fmt.Sprintf("%s%d_%d", throwawayPrefix, os.Getpid(), time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create throwaway database %s: %v", name, err)
	}

	parsed.Path = "/" + name
	pool, err := NewPool(ctx, parsed.String())
	if err != nil {
		t.Fatalf("connect throwaway database %s: %v", name, err)
	}

	t.Cleanup(func() {
		pool.Close()
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer dropCancel()
		// WITH (FORCE): no lingering connection can veto the drop.
		if _, err := admin.Exec(dropCtx, fmt.Sprintf("DROP DATABASE %s WITH (FORCE)", name)); err != nil {
			t.Errorf("drop throwaway database %s: %v", name, err)
		}
		admin.Close()
	})
	return pool
}

func TestMigrationsEmbedTheExpectedFileSet(t *testing.T) {
	t.Parallel()

	files, err := fs.ReadDir(Migrations(), ".")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("embedded migrations = %d files, want 2 (init + audit_log)", len(files))
	}
	want := []string{"00001_init.sql", "00002_audit_log.sql"}
	for i, file := range files {
		if file.Name() != want[i] {
			t.Errorf("migration %d = %q, want %q", i, file.Name(), want[i])
		}
	}
}

func TestNewPoolFailsFastOnUnreachableDatabase(t *testing.T) {
	t.Parallel()

	// Port 1 on localhost: connection refused instantly — the boot path's
	// fail-fast contract, proven without waiting on a real timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := NewPool(ctx, "postgres://postgres:guide@127.0.0.1:1/guide?sslmode=disable"); err == nil {
		t.Fatal("NewPool must fail when PostgreSQL is unreachable — fail-fast at boot is the contract")
	}
}

func TestMigrateAppliesTheFullSetThenIsIdempotent(t *testing.T) {
	t.Parallel()
	pool := newThrowawayPool(t)
	ctx := context.Background()

	first, err := Migrate(ctx, pool)
	if err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if first != 2 {
		t.Fatalf("first Migrate applied %d migrations, want 2", first)
	}

	second, err := Migrate(ctx, pool)
	if err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if second != 0 {
		t.Fatalf("second Migrate applied %d migrations, want 0 — goose versioning must make re-runs no-ops", second)
	}
}

func TestMigratedSchemaMatchesWhatTheTopicTeaches(t *testing.T) {
	t.Parallel()
	pool := newThrowawayPool(t)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	snapshot, err := NewIntrospector(pool).Introspect(ctx)
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}

	// Tables: the four taught tables plus goose's own ledger.
	wantTables := []string{"audit_log", "goose_db_version", "post_tags", "posts", "tags"}
	if strings.Join(snapshot.Tables, ",") != strings.Join(wantTables, ",") {
		t.Errorf("tables = %v, want %v", snapshot.Tables, wantTables)
	}

	// Columns: spot-check the load-bearing typings.
	columns := make(map[string]demos.ColumnRow)
	for _, column := range snapshot.Columns {
		columns[column.Table+"."+column.Name] = column
	}
	postsID := columns["posts.id"]
	if postsID.Type != "uuid" || postsID.Nullable || !postsID.HasDefault || postsID.Default != "gen_random_uuid()" {
		t.Errorf("posts.id = %+v, want NOT NULL uuid DEFAULT gen_random_uuid()", postsID)
	}
	auditPostID := columns["audit_log.post_id"]
	if auditPostID.Type != "uuid" || !auditPostID.Nullable {
		t.Errorf("audit_log.post_id = %+v, want NULLABLE uuid (history outlives its subject)", auditPostID)
	}
	auditAction := columns["audit_log.action"]
	if auditAction.Type != "text" || auditAction.Nullable {
		t.Errorf("audit_log.action = %+v, want NOT NULL text", auditAction)
	}
	postTagsTag := columns["post_tags.tag_name"]
	if postTagsTag.Type != "text" || postTagsTag.Nullable {
		t.Errorf("post_tags.tag_name = %+v, want NOT NULL text", postTagsTag)
	}

	// Constraints: the explicit ON DELETE behaviors, exactly as taught.
	constraints := make(map[string]demos.ConstraintRow)
	for _, constraint := range snapshot.Constraints {
		constraints[constraint.Name] = constraint
	}
	for name, wantFragment := range map[string]string{
		"post_tags_post_id_fkey":  "FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE",
		"post_tags_tag_name_fkey": "FOREIGN KEY (tag_name) REFERENCES tags(name) ON DELETE CASCADE",
		"audit_log_post_id_fkey":  "FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE SET NULL",
		"post_tags_pkey":          "PRIMARY KEY (post_id, tag_name)",
	} {
		got, ok := constraints[name]
		if !ok {
			t.Errorf("constraint %q missing from the live schema", name)
			continue
		}
		if !strings.Contains(got.Definition, wantFragment) {
			t.Errorf("constraint %q definition = %q, want it to contain %q", name, got.Definition, wantFragment)
		}
	}
	checks := 0
	for _, constraint := range snapshot.Constraints {
		if constraint.Kind == "CHECK" {
			checks++
		}
	}
	if checks < 3 {
		t.Errorf("CHECK constraints = %d, want at least 3 (title length, tag name, audit action)", checks)
	}

	// Indexes: the two explicit ones, with their access methods.
	indexes := make(map[string]demos.IndexRow)
	for _, index := range snapshot.Indexes {
		indexes[index.Name] = index
	}
	createdIdx, ok := indexes["posts_created_at_idx"]
	if !ok || createdIdx.Kind != "btree" || createdIdx.Unique || createdIdx.Primary {
		t.Errorf("posts_created_at_idx = %+v, want a non-unique non-primary btree", createdIdx)
	}
	tagIdx, ok := indexes["post_tags_tag_name_idx"]
	if !ok || tagIdx.Kind != "btree" {
		t.Errorf("post_tags_tag_name_idx = %+v, want an explicit btree (FKs are not auto-indexed)", tagIdx)
	}
	auditIdx, ok := indexes["audit_log_post_changed_idx"]
	if !ok || len(auditIdx.Columns) != 2 || auditIdx.Columns[0] != "post_id" || auditIdx.Columns[1] != "changed_at" {
		t.Errorf("audit_log_post_changed_idx = %+v, want (post_id, changed_at)", auditIdx)
	}

	// History: goose's ledger, live — its version-0 row (the baseline
	// insert that creates the table) plus the two applied migrations.
	if len(snapshot.History) != 3 || !snapshot.History[0].Applied || snapshot.History[0].Version != 0 ||
		snapshot.History[1].Version != 1 || snapshot.History[2].Version != 2 {
		t.Errorf("migration history = %+v, want baseline 0 plus versions 1 and 2 applied", snapshot.History)
	}
}

func TestUpdatedAtTriggerStampsWithoutTheApp(t *testing.T) {
	t.Parallel()
	pool := newThrowawayPool(t)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO posts (title, body) VALUES ('trigger proof', 'updated_at belongs to the database') RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("insert post: %v", err)
	}
	// The trigger's clock_timestamp() advances within microseconds, so a
	// deliberate pause makes the comparison unambiguous.
	time.Sleep(10 * time.Millisecond)
	if _, err := pool.Exec(ctx, `UPDATE posts SET title = 'trigger proof 2' WHERE id = $1`, id); err != nil {
		t.Fatalf("update post: %v", err)
	}

	var created, updated time.Time
	if err := pool.QueryRow(ctx, `SELECT created_at, updated_at FROM posts WHERE id = $1`, id).Scan(&created, &updated); err != nil {
		t.Fatalf("read back post: %v", err)
	}
	if !updated.After(created) {
		t.Errorf("updated_at %v must be stamped AFTER created_at %v by the trigger — no app code ran", updated, created)
	}

	// The whole point of ON DELETE SET NULL: history survives its subject.
	var auditID string
	err = pool.QueryRow(ctx,
		`INSERT INTO audit_log (post_id, action, actor, summary) VALUES ($1, 'delete', 'test', 'deleting the subject') RETURNING id`,
		id).Scan(&auditID)
	if err != nil {
		t.Fatalf("insert audit row: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM posts WHERE id = $1`, id); err != nil {
		t.Fatalf("delete post: %v", err)
	}
	var postID *string
	if err := pool.QueryRow(ctx, `SELECT post_id FROM audit_log WHERE id = $1`, auditID).Scan(&postID); err != nil {
		t.Fatalf("read audit row: %v", err)
	}
	if postID != nil {
		t.Errorf("audit row post_id = %v, want NULL after the subject's delete (SET NULL, not CASCADE)", *postID)
	}
}

func TestIntrospectOnAnUnmigratedDatabaseReportsEmptyHistory(t *testing.T) {
	t.Parallel()
	pool := newThrowawayPool(t)

	snapshot, err := NewIntrospector(pool).Introspect(context.Background())
	if err != nil {
		t.Fatalf("Introspect on a fresh database must degrade to empty, got error: %v", err)
	}
	if len(snapshot.Tables) != 0 || len(snapshot.History) != 0 {
		t.Errorf("fresh database snapshot = %+v, want empty tables and history", snapshot)
	}
}
