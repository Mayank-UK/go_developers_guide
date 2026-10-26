package ormprobe

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"guide/internal/domain/demos"
)

// The topic's testcontainers showcase — and the first real consumer of
// the Phase 6 docker.sock wiring after the smoke test. A test inside the
// dev container drives the HOST Docker daemon, spawns a throwaway
// postgres:17 sibling, and runs the ENTIRE probe against it: schema
// creation, idempotent seeding, the four measured trials, and the
// unit-of-work exhibit.
//
// DERIVATION OF THE PINNED NUMBERS (never guessed — measured):
//   - the seed inserts 12 posts and 5 tags, 2 edges per post → 24 edges;
//   - pgx baseline: ONE explicit statement → Statements == 1;
//   - sqlc: the measured phase contains exactly one generated call site
//     → Statements == 1;
//   - GORM lazy: Find(page) + one Association("Tags") query per post =
//     1 + 12 = 13 statements — the N+1, counted by the session logger;
//   - GORM eager: Preload on a many2many issues the page query plus TWO
//     relation queries — the join table's post_id IN (…) lookup and the
//     tags' name IN (…) lookup (the samples show all three) — so 3
//     statements, flat regardless of page size. A belongs-to preload
//     would cost 2; the association's SHAPE decides;
//   - unit of work: two repositories in one deliberately-failed
//     transaction → PostsBefore == PostsAfter == 12, RolledBack,
//     HookApplied (the read-back inside the unit saw the trimmed title).
//
// Every number below was first observed by RUNNING this test against the
// real container, then pinned.
const (
	// dockerSocket is where compose mounts the host daemon into the dev
	// container; without it the showcase skips, never fails.
	dockerSocket = "/var/run/docker.sock"
	// containerStartup bounds image create + PostgreSQL's own boot; the
	// image is already on the host (it IS the stack's engine).
	containerStartup = 90 * time.Second
)

// startProbePostgres spawns the throwaway postgres:17 the probe runs
// against, mirrors testcontainers_smoke_test.go's proven template, and
// returns its DSN.
func startProbePostgres(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(dockerSocket); err != nil {
		t.Skipf("docker socket unavailable at %s: testcontainers wiring not present on this machine", dockerSocket)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Under `go test ./...`, several test binaries may start their Ryuk
	// reaper containers at the same instant, and the daemon occasionally
	// leaves a reaper in "created" — a known testcontainers startup race,
	// not a probe problem. One bounded retry absorbs exactly that.
	start := func() (testcontainers.Container, error) {
		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "postgres:17",
				ExposedPorts: []string{"5432/tcp"},
				Env: map[string]string{
					"POSTGRES_USER":     "postgres",
					"POSTGRES_PASSWORD": "guide",
					"POSTGRES_DB":       "guide",
				},
				// postgres logs "ready to accept connections" twice — only the
				// second one answers queries.
				WaitingFor: wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(containerStartup),
			},
			Started: true,
		})
		if err != nil && strings.Contains(err.Error(), "reaper") {
			t.Logf("reaper startup race (%v) — retrying the container start once", err)
			return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					Image:        "postgres:17",
					ExposedPorts: []string{"5432/tcp"},
					Env: map[string]string{
						"POSTGRES_USER":     "postgres",
						"POSTGRES_PASSWORD": "guide",
						"POSTGRES_DB":       "guide",
					},
					WaitingFor: wait.ForLog("database system is ready to accept connections").
						WithOccurrence(2).
						WithStartupTimeout(containerStartup),
				},
				Started: true,
			})
		}
		return container, err
	}
	container, err := start()
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		terminateCtx, terminateCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer terminateCancel()
		if err := container.Terminate(terminateCtx); err != nil {
			t.Logf("terminate container: %v (Ryuk reaps it regardless)", err)
		}
	})

	// container.Host honors TESTCONTAINERS_HOST_OVERRIDE: the sibling is
	// reachable through its published port on the Docker Desktop host
	// alias, not through the compose bridge gateway.
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("resolve container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("resolve mapped port: %v", err)
	}
	return fmt.Sprintf("postgres://postgres:guide@%s:%s/guide?sslmode=disable", host, port.Port())
}

// trialByStrategy indexes the observation's trials for pinning.
func trialByStrategy(t *testing.T, observation demos.ORMProbeObservation) map[string]demos.ORMProbeTrialFacts {
	t.Helper()
	trials := make(map[string]demos.ORMProbeTrialFacts, len(observation.Trials))
	for _, trial := range observation.Trials {
		trials[trial.Strategy] = trial
	}
	return trials
}

func TestDBORMProbeMeasuresAgainstRealPostgreSQL(t *testing.T) {
	runner := NewRunner(startProbePostgres(t))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	observation, err := runner.Probe(ctx)
	if err != nil {
		t.Fatalf("probe must run against a fresh real PostgreSQL: %v", err)
	}

	if got := observation.Dataset; got.Posts != 12 || got.Tags != 5 || got.Edges != 24 {
		t.Errorf("dataset = %d posts / %d tags / %d edges, want 12 / 5 / 24", got.Posts, got.Tags, got.Edges)
	}

	trials := trialByStrategy(t, observation)
	pinned := map[string]int{
		demos.ORMProbeStrategyPgx:       1,  // one explicit JOIN
		demos.ORMProbeStrategySQLC:      1,  // one generated call site
		demos.ORMProbeStrategyGormLazy:  13, // 1 + 12: the N+1
		demos.ORMProbeStrategyGormEager: 3,  // page + join-table IN + tags IN
	}
	for strategy, want := range pinned {
		trial, ok := trials[strategy]
		if !ok {
			t.Errorf("trial %q missing from the observation", strategy)
			continue
		}
		if trial.Statements != want {
			t.Errorf("%s statements = %d, want %d (samples: %v)",
				strategy, trial.Statements, want, trial.SampleStatements)
		}
		if trial.Rows != 12 {
			t.Errorf("%s rows = %d, want 12", strategy, trial.Rows)
		}
	}

	if !observation.ResultsAgree {
		t.Error("all four trials must answer the same rows — the comparison is apples-to-apples")
	}

	unit := observation.UnitOfWork
	if !unit.RolledBack || unit.PostsBefore != 12 || unit.PostsAfter != 12 {
		t.Errorf("unit of work = rolled back %t, posts %d→%d, want true, 12→12",
			unit.RolledBack, unit.PostsBefore, unit.PostsAfter)
	}
	if !unit.HookApplied {
		t.Error("the BeforeCreate hook's effect must be observable inside the unit")
	}
	if len(unit.Repositories) != 2 || unit.Repositories[0] != "GormPostsRepository" {
		t.Errorf("unit repositories = %v, want the two GORM repositories", unit.Repositories)
	}
}

func TestDBORMProbeIsIdempotentAcrossRuns(t *testing.T) {
	runner := NewRunner(startProbePostgres(t))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	first, err := runner.Probe(ctx)
	if err != nil {
		t.Fatalf("first probe run: %v", err)
	}
	second, err := runner.Probe(ctx)
	if err != nil {
		t.Fatalf("second probe run: %v", err)
	}

	if first.Dataset != second.Dataset {
		t.Errorf("dataset drifted between runs: %+v then %+v — the seed is not idempotent", first.Dataset, second.Dataset)
	}
	firstTrials, secondTrials := trialByStrategy(t, first), trialByStrategy(t, second)
	for _, strategy := range []string{
		demos.ORMProbeStrategyPgx, demos.ORMProbeStrategySQLC,
		demos.ORMProbeStrategyGormLazy, demos.ORMProbeStrategyGormEager,
	} {
		if firstTrials[strategy].Statements != secondTrials[strategy].Statements {
			t.Errorf("%s statements drifted: %d then %d — residue or a broken measurement window",
				strategy, firstTrials[strategy].Statements, secondTrials[strategy].Statements)
		}
	}
	if second.UnitOfWork.PostsBefore != 12 || second.UnitOfWork.PostsAfter != 12 {
		t.Errorf("unit of work left residue: posts %d→%d on the second run",
			second.UnitOfWork.PostsBefore, second.UnitOfWork.PostsAfter)
	}
}

func TestDBORMProbeRunnerDegradesOnBadDSN(t *testing.T) {
	// Port 1 refuses connections immediately: the probe must return an
	// error (which the handler maps to the typed service_unavailable),
	// never hang, never panic.
	runner := NewRunner("postgres://postgres:guide@127.0.0.1:1/guide?sslmode=disable")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := runner.Probe(ctx); err == nil {
		t.Fatal("a probe against an unreachable database must fail honestly")
	}
}
