package demos

import (
	"encoding/json"
	"testing"
	"testing/fstest"
)

// The best-practices checklist builder: pure shaping from raw facts —
// verdicts derived only from what the probes recorded, the tools
// exited, and the files contained; honest fails (never invented passes)
// when facts are absent; manual rows claiming nothing.

// bestCleanFacts is the happy-path audit: every probe completed, every
// tool ran clean, every file read carried its markers.
func bestCleanFacts() BestChecklistFacts {
	return BestChecklistFacts{
		Probes: map[string]BestProbeRecording{
			BestProbeHealth: {
				Label: BestProbeHealth, Status: 200, HeaderRequestID: "best-checklist-probe",
				Body: `{"status":"ok","env":"dev"}`,
			},
			BestProbeScenarioGet: {
				Label: BestProbeScenarioGet, Status: 200,
				Body: `{"dataMode":"fake","injectFailure":"none"}`,
			},
			BestProbeScenarioPatchBad: {
				Label: BestProbeScenarioPatchBad, Status: 422,
				Body: `{"error":{"code":"validation_failed","message":"Invalid scenario patch.","requestId":"best-checklist-probe","details":{"dataMode":{"dataMode":"bogus","expected":["fake","live"]}}}}`,
			},
			BestProbePostsNotFound: {
				Label: BestProbePostsNotFound, Status: 404, HeaderRequestID: "best-checklist-probe",
				Body: `{"error":{"code":"not_found","message":"No post with id best-checklist-nope.","requestId":"best-checklist-probe"}}`,
			},
			BestProbePostsInvalidPost: {
				Label: BestProbePostsInvalidPost, Status: 422,
				Body: `{"error":{"code":"validation_failed","message":"Invalid post body.","requestId":"best-checklist-probe"}}`,
			},
		},
		Gofmt:   BestExecOutcome{Label: "gofmt", Command: "gofmt -l .", Ran: true, ExitCode: 0},
		Ratchet: BestExecOutcome{Label: "go-test", Command: "go test -count=1 -run ratchet ./internal/content", Ran: true, ExitCode: 0, Output: "ok  guide/internal/content 1s"},
		Race:    BestExecOutcome{Label: "go-test-race", Command: "go test -race -count=1 -run race ./internal/domain/demos", Ran: true, ExitCode: 0, Output: "ok  guide/internal/domain/demos 2s"},
		GoMod: BestFileRead{RelPath: "server/go.mod", Content: `module guide

go 1.27.3

require (
	github.com/jackc/pgx/v5 v5.11.0
	github.com/redis/go-redis/v9 v9.23.0
	golang.org/x/time v0.16.0 // indirect
)

tool (
	github.com/99designs/gqlgen
	github.com/sqlc-dev/sqlc/cmd/sqlc
)
`},
		MainGo: BestFileRead{RelPath: "server/cmd/guide/main.go", Content: `
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	grpcServer.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	if err := srv.Shutdown(shutdownCtx); err != nil {
`},
		PasswordGo:  BestFileRead{RelPath: "server/internal/domain/auth/password.go", Content: "func (h *Hasher) VerifyUniform(password, encoded string) bool {\n\tif encoded == \"\" {\n\t\tencoded = DecoyHash\n}\nvar DecoyHash = func() string {"},
		AuthStories: BestFileRead{RelPath: "server/internal/data/auth/stories_tokens.go", Content: "// The uniform decoy burn: same work as a real wrong password.\n\thasher.VerifyUniform(password, \"\")"},
		Compose: BestFileRead{RelPath: ".docker/docker-compose.yml", Content: `
services:
  postgres:
    healthcheck:
      test: ["CMD-SHELL", "pg_isready"]
  redis:
    healthcheck:
      test: ["CMD-SHELL", "redis-cli ping"]
  mongo:
    healthcheck:
      test: ["CMD-SHELL", "mongosh --eval"]
  gdg-go:
    name: gdg-go
    external: true
  gdg-gocache:
    name: gdg-gocache
    external: true
`},
		MigrationsFS: fstest.MapFS{
			"00001_init.sql":      {Data: []byte("-- +goose Up\nCREATE TABLE posts ();")},
			"00002_audit_log.sql": {Data: []byte("-- +goose Up\nCREATE TABLE audit_log ();")},
			"00003_fts_index.sql": {Data: []byte("-- +goose Up\nCREATE INDEX ON posts USING gin();")},
		},
		Config: BestConfigFacts{Port: 8100, GRPCAddr: ":8102", Env: "dev"},
	}
}

func bestRowsByID(panel BestChecklistPanel) map[string]BestChecklistItemRow {
	rows := make(map[string]BestChecklistItemRow)
	for _, area := range panel.Areas {
		for _, item := range area.Items {
			rows[item.ID] = item
		}
	}
	return rows
}

func TestBestChecklistPanelPassesWhenFactsAreClean(t *testing.T) {
	panel := BuildBestChecklistPanel(bestCleanFacts())

	if panel.Summary.Items != 15 || panel.Summary.Passed != 13 || panel.Summary.Failed != 0 || panel.Summary.Manual != 2 {
		t.Errorf("summary = %+v, want 15 items / 13 passed / 0 failed / 2 manual", panel.Summary)
	}
	rows := bestRowsByID(panel)
	for id, wantKind := range map[string]string{
		"gofmt-clean":            BestCheckKindExec,
		"module-pins":            BestCheckKindFile,
		"error-envelope-on-miss": BestCheckKindLive,
		"boundary-validation":    BestCheckKindLive,
		"scenario-contract":      BestCheckKindLive,
		"uniform-login-cost":     BestCheckKindFile,
		"migrations-own-schema":  BestCheckKindFile,
		"graceful-drain-wired":   BestCheckKindFile,
		"race-stance":            BestCheckKindExec,
		"health-contract":        BestCheckKindLive,
		"one-config-source":      BestCheckKindLive,
		"stack-as-files":         BestCheckKindFile,
		"ratchet-green":          BestCheckKindExec,
	} {
		row, ok := rows[id]
		if !ok {
			t.Errorf("row %q missing from the panel", id)
			continue
		}
		if row.CheckKind != wantKind {
			t.Errorf("row %q checkKind = %q, want %q", id, row.CheckKind, wantKind)
		}
		if row.Verdict != BestVerdictPass {
			t.Errorf("row %q verdict = %q with evidence %v, want pass", id, row.Verdict, row.Evidence)
		}
	}
	for _, id := range []string{"review-your-diffs", "prod-secrets-real"} {
		row, ok := rows[id]
		if !ok || row.Verdict != BestVerdictManual || row.CheckKind != BestCheckKindManual || row.WhyManual == "" {
			t.Errorf("manual row %q = %+v, want manual with a why", id, rows[id])
		}
	}
	if len(panel.NotDones) != 4 {
		t.Errorf("not-dos = %d, want 4", len(panel.NotDones))
	}
}

// TestBestChecklistEveryMachineRowCarriesEvidence is the family honesty
// pin: no verdict row may claim anything without the evidence that
// produced it, and no manual row may carry a machine verdict.
func TestBestChecklistEveryMachineRowCarriesEvidence(t *testing.T) {
	for name, facts := range map[string]BestChecklistFacts{
		"clean": bestCleanFacts(),
		"empty": {}, // nothing ran, nothing was read
	} {
		panel := BuildBestChecklistPanel(facts)
		for _, area := range panel.Areas {
			for _, item := range area.Items {
				switch item.CheckKind {
				case BestCheckKindManual:
					if item.Verdict != BestVerdictManual || item.WhyManual == "" || len(item.Evidence) != 0 {
						t.Errorf("%s manual row %q = %+v, want manual + why + no evidence", name, item.ID, item)
					}
				default:
					if item.Verdict != BestVerdictPass && item.Verdict != BestVerdictFail {
						t.Errorf("%s row %q verdict = %q, want pass or fail", name, item.ID, item.Verdict)
					}
					if len(item.Evidence) == 0 {
						t.Errorf("%s row %q carries a verdict with no evidence — never allowed", name, item.ID)
					}
				}
			}
		}
	}
}

func TestBestChecklistHonestWhenEverythingBreaks(t *testing.T) {
	panel := BuildBestChecklistPanel(BestChecklistFacts{})

	if panel.Summary.Failed != 12 {
		t.Errorf("failed = %d, want 12 (every machine check that could not run)", panel.Summary.Failed)
	}
	if panel.Summary.Manual != 3 {
		t.Errorf("manual = %d, want 3 (two by design + the compose row degraded honestly)", panel.Summary.Manual)
	}
	rows := bestRowsByID(panel)
	stack := rows["stack-as-files"]
	if stack.Verdict != BestVerdictManual || stack.WhyManual == "" {
		t.Errorf("stack-as-files = %+v, want the honest manual degrade", stack)
	}
	gofmt := rows["gofmt-clean"]
	if gofmt.Verdict != BestVerdictFail || len(gofmt.Evidence) == 0 {
		t.Errorf("gofmt-clean = %+v, want fail with its reason carried", gofmt)
	}
	health := rows["health-contract"]
	if health.Verdict != BestVerdictFail {
		t.Errorf("health-contract = %+v, want fail when the probe never ran", health)
	}
}

func TestBestChecklistParseGoModPins(t *testing.T) {
	parsed := ParseGoModPins(bestCleanFacts().GoMod.Content)
	if parsed.GoDirective != "1.27.3" {
		t.Errorf("go directive = %q, want 1.27.3", parsed.GoDirective)
	}
	if len(parsed.DirectRequires) != 2 {
		t.Errorf("direct requires = %v, want the two non-indirect pins", parsed.DirectRequires)
	}
	if len(parsed.ToolDirectives) != 2 {
		t.Errorf("tool directives = %v, want two", parsed.ToolDirectives)
	}
	if parsed.DirectRequires[0] != "github.com/jackc/pgx/v5 v5.11.0" {
		t.Errorf("first require = %q", parsed.DirectRequires[0])
	}
	if !goModPinPattern.MatchString(parsed.DirectRequires[0]) {
		t.Errorf("pin pattern rejected %q", parsed.DirectRequires[0])
	}
	if goModPinPattern.MatchString("github.com/x/y") {
		t.Error("pin pattern accepted a versionless require")
	}
}

func TestBestChecklistMigrationCounterWalksForReal(t *testing.T) {
	counted := CountGooseMigrations(bestCleanFacts().MigrationsFS)
	if len(counted.Names) != 3 || !counted.AllMarked {
		t.Fatalf("counted = %+v, want 3 marked migrations", counted)
	}
	if !migrationNumbersAreSequential(counted.Names) {
		t.Errorf("names %v reported non-sequential", counted.Names)
	}

	unmarked := fstest.MapFS{"00001_init.sql": {Data: []byte("CREATE TABLE posts ();")}}
	if broken := CountGooseMigrations(unmarked); broken.AllMarked {
		t.Error("a file without the goose Up marker must not count as marked")
	}
	gapped := fstest.MapFS{
		"00001_init.sql": {Data: []byte(gooseUpMarker)},
		"00003_gap.sql":  {Data: []byte(gooseUpMarker)},
	}
	if migrationNumbersAreSequential(CountGooseMigrations(gapped).Names) {
		t.Error("a numbering gap must fail the sequential check")
	}
	if none := CountGooseMigrations(nil); len(none.Names) != 0 || none.AllMarked {
		t.Error("an absent directory must count as zero, unmarked")
	}
}

// TestBestChecklistPanelIsDeterministicFromItsFacts pins the boot-cache
// contract: the same facts shape byte-identical panels, which is what
// makes the served snapshot replay identically on every request.
func TestBestChecklistPanelIsDeterministicFromItsFacts(t *testing.T) {
	facts := bestCleanFacts()
	first, err := json.Marshal(BuildBestChecklistPanel(facts))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	second, err := json.Marshal(BuildBestChecklistPanel(facts))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(first) != string(second) {
		t.Error("two builds from identical facts produced different panels")
	}
}
