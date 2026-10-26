package db

import (
	"context"
	"strings"
	"testing"

	postsdata "guide/internal/data/posts"
	"guide/internal/domain/demos"
)

// The SQL demo runner's pins, against a REAL throwaway database —
// migrated and seeded exactly like a fresh boot. These tests are where
// the payloads' claimed determinism is actually earned: the counts, the
// ranked rows, the choreographed observations, and the no-residue
// guarantees (every demo leaves the post count exactly where it found
// it) all run for real here.

// newSeededRunner draws a throwaway database, migrates it, seeds the six
// guide posts, and returns the demo runner over its pool.
func newSeededRunner(t *testing.T) *SQLDemoRunner {
	t.Helper()
	pool := newThrowawayPool(t)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := postsdata.Seed(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return NewSQLDemoRunner(pool)
}

// postCount asserts the database's post count — the no-residue pin every
// demo must restore after itself.
func postCount(t *testing.T, runner *SQLDemoRunner) int {
	t.Helper()
	var count int
	if err := runner.pool.QueryRow(context.Background(), "SELECT count(*) FROM posts").Scan(&count); err != nil {
		t.Fatalf("count posts: %v", err)
	}
	return count
}

func TestDBSQLSafetyRunnerBindsAttacksAsData(t *testing.T) {
	t.Parallel()
	runner := newSeededRunner(t)
	ctx := context.Background()

	observation, err := runner.ObserveSQLSafety(ctx, demos.SQLSafetyAttackInputs())
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if len(observation.Attempts) != 2 {
		t.Fatalf("attempts = %d, want one per attack", len(observation.Attempts))
	}
	for _, attempt := range observation.Attempts {
		if attempt.RowsMatched != 0 {
			t.Errorf("attack %q matched %d rows — it must land as data nobody stored", attempt.Input, attempt.RowsMatched)
		}
		if attempt.PostsTotal != 6 {
			t.Errorf("attack %q left %d posts — the table must be intact", attempt.Input, attempt.PostsTotal)
		}
	}
}

func TestDBTxStoryRunnerDancesTheSavepointChoreography(t *testing.T) {
	t.Parallel()
	runner := newSeededRunner(t)
	ctx := context.Background()

	observation, err := runner.RunTxStory(ctx)
	if err != nil {
		t.Fatalf("run the story: %v", err)
	}
	wantCounts := []int{6, 7, 7, 8, 7, 6}
	if len(observation.Steps) != len(wantCounts) {
		t.Fatalf("steps = %d, want %d", len(observation.Steps), len(wantCounts))
	}
	for i, want := range wantCounts {
		if observation.Steps[i].Count != want {
			t.Errorf("step %q counted %d posts, want %d", observation.Steps[i].Action, observation.Steps[i].Count, want)
		}
	}
	// A fresh throwaway carries the baseline row plus all three migrations.
	if len(observation.History) != 4 || observation.History[0].Version != 0 || observation.History[3].Version != 3 {
		t.Errorf("goose ledger = %+v, want baseline plus versions 1-3", observation.History)
	}
	if got := postCount(t, runner); got != 6 {
		t.Errorf("the story left %d posts — ROLLBACK must leave nothing behind", got)
	}
}

func TestDBSQLLabRunnerServesDeterministicSeedRows(t *testing.T) {
	t.Parallel()
	runner := newSeededRunner(t)
	ctx := context.Background()

	observation, err := runner.RunSQLLab(ctx)
	if err != nil {
		t.Fatalf("run the lab: %v", err)
	}

	wantTags := map[string]int{
		"api": 2, "architecture": 1, "design": 1, "errors": 1,
		"go": 4, "http": 2, "observability": 1, "routing": 1,
	}
	if len(observation.TagTotals) != len(wantTags) {
		t.Fatalf("tag totals = %d rows, want %d: %+v", len(observation.TagTotals), len(wantTags), observation.TagTotals)
	}
	for _, row := range observation.TagTotals {
		if wantTags[row.Tag] != row.Posts {
			t.Errorf("tag %q counts %d posts, want %d", row.Tag, row.Posts, wantTags[row.Tag])
		}
	}

	if len(observation.LengthRanks) != 6 || observation.LengthRanks[0].Rank != 1 {
		t.Fatalf("ranks = %+v, want all six posts with rank 1 first", observation.LengthRanks)
	}
	for i := 1; i < len(observation.LengthRanks); i++ {
		prev, next := observation.LengthRanks[i-1], observation.LengthRanks[i]
		if prev.Runes < next.Runes {
			t.Errorf("bodies must shorten down the ranking: %d before %d", prev.Runes, next.Runes)
		}
		if prev.Runes == next.Runes && next.Rank != prev.Rank {
			t.Errorf("tied bodies must share a rank: %+v and %+v", prev, next)
		}
	}

	wantGaps := []int{2, 3, 2, 3, 3} // Jan 5→7→10→12→15→18
	if len(observation.PublishGaps) != 6 {
		t.Fatalf("gaps = %d rows, want six", len(observation.PublishGaps))
	}
	if observation.PublishGaps[0].HasGap {
		t.Error("the oldest post has no previous row to lag from")
	}
	for i, want := range wantGaps {
		row := observation.PublishGaps[i+1]
		if !row.HasGap || row.Days != want {
			t.Errorf("gap after %q = %d days (has=%v), want %d", row.Title, row.Days, row.HasGap, want)
		}
	}
	if got := postCount(t, runner); got != 6 {
		t.Errorf("the lab must not write; %d posts", got)
	}
}

func TestDBSearchRunnerMatchesRanksAndExplains(t *testing.T) {
	t.Parallel()
	runner := newSeededRunner(t)
	ctx := context.Background()

	observation, err := runner.RunSearch(ctx)
	if err != nil {
		t.Fatalf("run the search: %v", err)
	}
	if observation.TsQuery != "'log' | 'error'" {
		t.Errorf("tsquery = %q, want the stemmed pair", observation.TsQuery)
	}
	// Three seeded posts carry a stem: the slog post ('log'), the errors
	// post ('error' in title AND body), and the pagination post (whose
	// body says "not an error") — ranked heaviest-match first.
	if len(observation.Matches) != 3 {
		t.Fatalf("matches = %+v, want the three stem-carrying posts", observation.Matches)
	}
	titles := observation.Matches[0].Title + "|" + observation.Matches[1].Title + "|" + observation.Matches[2].Title
	for _, want := range []string{"Structured logging with slog", "Errors are values", "Pagination that tells the truth"} {
		if !strings.Contains(titles, want) {
			t.Errorf("matches drifted: %s", titles)
		}
	}
	if len(observation.Plan) == 0 {
		t.Error("the EXPLAIN returned no plan lines")
	}
	if !strings.Contains(observation.IndexDefinition, "USING gin") || !strings.Contains(observation.IndexDefinition, "to_tsvector") {
		t.Errorf("index definition = %q, want the gin expression index", observation.IndexDefinition)
	}
}

func TestDBRaceRunnerObservesTheDeterministicSequence(t *testing.T) {
	t.Parallel()
	runner := newSeededRunner(t)
	ctx := context.Background()

	observation, err := runner.RunRace(ctx)
	if err != nil {
		t.Fatalf("run the race: %v", err)
	}
	if observation.Isolation != "read committed" {
		t.Errorf("isolation = %q, want the database's default", observation.Isolation)
	}
	byPhase := make(map[string]string, len(observation.Steps))
	for _, step := range observation.Steps {
		byPhase[step.Phase] = step.Observed
	}
	if got := byPhase["reader-looks"]; got != "race demo scratch" {
		t.Errorf("the reader saw %q before the commit — under read committed it must see the old row", got)
	}
	if got := byPhase["reader-looks-again"]; got != "race demo scratch (updated)" {
		t.Errorf("the reader saw %q after the commit — the new version must be visible", got)
	}
	if !strings.Contains(byPhase["lock-waited"], "waited for the lock") {
		t.Errorf("the lock exhibit = %q, must confirm the wait", byPhase["lock-waited"])
	}
	if got := postCount(t, runner); got != 6 {
		t.Errorf("the race left %d posts — the scratch row must be cleaned up", got)
	}
}

func TestDBAuditRunnerProvesBothVerdictsOnTheRealDatabase(t *testing.T) {
	t.Parallel()
	runner := newSeededRunner(t)
	ctx := context.Background()

	observation, err := runner.RunAuditStory(ctx)
	if err != nil {
		t.Fatalf("run the audit story: %v", err)
	}
	payload := demos.BuildAuditPayload(observation)
	if !payload.CommittedTogether {
		t.Errorf("the commit phase must leave the change AND its history visible: %+v", observation.Steps)
	}
	if !payload.RolledBackTogether {
		t.Errorf("the rollback phase must take the change AND its audit row away: %+v", observation.Steps)
	}
	var auditLeft, postsLeft int
	if err := runner.pool.QueryRow(ctx,
		`SELECT (SELECT count(*) FROM audit_log), (SELECT count(*) FROM posts)`).Scan(&auditLeft, &postsLeft); err != nil {
		t.Fatalf("count leftovers: %v", err)
	}
	if auditLeft != 0 || postsLeft != 6 {
		t.Errorf("leftovers: %d audit rows, %d posts — the demo must clean up after itself", auditLeft, postsLeft)
	}
}
