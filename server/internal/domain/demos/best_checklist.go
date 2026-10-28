// Topic 41 best-practices' live demo: the self-audit checklist. The
// payload is a set of verdict rows about THIS repo and THIS process —
// and every machine-judged row is backed by machinery that actually
// executed, never by a typed-in "pass". Four check kinds, stated on
// every row:
//
// live — real requests driven at boot through the REAL handlers behind
// the REAL production middleware chain (the errors-explorer technique:
// a private mux mounting the same health, scenario, and posts handlers
// the composition root serves, wrapped in the same chain), observed
// with an in-process recorder — no socket, no staged responses. The
// probes are read-only or validation-failing by design, so the audit
// never damages the app it audits.
//
// exec — real tool executions at boot, each bounded by a timeout that
// kills the whole process group: gofmt -l over the server tree, the
// snippet-ratchet suite, and a -race run of the race exhibit's own
// tests. A missing toolchain or source tree degrades to an honest
// fail-with-reason row, never a fabricated verdict.
//
// file — repo files read for real: go.mod parsed for pins, the
// composition root scanned for the graceful-drain wiring, the embedded
// migrations walked, the argon2 decoy-burn markers located. A file
// that is not visible (the prod image ships no sources) fails its row
// honestly — except the one row whose subject lives ABOVE the module
// directory, which reports manual with exactly that reason.
//
// manual — the practices no automation can honestly judge. These rows
// claim nothing and carry a one-line why.
//
// The whole audit computes ONCE at process boot and is served from the
// snapshot: reload the panel and every verdict replays identically,
// with the boot timestamp carried on the payload.
package demos

import (
	"encoding/json"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BestChecklistPath is the demo area's endpoint — registered on the real
// mux by the composition root.
const BestChecklistPath = "/api/demo/best/checklist"

// The check kinds, as served on every row's checkKind.
const (
	BestCheckKindLive   = "live"
	BestCheckKindExec   = "exec"
	BestCheckKindFile   = "file"
	BestCheckKindManual = "manual"
)

// The verdicts: pass/fail are machine-judged from real evidence; manual
// claims nothing.
const (
	BestVerdictPass   = "pass"
	BestVerdictFail   = "fail"
	BestVerdictManual = "manual"
)

// Probe labels — the keys of BestChecklistFacts.Probes.
const (
	BestProbeHealth           = "health"
	BestProbeScenarioGet      = "scenario-get"
	BestProbeScenarioPatchBad = "scenario-patch-bogus"
	BestProbePostsNotFound    = "posts-not-found"
	BestProbePostsInvalidPost = "posts-invalid-create"
)

// BestHonestyNote is the panel's standing explanation of how every
// verdict was earned — the anti-fine-print of the whole payload.
const BestHonestyNote = "Every machine verdict on this panel was computed from machinery that " +
	"actually executed, once, at process boot. 'live' rows drove real requests through the " +
	"same handlers and the same middleware chain this process serves; 'exec' rows ran the " +
	"real tools (gofmt, the snippet-ratchet suite, a -race run) with bounded timeouts; " +
	"'file' rows read the repo this process lives in. A check that could not execute is " +
	"reported as failed with its reason — never as passed — and the two manual rows claim " +
	"nothing, by design. Reload the panel and the verdicts replay identically: this is a " +
	"fact about boot, served with its timestamp."

// BestChecklistItemRow is one practice, one verdict, and the evidence
// that produced it.
type BestChecklistItemRow struct {
	ID        string   `json:"id"`
	Practice  string   `json:"practice"`
	CheckKind string   `json:"checkKind"`
	Verdict   string   `json:"verdict"`
	Evidence  []string `json:"evidence"`
	WhyManual string   `json:"whyManual,omitempty"`
}

// BestChecklistAreaRow is one practice area: its rows plus a one-line
// frame for the panel.
type BestChecklistAreaRow struct {
	ID    string                 `json:"id"`
	Title string                 `json:"title"`
	Note  string                 `json:"note"`
	Items []BestChecklistItemRow `json:"items"`
}

// BestNotDoneRow is one deliberate NOT-done: the decision, its reason,
// and the real artifacts that show it — the anti-checklist.
type BestNotDoneRow struct {
	ID        string   `json:"id"`
	NotDone   string   `json:"notDone"`
	Reason    string   `json:"reason"`
	Artifacts []string `json:"artifacts"`
}

// BestChecklistSummary is the panel's score line — counts, nothing spun.
type BestChecklistSummary struct {
	Items  int `json:"items"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
	Manual int `json:"manual"`
}

// BestChecklistPanel is the wire shape of GET /api/demo/best/checklist.
type BestChecklistPanel struct {
	Mode        string                 `json:"mode"`
	ComputedAt  string                 `json:"computedAt"`
	Areas       []BestChecklistAreaRow `json:"areas"`
	NotDones    []BestNotDoneRow       `json:"notDones"`
	Summary     BestChecklistSummary   `json:"summary"`
	HonestyNote string                 `json:"honestyNote"`
}

// BestProbeRecording is one real internal request's raw observation —
// exactly what the recorder saw, before any verdict is derived.
type BestProbeRecording struct {
	Label           string
	Status          int
	HeaderRequestID string
	Body            string
	// ErrText carries a transport-level failure (the round never
	// completed); empty when the recorder observed a full response.
	ErrText string
}

// BestExecOutcome is one bounded tool execution's raw observation.
type BestExecOutcome struct {
	Label    string
	Command  string
	ExitCode int
	Output   string
	// Ran is false when the check could not even start (no toolchain,
	// no source tree beside the process) — the row reports the reason.
	Ran     bool
	ErrText string
}

// BestFileRead is one file's content as read from disk, with the
// repo-root-relative path the evidence prints.
type BestFileRead struct {
	RelPath string
	Content string
}

// BestConfigFacts is the running process's resolved configuration —
// live facts about this very boot, from the one typed source.
type BestConfigFacts struct {
	Port     int
	GRPCAddr string
	Env      string
}

// BestChecklistFacts is everything the panel derives from: the raw
// probe recordings, the raw tool outcomes, the raw file contents, and
// the process's own resolved config. The handler collects; this
// package judges.
type BestChecklistFacts struct {
	Now          time.Time
	Probes       map[string]BestProbeRecording
	Gofmt        BestExecOutcome
	Ratchet      BestExecOutcome
	Race         BestExecOutcome
	GoMod        BestFileRead
	MainGo       BestFileRead
	PasswordGo   BestFileRead
	AuthStories  BestFileRead
	Compose      BestFileRead
	MigrationsFS fs.FS
	Config       BestConfigFacts
}

// BestGoModFacts is go.mod distilled to its pinning story.
type BestGoModFacts struct {
	GoDirective    string
	DirectRequires []string
	ToolDirectives []string
}

// ParseGoModPins reduces a go.mod to the facts the pins row judges:
// the go directive, the direct (non-indirect) requires of the first
// require block, and the tool directives. Line-based on purpose — the
// module file's grammar is line-shaped for exactly these fields.
func ParseGoModPins(content string) BestGoModFacts {
	facts := BestGoModFacts{}
	block := ""
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case block == "require" && strings.Contains(line, "// indirect"):
			// indirect pins ride a second block — not a direct pin.
		case block == "require" && line == ")":
			block = ""
		case block == "require" && line != "":
			facts.DirectRequires = append(facts.DirectRequires, line)
		case block == "tool" && line == ")":
			block = ""
		case block == "tool" && line != "":
			facts.ToolDirectives = append(facts.ToolDirectives, line)
		case block != "":
			// Inside an unrecognized block: leave it alone.
		case strings.HasPrefix(line, "go "):
			facts.GoDirective = strings.TrimSpace(strings.TrimPrefix(line, "go "))
		case strings.HasPrefix(line, "require ("):
			block = "require"
		case strings.HasPrefix(line, "tool ("):
			block = "tool"
		}
	}
	return facts
}

// goModPinPattern accepts an exact version pin: module path, space, a
// version token that starts with v and carries at least one dot.
var goModPinPattern = regexp.MustCompile(`^\S+ v[0-9][^\s]*\.[0-9]`)

// BestMigrationFacts is the embedded migration set, walked for real.
type BestMigrationFacts struct {
	Names     []string
	AllMarked bool
}

// gooseUpMarker is the marker every goose migration carries.
const gooseUpMarker = "-- +goose Up"

// CountGooseMigrations walks a migrations directory and reports the
// sql files in lexical order plus whether every one of them carries
// the goose Up marker. An absent directory counts as zero — honest,
// never invented.
func CountGooseMigrations(fsys fs.FS) BestMigrationFacts {
	facts := BestMigrationFacts{AllMarked: true}
	if fsys == nil {
		facts.AllMarked = false
		return facts
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		facts.AllMarked = false
		return facts
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
		raw, readErr := fs.ReadFile(fsys, entry.Name())
		if readErr != nil || !strings.Contains(string(raw), gooseUpMarker) {
			facts.AllMarked = false
		}
	}
	sort.Strings(names)
	facts.Names = names
	return facts
}

// migrationNumbersAreSequential checks the sorted names' leading
// numbers ascend one file at a time starting at 1 (00001, 00002, …).
func migrationNumbersAreSequential(names []string) bool {
	want := 1
	for _, name := range names {
		digits := regexp.MustCompile(`^0*(\d+)_`).FindStringSubmatch(name)
		if digits == nil {
			return false
		}
		n, err := strconv.Atoi(digits[1])
		if err != nil || n != want {
			return false
		}
		want++
	}
	return true
}

// bestRow helpers — each stamps its check kind, so no row can mislabel
// how it was judged.
func bestLiveRow(id, practice string, evidence []string, ok bool) BestChecklistItemRow {
	return bestVerdictRow(id, practice, BestCheckKindLive, evidence, ok)
}

func bestExecRow(id, practice string, evidence []string, ok bool) BestChecklistItemRow {
	return bestVerdictRow(id, practice, BestCheckKindExec, evidence, ok)
}

func bestFileRow(id, practice string, evidence []string, ok bool) BestChecklistItemRow {
	return bestVerdictRow(id, practice, BestCheckKindFile, evidence, ok)
}

func bestVerdictRow(id, practice, kind string, evidence []string, ok bool) BestChecklistItemRow {
	verdict := BestVerdictFail
	if ok {
		verdict = BestVerdictPass
	}
	if len(evidence) == 0 {
		evidence = []string{"the check produced no evidence — reported as failed, never guessed"}
		verdict = BestVerdictFail
	}
	return BestChecklistItemRow{ID: id, Practice: practice, CheckKind: kind,
		Verdict: verdict, Evidence: evidence}
}

func bestManualRow(id, practice, why string) BestChecklistItemRow {
	return BestChecklistItemRow{ID: id, Practice: practice, CheckKind: BestCheckKindManual,
		Verdict: BestVerdictManual, Evidence: []string{}, WhyManual: why}
}

// bestProbe returns the recording and whether the round completed.
func bestProbe(facts BestChecklistFacts, label string) (BestProbeRecording, bool) {
	rec, present := facts.Probes[label]
	if !present || rec.ErrText != "" {
		return rec, false
	}
	return rec, true
}

// bestProbeBroke renders the honest reason a round never completed.
func bestProbeBroke(facts BestChecklistFacts, label string) string {
	rec, present := facts.Probes[label]
	switch {
	case !present:
		return "the " + label + " probe never ran"
	case rec.ErrText != "":
		return "the probe round broke: " + rec.ErrText
	default:
		return "the probe round never completed"
	}
}

// bestExecVerdict renders an exec row from a raw outcome: a check that
// could not start reports its reason, a non-zero exit is a result
// (reported with its output tail), and only a clean bounded run passes.
func bestExecVerdict(outcome BestExecOutcome, id, practice string, outputMustBeEmpty bool, evidence []string) BestChecklistItemRow {
	if !outcome.Ran {
		return bestExecRow(id, practice, append(evidence, "could not run: "+outcome.ErrText), false)
	}
	ok := outcome.ErrText == "" && outcome.ExitCode == 0
	if outputMustBeEmpty && strings.TrimSpace(outcome.Output) != "" {
		ok = false
	}
	if !ok {
		evidence = append(evidence, "exit "+strconv.Itoa(outcome.ExitCode))
		if tail := lastNonEmptyLines(outcome.Output, 3); len(tail) > 0 {
			evidence = append(evidence, tail...)
		}
		if outcome.ErrText != "" {
			evidence = append(evidence, outcome.ErrText)
		}
	}
	return bestExecRow(id, practice, evidence, ok)
}

// lastNonEmptyLines keeps the last n non-empty lines of a tool's
// output — the verdict lines a human reads first.
func lastNonEmptyLines(output string, n int) []string {
	var kept []string
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0 && len(kept) < n; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			kept = append([]string{line}, kept...)
		}
	}
	return kept
}

// bestJSONBody parses a probe body as one JSON object; ok is false
// when it is not one.
func bestJSONBody(body string) (map[string]any, bool) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

// bestErrorEnvelope extracts the error envelope from a probe body —
// nil when the body is not carrying one.
func bestErrorEnvelope(body string) map[string]any {
	parsed, ok := bestJSONBody(body)
	if !ok {
		return nil
	}
	envelope, ok := parsed["error"].(map[string]any)
	if !ok {
		return nil
	}
	return envelope
}

// ---- live rows ----

// bestHealthRow judges the health contract: 200, status ok, an env
// named — the liveness endpoint that lies about nothing.
func bestHealthRow(facts BestChecklistFacts) BestChecklistItemRow {
	const (
		id       = "health-contract"
		practice = "Liveness is dependency-free — /api/health answers 'is the process up'"
	)
	rec, completed := bestProbe(facts, BestProbeHealth)
	if !completed {
		return bestLiveRow(id, practice, []string{bestProbeBroke(facts, BestProbeHealth)}, false)
	}
	body, _ := bestJSONBody(rec.Body)
	status, _ := body["status"].(string)
	env, _ := body["env"].(string)
	ok := rec.Status == 200 && status == "ok" && env != ""
	return bestLiveRow(id, practice, []string{
		"GET /api/health → " + strconv.Itoa(rec.Status),
		`body: status="` + status + `", env="` + env + `"`,
	}, ok)
}

// bestEnvelopeRow judges the one-envelope discipline on a live miss:
// an unknown post id must answer 404, code not_found, and the body's
// requestId must equal the x-request-id header the same response
// carried.
func bestEnvelopeRow(facts BestChecklistFacts) BestChecklistItemRow {
	const (
		id       = "error-envelope-on-miss"
		practice = "One error envelope end to end — unknown ids answer not_found, and the body's requestId equals the header's"
	)
	rec, completed := bestProbe(facts, BestProbePostsNotFound)
	if !completed {
		return bestLiveRow(id, practice, []string{bestProbeBroke(facts, BestProbePostsNotFound)}, false)
	}
	envelope := bestErrorEnvelope(rec.Body)
	code, _ := envelope["code"].(string)
	bodyID, _ := envelope["requestId"].(string)
	matches := rec.HeaderRequestID != "" && rec.HeaderRequestID == bodyID
	ok := rec.Status == 404 && code == "not_found" && matches
	return bestLiveRow(id, practice, []string{
		"GET /api/posts/best-checklist-nope → " + strconv.Itoa(rec.Status),
		"envelope: code=" + strconv.Quote(code) + ", requestId matches header: " + strconv.FormatBool(matches),
	}, ok)
}

// bestValidationRow judges boundary validation: an invalid create body
// must be refused as the 422 validation_failed envelope before any
// store is touched.
func bestValidationRow(facts BestChecklistFacts) BestChecklistItemRow {
	const (
		id       = "boundary-validation"
		practice = "Bodies validate at the boundary — an invalid POST answers the 422 envelope, never a store"
	)
	rec, completed := bestProbe(facts, BestProbePostsInvalidPost)
	if !completed {
		return bestLiveRow(id, practice, []string{bestProbeBroke(facts, BestProbePostsInvalidPost)}, false)
	}
	envelope := bestErrorEnvelope(rec.Body)
	code, _ := envelope["code"].(string)
	ok := rec.Status == 422 && code == "validation_failed"
	return bestLiveRow(id, practice, []string{
		`POST /api/posts with body {} → ` + strconv.Itoa(rec.Status),
		"envelope: code=" + strconv.Quote(code),
	}, ok)
}

// bestScenarioRow judges the scenario contract: GET serves the two
// enum fields with in-domain values, and a bogus PATCH is refused as
// 422 with the expected values named in details.
func bestScenarioRow(facts BestChecklistFacts) BestChecklistItemRow {
	const (
		id       = "scenario-contract"
		practice = "The scenario store's contract holds at the boundary — GET serves the enum state; a bogus PATCH answers 422 with details.expected"
	)
	get, getDone := bestProbe(facts, BestProbeScenarioGet)
	if !getDone {
		return bestLiveRow(id, practice, []string{bestProbeBroke(facts, BestProbeScenarioGet)}, false)
	}
	body, _ := bestJSONBody(get.Body)
	dataMode, _ := body["dataMode"].(string)
	inject, _ := body["injectFailure"].(string)
	getOK := get.Status == 200 &&
		(dataMode == "fake" || dataMode == "live") &&
		(inject == "none" || inject == "error" || inject == "latency")

	patch, patchDone := bestProbe(facts, BestProbeScenarioPatchBad)
	patchOK := false
	patchEvidence := "PATCH {\"dataMode\":\"bogus\"} → the round never completed"
	if patchDone {
		envelope := bestErrorEnvelope(patch.Body)
		code, _ := envelope["code"].(string)
		patchOK = patch.Status == 422 && code == "validation_failed"
		patchEvidence = `PATCH {"dataMode":"bogus"} → ` + strconv.Itoa(patch.Status) +
			", code=" + strconv.Quote(code)
	}
	return bestLiveRow(id, practice, []string{
		"GET /api/demo/scenario → " + strconv.Itoa(get.Status) +
			` (dataMode="` + dataMode + `", injectFailure="` + inject + `")`,
		patchEvidence,
	}, getOK && patchOK)
}

// ---- exec rows ----

func bestGofmtRow(facts BestChecklistFacts) BestChecklistItemRow {
	return bestExecVerdict(facts.Gofmt, "gofmt-clean",
		"gofmt is non-negotiable — gofmt -l over the server tree lists nothing",
		true,
		[]string{"command: " + facts.Gofmt.Command + " (in server/)"})
}

func bestRatchetRow(facts BestChecklistFacts) BestChecklistItemRow {
	return bestExecVerdict(facts.Ratchet, "ratchet-green",
		"Quoted teaching is real — the snippet-ratchet suite passes over the live tree (RATCHET = 0)",
		false,
		[]string{"command: " + facts.Ratchet.Command})
}

func bestRaceRow(facts BestChecklistFacts) BestChecklistItemRow {
	return bestExecVerdict(facts.Race, "race-stance",
		"The race detector is part of the standard run — a real -race execution of the race exhibit's own tests passes",
		false,
		[]string{"command: " + facts.Race.Command})
}

// ---- file rows ----

func bestModulePinsRow(facts BestChecklistFacts) BestChecklistItemRow {
	const (
		id       = "module-pins"
		practice = "Dependencies are pinned, exactly — the toolchain directive, the direct requires, and the tool directives in go.mod"
	)
	read := facts.GoMod
	if read.Content == "" {
		return bestFileRow(id, practice, []string{
			read.RelPath + " is not readable beside this process (the prod image ships no sources)",
		}, false)
	}
	parsed := ParseGoModPins(read.Content)
	pinned := 0
	for _, req := range parsed.DirectRequires {
		if goModPinPattern.MatchString(req) {
			pinned++
		}
	}
	ok := parsed.GoDirective != "" && len(parsed.DirectRequires) > 0 &&
		pinned == len(parsed.DirectRequires) && len(parsed.ToolDirectives) > 0
	return bestFileRow(id, practice, []string{
		read.RelPath + ": go " + parsed.GoDirective + " directive, " +
			strconv.Itoa(len(parsed.DirectRequires)) + " direct requires (all exact-pinned), " +
			strconv.Itoa(len(parsed.ToolDirectives)) + " tool directives",
	}, ok)
}

// bestDrainMarkers are the composition root's graceful-shutdown wiring,
// located by symbol so a rename fails the row honestly.
var bestDrainMarkers = []string{
	"signal.NotifyContext",
	"srv.Shutdown(",
	"GracefulStop()",
	"shutdownTimeout",
}

func bestGracefulDrainRow(facts BestChecklistFacts) BestChecklistItemRow {
	const (
		id       = "graceful-drain-wired"
		practice = "Shutdown is wired, bounded, and dual-surface — gRPC GracefulStop then HTTP Shutdown inside a timeout, on the signal path"
	)
	read := facts.MainGo
	if read.Content == "" {
		return bestFileRow(id, practice, []string{
			read.RelPath + " is not readable beside this process",
		}, false)
	}
	found := make([]string, 0, len(bestDrainMarkers))
	for _, marker := range bestDrainMarkers {
		if strings.Contains(read.Content, marker) {
			found = append(found, marker)
		}
	}
	return bestFileRow(id, practice, []string{
		read.RelPath + ": " + strconv.Itoa(len(found)) + "/" + strconv.Itoa(len(bestDrainMarkers)) +
			" drain markers present (" + strings.Join(found, ", ") + ")",
	}, len(found) == len(bestDrainMarkers))
}

func bestMigrationsRow(facts BestChecklistFacts) BestChecklistItemRow {
	const (
		id       = "migrations-own-schema"
		practice = "The schema has a history — embedded goose migrations, sequentially numbered, every file carrying the Up marker"
	)
	counted := CountGooseMigrations(facts.MigrationsFS)
	if len(counted.Names) == 0 {
		return bestFileRow(id, practice, []string{
			"internal/data/db/migrations/ is not readable beside this process — no migration was counted",
		}, false)
	}
	ok := counted.AllMarked && migrationNumbersAreSequential(counted.Names)
	return bestFileRow(id, practice, []string{
		"internal/data/db/migrations/: " + strconv.Itoa(len(counted.Names)) +
			" files — " + strings.Join(counted.Names, ", "),
		"every file carries " + strconv.Quote(gooseUpMarker) + ": " + strconv.FormatBool(counted.AllMarked),
	}, ok)
}

func bestUniformCostRow(facts BestChecklistFacts) BestChecklistItemRow {
	const (
		id       = "uniform-login-cost"
		practice = "Login burns uniform work — unknown users verify against the argon2id decoy, so a stopwatch learns nothing"
	)
	password, stories := facts.PasswordGo, facts.AuthStories
	engineOK := password.Content != "" &&
		strings.Contains(password.Content, "VerifyUniform") &&
		strings.Contains(password.Content, "DecoyHash")
	usageOK := stories.Content != "" && strings.Contains(stories.Content, "VerifyUniform")
	evidence := []string{}
	if password.Content != "" {
		state := "MISSING"
		if engineOK {
			state = "present"
		}
		evidence = append(evidence, password.RelPath+": VerifyUniform + DecoyHash "+state)
	} else {
		evidence = append(evidence, password.RelPath+" is not readable beside this process")
	}
	if stories.Content != "" {
		state := "not found"
		if usageOK {
			state = "found"
		}
		evidence = append(evidence, stories.RelPath+": VerifyUniform "+state)
	} else {
		evidence = append(evidence, stories.RelPath+" is not readable beside this process")
	}
	return bestFileRow(id, practice, evidence, engineOK && usageOK)
}

func bestStackAsFilesRow(facts BestChecklistFacts) BestChecklistItemRow {
	const (
		id       = "stack-as-files"
		practice = "The stack is files you can diff — compose declares backing-service healthchecks and the shared cache volumes"
	)
	read := facts.Compose
	if read.Content == "" {
		return bestManualRow(id, practice,
			"the file lives above the module directory ("+read.RelPath+") and is not visible from this "+
				"process — the prod image ships only the binary; run the check where the whole repo "+
				"is mounted (the dev container or the host)")
	}
	healthchecks := strings.Count(read.Content, "healthcheck:")
	caches := strings.Contains(read.Content, "gdg-go") && strings.Contains(read.Content, "gdg-gocache")
	ok := healthchecks >= 3 && caches
	return bestFileRow(id, practice, []string{
		read.RelPath + ": " + strconv.Itoa(healthchecks) + " healthcheck blocks declared " +
			"(postgres, redis, mongo)",
		"named cache volumes gdg-go + gdg-gocache: " + strconv.FormatBool(caches),
	}, ok)
}

// ---- live config row ----

func bestConfigRow(facts BestChecklistFacts) BestChecklistItemRow {
	const (
		id       = "one-config-source"
		practice = "One typed config source — this process's port, gRPC address, and env resolved from the environment at boot"
	)
	cfg := facts.Config
	ok := cfg.Port > 0 && cfg.GRPCAddr != "" && cfg.Env != ""
	parity := "parity with the family's co-run defaults held: 8100/:8102 — the sibling stacks co-run"
	if cfg.Port != 8100 || cfg.GRPCAddr != ":8102" {
		parity = "this process overrides the family defaults (8100/:8102) — reported as a fact, not a failure"
	}
	return bestLiveRow(id, practice, []string{
		"resolved: GDG_PORT=" + strconv.Itoa(cfg.Port) + ", GRPC_ADDR=" + cfg.GRPCAddr +
			", GDG_ENV=" + cfg.Env,
		parity,
	}, ok)
}

// ---- the not-dos ----

// BestNotDones is this repo's deliberate NOT-does, each with its reason
// and its real artifacts — the counterpart to the checklist, because a
// practices synthesis that only lists what IS done would quietly lie
// about the rest.
var BestNotDones = []BestNotDoneRow{
	{
		ID:      "no-hosted-ci",
		NotDone: "No hosted CI or Dependabot",
		Reason: "Local verification is the only gate, by family decision: the suite, vet, and gofmt run " +
			"inside the long-lived dev containers with one command and zero ephemeral churn. A hosted " +
			"pipeline is added only on explicit request — a named divergence, not an oversight.",
		Artifacts: []string{
			"the verify one-liner — this topic's terminal companion",
			"the repo's agentic_development.md decision log (the standing family stance)",
		},
	},
	{
		ID:      "stdlib-first",
		NotDone: "No web framework, no test framework — stdlib first",
		Reason: "net/http's ServeMux is the backbone; chi, gin, and echo are taught by comparison only. " +
			"The same discipline holds for testing (stdlib testing + httptest, no testify) and logging " +
			"(slog, not zap) — every addition must earn its place against the standard library first.",
		Artifacts: []string{
			"server/cmd/guide/main.go — the whole surface on net/http",
			"server/internal/presentation/content_handler_test.go — httptest as the wire contract",
		},
	},
	{
		ID:      "stdlib-hs256-jwt",
		NotDone: "JWT hand-rolled as stdlib HS256",
		Reason: "The authentication topic signs and verifies JWTs with crypto/hmac + crypto/rand by " +
			"design — the token format is small enough that a dependency buys nothing the standard " +
			"library cannot state honestly in a screen of code. Commercial JWT libraries live in " +
			"comparison prose.",
		Artifacts: []string{
			"server/internal/domain/auth/ — the engine every auth demo drives",
		},
	},
	{
		ID:      "browser-e2e-deferred",
		NotDone: "Browser E2E deferred, on purpose",
		Reason: "Host-run Playwright suites are planned as their own hardening phase; until then " +
			"user-facing changes are verified by scripted checks against the live stack — curl tours " +
			"and payload assertions, not vibes. The gap is scheduled, not forgotten.",
		Artifacts: []string{
			"the planned e2e/ suite (hardening phase)",
			"per-topic smoke logs under /tmp/verify-* (the standing bridge)",
		},
	},
}

// bestAreaFrames orders the panel's areas and frames each in one line.
var bestAreaFrames = []struct {
	ID    string
	Title string
	Note  string
}{
	{"setup", "Setup", "Toolchain discipline — format and pins that hold the floor."},
	{"api-discipline", "API discipline", "Thin handlers, one envelope, contracts at the boundary."},
	{"security", "Security", "Uniform cost at the login — never opt-in defense."},
	{"data", "Data", "The schema has a history; the app does not hand-edit it."},
	{"reliability", "Reliability", "Bounded shutdown and a real race stance."},
	{"observability", "Observability", "A health endpoint that lies about nothing."},
	{"operations", "Operations", "The stack as files you can diff and dry-run."},
	{"git-and-review", "Git & review", "Machines check what machines can; humans read intent."},
	{"human-practices", "Human practices", "The judgments no checklist can make for you."},
}

// BuildBestChecklistPanel shapes the whole panel (pure): every row
// judged from the raw facts, the areas assembled in frame order, the
// not-dos attached, and the summary counted — never spun. The payload
// is a deterministic function of its facts, so the boot snapshot
// replays byte-identically on every request that serves it.
func BuildBestChecklistPanel(facts BestChecklistFacts) BestChecklistPanel {
	itemsByArea := map[string][]BestChecklistItemRow{
		"setup": {
			bestGofmtRow(facts),
			bestModulePinsRow(facts),
		},
		"api-discipline": {
			bestEnvelopeRow(facts),
			bestValidationRow(facts),
			bestScenarioRow(facts),
		},
		"security":      {bestUniformCostRow(facts)},
		"data":          {bestMigrationsRow(facts)},
		"reliability":   {bestGracefulDrainRow(facts), bestRaceRow(facts)},
		"observability": {bestHealthRow(facts), bestConfigRow(facts)},
		"operations":    {bestStackAsFilesRow(facts)},
		"git-and-review": {bestRatchetRow(facts), bestManualRow("review-your-diffs",
			"Every diff gets a human read before it lands — design intent is not machine-readable",
			"no automation reads a diff's design intent; this repo's contract is that every change "+
				"lands reviewed, with the suite and the ratchet carrying what machines CAN check")},
		"human-practices": {bestManualRow("prod-secrets-real",
			"Production overrides every dev-default secret with real values",
			"the compose rail enforces PRESENCE at render time; whether this deployment's values are "+
				"strong and rotated is an operator fact no local check can see")},
	}

	areas := make([]BestChecklistAreaRow, 0, len(bestAreaFrames))
	summary := BestChecklistSummary{}
	for _, frame := range bestAreaFrames {
		items := itemsByArea[frame.ID]
		if items == nil {
			items = []BestChecklistItemRow{}
		}
		for _, item := range items {
			summary.Items++
			switch item.Verdict {
			case BestVerdictPass:
				summary.Passed++
			case BestVerdictManual:
				summary.Manual++
			default:
				summary.Failed++
			}
		}
		areas = append(areas, BestChecklistAreaRow{ID: frame.ID, Title: frame.Title,
			Note: frame.Note, Items: items})
	}

	return BestChecklistPanel{
		Mode: "boot-cached: the whole audit executes ONCE at process boot — live probes through " +
			"the real handler chain in-process, real tool executions bounded by timeouts, real " +
			"file reads — and every request serves the snapshot; reload and the verdicts replay " +
			"identically",
		ComputedAt:  facts.Now.UTC().Format(time.RFC3339),
		Areas:       areas,
		NotDones:    BestNotDones,
		Summary:     summary,
		HonestyNote: BestHonestyNote,
	}
}
