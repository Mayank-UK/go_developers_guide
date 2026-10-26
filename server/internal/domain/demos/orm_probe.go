// Topic 18 sqlc-gorm's live demo: the ORM probe. The demo answers "what
// does each data-access style actually COST?" — not as opinion but as a
// measured exhibit: one seeded posts+tags dataset, one logical job (the
// posts page with its tags), run three ways (raw pgx, sqlc, GORM), every
// SQL statement counted, plus a two-repository unit-of-work exhibit that
// proves a transaction spans repos atomically. The measurement lives in
// internal/data/ormprobe; this file owns the port's types and the pure
// payload builder — data → domain, never the reverse.
package demos

import (
	"context"
	"fmt"
)

// ORMProbeRunner is the port the orm-probe demo reads through: one call
// runs the whole measured exhibit against a live PostgreSQL and returns
// the raw facts. The pgx/sqlc/GORM implementation lives in
// internal/data/ormprobe; tests and the handler wire fakes through this
// interface.
type ORMProbeRunner interface {
	// Probe creates and seeds the ormprobe_* tables if needed (idempotent),
	// then runs the measured trials and the unit-of-work exhibit.
	Probe(ctx context.Context) (ORMProbeObservation, error)
}

// Strategy keys — the fixed identities the payload orders trials by and
// the client's DTO validates against.
const (
	// ORMProbeStrategyPgx is the raw-pgx baseline: one explicit JOIN.
	ORMProbeStrategyPgx = "pgx-baseline"
	// ORMProbeStrategySQLC is the sqlc trial: the same JOIN, generated code.
	ORMProbeStrategySQLC = "sqlc-join"
	// ORMProbeStrategyGormLazy is GORM resolving each post's tags per row —
	// the N+1 exhibit.
	ORMProbeStrategyGormLazy = "gorm-lazy"
	// ORMProbeStrategyGormEager is GORM's Preload — the structural fix.
	ORMProbeStrategyGormEager = "gorm-eager"
)

// ormProbeTrialOrder is the payload's fixed trial order: baseline first,
// the fix last, so the statement-count column tells the story on its own.
var ormProbeTrialOrder = []string{
	ORMProbeStrategyPgx,
	ORMProbeStrategySQLC,
	ORMProbeStrategyGormLazy,
	ORMProbeStrategyGormEager,
}

// ORMProbeDatasetFacts is the seeded world the probe measured against.
type ORMProbeDatasetFacts struct {
	// Posts is the number of seeded posts (the N in N+1).
	Posts int
	// Tags is the size of the seeded tag vocabulary.
	Tags int
	// Edges is the number of post↔tag edges (posts × tags per post).
	Edges int
}

// ORMProbeTrialFacts is one measured trial: what the strategy did, how
// many SQL statements it took, what it answered, and how long that run
// took (a live observation — never a promise).
type ORMProbeTrialFacts struct {
	// Strategy is the fixed trial identity (the constants above).
	Strategy string
	// Statements is the measured SQL statement count — the durable fact.
	Statements int
	// Rows is how many posts the trial answered.
	Rows int
	// DurationMicros is THIS run's wall time in microseconds.
	DurationMicros int64
	// SampleStatements is the actual SQL the strategy issued (capped by the
	// builder for display; the count above is the whole truth).
	SampleStatements []string
}

// ORMProbeStepFacts is one observable step of the unit-of-work exhibit.
type ORMProbeStepFacts struct {
	Action string
	Detail string
}

// ORMProbeUnitOfWorkFacts is the transactions-across-repositories exhibit:
// two repositories over one transaction, a hook observed firing, then a
// deliberate failure — and the table provably unchanged afterwards.
type ORMProbeUnitOfWorkFacts struct {
	// Repositories names the two repository types that shared the tx.
	Repositories []string
	// Steps narrates what happened inside and after the transaction.
	Steps []ORMProbeStepFacts
	// HookApplied reports the BeforeCreate hook's observable effect.
	HookApplied bool
	// PostsBefore and PostsAfter bracket the rolled-back unit: equal means
	// nothing leaked.
	PostsBefore int
	PostsAfter  int
	// RolledBack reports the deliberate failure unwound every write.
	RolledBack bool
}

// ORMProbeObservation is the runner's whole haul, uncurated.
type ORMProbeObservation struct {
	Dataset      ORMProbeDatasetFacts
	Trials       []ORMProbeTrialFacts
	ResultsAgree bool
	UnitOfWork   ORMProbeUnitOfWorkFacts
}

// ORMProbePayload is the wire shape of GET /api/demo/db/orm-probe
// (camelCase JSON). Deterministic in structure and in every count; the
// duration fields are the live observation of the run that served this
// response. Every slice non-nil so the wire renders [] — never null.
type ORMProbePayload struct {
	Dataset      ORMProbeDataset    `json:"dataset"`
	Trials       []ORMProbeTrial    `json:"trials"`
	ResultsAgree bool               `json:"resultsAgree"`
	UnitOfWork   ORMProbeUnitOfWork `json:"unitOfWork"`
}

// ORMProbeDataset is the seeded world as the demo serves it.
type ORMProbeDataset struct {
	Posts int `json:"posts"`
	Tags  int `json:"tags"`
	Edges int `json:"edges"`
}

// ORMProbeTrial is one measured strategy as the demo serves it.
type ORMProbeTrial struct {
	// Strategy is the fixed identity the client keys rendering on.
	Strategy string `json:"strategy"`
	// Label is the human name.
	Label string `json:"label"`
	// Library names the data-access library behind the trial.
	Library string `json:"library"`
	// CountingVia names the observability seam the count came from.
	CountingVia string `json:"countingVia"`
	// Statements is the measured SQL statement count.
	Statements int `json:"statements"`
	// RoundTripRatio is statements relative to the baseline ("1x", "13x").
	RoundTripRatio string `json:"roundTripRatio"`
	// Rows is the posts answered.
	Rows int `json:"rows"`
	// DurationMicros is this run's wall time (live, not a promise).
	DurationMicros int64 `json:"durationMicros"`
	// TimingBand is the honest coarse range the duration falls in.
	TimingBand string `json:"timingBand"`
	// SampleStatements is up to three of the actual statements issued.
	SampleStatements []string `json:"sampleStatements"`
}

// ORMProbeStep is one unit-of-work step as the demo serves it.
type ORMProbeStep struct {
	Action string `json:"action"`
	Detail string `json:"detail"`
}

// ORMProbeUnitOfWork is the unit-of-work exhibit as the demo serves it.
type ORMProbeUnitOfWork struct {
	Repositories []string       `json:"repositories"`
	Steps        []ORMProbeStep `json:"steps"`
	HookApplied  bool           `json:"hookApplied"`
	PostsBefore  int            `json:"postsBefore"`
	PostsAfter   int            `json:"postsAfter"`
	RolledBack   bool           `json:"rolledBack"`
}

// ormProbeTrialMeta is the curated display layer: the fixed labels and
// counting-seam names per strategy — the payload's explanation of itself.
var ormProbeTrialMeta = map[string]struct{ Label, Library, CountingVia string }{
	ORMProbeStrategyPgx: {
		Label:       "Raw pgx: one JOIN, tags aggregated in the same round trip",
		Library:     "pgx v5",
		CountingVia: "pgx pool query tracer",
	},
	ORMProbeStrategySQLC: {
		Label:       "sqlc: the same JOIN as generated, typed code",
		Library:     "sqlc v1.31 + pgx v5",
		CountingVia: "explicit generated call sites",
	},
	ORMProbeStrategyGormLazy: {
		Label:       "GORM, the naive loop: each post's tags fetched per row",
		Library:     "GORM v1.31",
		CountingVia: "gorm session logger",
	},
	ORMProbeStrategyGormEager: {
		Label:       "GORM with Preload: the page, then one IN query for all tags",
		Library:     "GORM v1.31",
		CountingVia: "gorm session logger",
	},
}

// maxSampleStatements caps the per-trial SQL receipts the payload carries.
const maxSampleStatements = 3

// BuildORMProbePayload shapes a raw observation into the demo payload:
// trials in the fixed teaching order with their curated labels, ratios
// measured against the pgx baseline, durations bucketed into honest
// ranges, statement samples capped, and every slice materialized non-nil.
// Unknown strategy keys are dropped — the payload never renders a trial
// it cannot explain.
func BuildORMProbePayload(observation ORMProbeObservation) ORMProbePayload {
	byStrategy := make(map[string]ORMProbeTrialFacts, len(observation.Trials))
	var baseline int
	for _, trial := range observation.Trials {
		byStrategy[trial.Strategy] = trial
		if trial.Strategy == ORMProbeStrategyPgx {
			baseline = trial.Statements
		}
	}

	trials := make([]ORMProbeTrial, 0, len(observation.Trials))
	for _, strategy := range ormProbeTrialOrder {
		facts, ok := byStrategy[strategy]
		if !ok {
			continue
		}
		meta := ormProbeTrialMeta[strategy]
		trials = append(trials, ORMProbeTrial{
			Strategy:         strategy,
			Label:            meta.Label,
			Library:          meta.Library,
			CountingVia:      meta.CountingVia,
			Statements:       facts.Statements,
			RoundTripRatio:   ratioToBaseline(facts.Statements, baseline),
			Rows:             facts.Rows,
			DurationMicros:   facts.DurationMicros,
			TimingBand:       timingBand(facts.DurationMicros),
			SampleStatements: cappedSamples(facts.SampleStatements),
		})
	}

	steps := make([]ORMProbeStep, 0, len(observation.UnitOfWork.Steps))
	for _, step := range observation.UnitOfWork.Steps {
		steps = append(steps, ORMProbeStep{Action: step.Action, Detail: step.Detail})
	}

	return ORMProbePayload{
		Dataset: ORMProbeDataset{
			Posts: observation.Dataset.Posts,
			Tags:  observation.Dataset.Tags,
			Edges: observation.Dataset.Edges,
		},
		Trials:       trials,
		ResultsAgree: observation.ResultsAgree,
		UnitOfWork: ORMProbeUnitOfWork{
			Repositories: nonNilStrings(observation.UnitOfWork.Repositories),
			Steps:        steps,
			HookApplied:  observation.UnitOfWork.HookApplied,
			PostsBefore:  observation.UnitOfWork.PostsBefore,
			PostsAfter:   observation.UnitOfWork.PostsAfter,
			RolledBack:   observation.UnitOfWork.RolledBack,
		},
	}
}

// ratioToBaseline renders the trial's statement count relative to the
// baseline's — the payload's durable comparison, immune to clock noise.
func ratioToBaseline(statements, baseline int) string {
	if baseline <= 0 {
		return "n/a"
	}
	return fmt.Sprintf("%dx", statements/baseline)
}

// timingBand buckets a live duration into the honest range language the
// demo promises: counts are facts, timings are ranges.
func timingBand(durationMicros int64) string {
	switch {
	case durationMicros < 1500:
		return "sub-millisecond"
	case durationMicros < 10000:
		return "low-milliseconds"
	default:
		return "milliseconds"
	}
}

// cappedSamples keeps at most maxSampleStatements receipts per trial; the
// trial's Statements count already tells the whole story.
func cappedSamples(samples []string) []string {
	capped := make([]string, 0, maxSampleStatements)
	for _, sample := range samples {
		if len(capped) == maxSampleStatements {
			break
		}
		capped = append(capped, sample)
	}
	return capped
}
