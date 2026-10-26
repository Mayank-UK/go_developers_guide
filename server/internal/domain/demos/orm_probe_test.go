package demos

import (
	"strings"
	"testing"
)

// The orm-probe payload assembly, tested against a stubbed observation
// shaped like the real one the data-layer runner returns: canonical trial
// order, curated labels, ratios measured against the baseline, duration
// bands, capped statement samples, dropped unknown strategies, and the
// wire's never-nil rule.

func stubORMProbeObservation() ORMProbeObservation {
	return ORMProbeObservation{
		Dataset: ORMProbeDatasetFacts{Posts: 12, Tags: 5, Edges: 24},
		Trials: []ORMProbeTrialFacts{
			{
				Strategy:         ORMProbeStrategyPgx,
				Statements:       1,
				Rows:             12,
				DurationMicros:   800,
				SampleStatements: []string{"SELECT p.id, …"},
			},
			{
				Strategy:         ORMProbeStrategySQLC,
				Statements:       1,
				Rows:             12,
				DurationMicros:   900,
				SampleStatements: []string{"-- name: ListPostsWithTags :many SELECT …"},
			},
			{
				Strategy:       ORMProbeStrategyGormLazy,
				Statements:     13,
				Rows:           12,
				DurationMicros: 3200,
				SampleStatements: []string{
					`SELECT * FROM "ormprobe_posts" ORDER BY id`,
					`SELECT * FROM "ormprobe_tags" … WHERE post_id = 1`,
					`SELECT * FROM "ormprobe_tags" … WHERE post_id = 2`,
					`SELECT * FROM "ormprobe_tags" … WHERE post_id = 3`,
				},
			},
			{
				Strategy:         ORMProbeStrategyGormEager,
				Statements:       3,
				Rows:             12,
				DurationMicros:   1200,
				SampleStatements: []string{`SELECT * FROM "ormprobe_post_tags" …`},
			},
		},
		ResultsAgree: true,
		UnitOfWork: ORMProbeUnitOfWorkFacts{
			Repositories: []string{"GormPostsRepository", "GormTagsRepository"},
			Steps: []ORMProbeStepFacts{
				{Action: "begin", Detail: "one unit of work"},
				{Action: "rollback", Detail: "deliberate failure"},
			},
			HookApplied: true,
			PostsBefore: 12,
			PostsAfter:  12,
			RolledBack:  true,
		},
	}
}

func TestBuildORMProbePayloadOrdersTrialsAndAttachesCuration(t *testing.T) {
	payload := BuildORMProbePayload(stubORMProbeObservation())

	wantOrder := []string{
		ORMProbeStrategyPgx, ORMProbeStrategySQLC,
		ORMProbeStrategyGormLazy, ORMProbeStrategyGormEager,
	}
	if len(payload.Trials) != len(wantOrder) {
		t.Fatalf("trials = %d, want %d", len(payload.Trials), len(wantOrder))
	}
	for i, want := range wantOrder {
		if payload.Trials[i].Strategy != want {
			t.Errorf("trial %d = %q, want %q", i, payload.Trials[i].Strategy, want)
		}
		if payload.Trials[i].Label == "" || payload.Trials[i].Library == "" || payload.Trials[i].CountingVia == "" {
			t.Errorf("trial %q lacks its curated label/library/counting seam", want)
		}
	}
}

func TestBuildORMProbePayloadDerivesRatiosAndBands(t *testing.T) {
	payload := BuildORMProbePayload(stubORMProbeObservation())

	want := map[string]struct {
		ratio string
		band  string
	}{
		ORMProbeStrategyPgx:       {"1x", "sub-millisecond"},
		ORMProbeStrategySQLC:      {"1x", "sub-millisecond"},
		ORMProbeStrategyGormLazy:  {"13x", "low-milliseconds"},
		ORMProbeStrategyGormEager: {"3x", "sub-millisecond"},
	}
	for _, trial := range payload.Trials {
		expected := want[trial.Strategy]
		if trial.RoundTripRatio != expected.ratio {
			t.Errorf("%s ratio = %q, want %q", trial.Strategy, trial.RoundTripRatio, expected.ratio)
		}
		if trial.TimingBand != expected.band {
			t.Errorf("%s band = %q, want %q", trial.Strategy, trial.TimingBand, expected.band)
		}
	}
}

func TestBuildORMProbePayloadCapsSamplesAndDropsUnknownStrategies(t *testing.T) {
	observation := stubORMProbeObservation()
	observation.Trials = append(observation.Trials, ORMProbeTrialFacts{
		Strategy:   "mystery-strategy",
		Statements: 99,
	})

	payload := BuildORMProbePayload(observation)

	if len(payload.Trials) != 4 {
		t.Errorf("trials = %d, want 4 — unknown strategies are dropped, not rendered", len(payload.Trials))
	}
	for _, trial := range payload.Trials {
		if len(trial.SampleStatements) > maxSampleStatements {
			t.Errorf("trial %q carries %d samples, want at most %d",
				trial.Strategy, len(trial.SampleStatements), maxSampleStatements)
		}
	}
	lazy := payload.Trials[2]
	if len(lazy.SampleStatements) != maxSampleStatements {
		t.Errorf("lazy trial samples = %d, want exactly the cap %d", len(lazy.SampleStatements), maxSampleStatements)
	}
	if !strings.Contains(lazy.SampleStatements[0], "ormprobe_posts") {
		t.Errorf("lazy samples lost their receipts: %v", lazy.SampleStatements)
	}
}

func TestBuildORMProbePayloadNeverRendersNilSlices(t *testing.T) {
	// The emptiest legal observation: everything absent must still render
	// as [] on the wire, never null.
	payload := BuildORMProbePayload(ORMProbeObservation{})

	if payload.Trials == nil || payload.UnitOfWork.Steps == nil || payload.UnitOfWork.Repositories == nil {
		t.Error("payload slices must be non-nil even when empty")
	}
	if len(payload.Trials) != 0 || len(payload.UnitOfWork.Steps) != 0 {
		t.Errorf("empty observation must produce empty collections, got %+v", payload)
	}
	// A missing baseline makes ratios honestly n/a rather than dividing
	// by zero.
	if ratio := ratioToBaseline(5, 0); ratio != "n/a" {
		t.Errorf("ratio without a baseline = %q, want n/a", ratio)
	}
}
