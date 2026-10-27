// Topic 36 environments' live demo: the env inspector. The payload is
// THIS process's resolved configuration, honestly derived at runtime:
// every variable the server reads, its validated value, whether it came
// from the environment or the code's dev-safe default, the
// twelve-factor scorecard as data, and the ScenarioStore's CURRENT
// state as the live flag panel. Sensitive values are REDACTED BY
// POLICY — a DSN's password (userinfo) and its entire query are never
// echoed anywhere in the payload, a rule the suite table-tests against
// postgres DSNs, mongo URIs with credentials, and query-borne tokens.
package demos

import (
	"net/url"
	"strconv"
	"strings"

	"guide/internal/core"
)

// EnvInspectorPath is the demo area's endpoint — registered on the real
// mux by the composition root.
const EnvInspectorPath = "/api/demo/env/inspector"

// The two attribution labels: where a variable's value came from, using
// the exact rule LoadConfig applies (a value is "from the environment"
// only when the variable is set to a non-blank string; empty and
// whitespace-only fall back to the default).
const (
	SourceEnvironment = "environment"
	SourceDefault     = "default"
)

// hidden marks values the redaction policy refuses to echo at all — a
// string in a sensitive slot that does not even parse as a URL could be
// anything, so nothing about it is shown.
const hidden = "(hidden)"

// RedactCredential applies the demo's one redaction rule to a
// URL-shaped connection string: the userinfo's PASSWORD and the ENTIRE
// query are dropped — either can carry credentials (postgres DSNs put
// the password in userinfo; mongo URIs and cloud DSNs put auth tokens
// in query parameters like password= or authSource=) — while scheme,
// username, host, port, and path are kept, so the row still shows WHERE
// the process talks to without ever showing HOW IT AUTHENTICATES. A
// string that does not parse as a URL is hidden entirely rather than
// echoed on trust: the policy never guesses what is sensitive, it
// refuses to print.
func RedactCredential(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" {
		return hidden
	}
	if parsed.User != nil {
		// url.User carries the username only; the password half of the
		// userinfo never survives this line.
		parsed.User = url.User(parsed.User.Username())
	}
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String()
}

// EnvInspectorVariable is one row of the resolved-config table: the Go
// field on core.Config, the environment variable that overrides it, the
// resolved value (redacted where the slot is sensitive), whether the
// value came from the environment or the default, whether the row is
// redacted, and the validation (or honest deferral) that gates the
// value at boot.
type EnvInspectorVariable struct {
	Field       string `json:"field"`
	EnvVar      string `json:"envVar"`
	Value       string `json:"value"`
	Source      string `json:"source"`
	Sensitive   bool   `json:"sensitive"`
	ValidatedBy string `json:"validatedBy"`
}

// EnvInspectorScoreRow is one factor of the twelve-factor scorecard as
// data: the factor's name, this repo's honest verdict ("hits" when the
// repo follows the factor, "differs" when it deliberately does not),
// and the one-line how that names the real machinery behind the
// verdict.
type EnvInspectorScoreRow struct {
	Factor  string `json:"factor"`
	Verdict string `json:"verdict"`
	How     string `json:"how"`
}

// EnvInspectorFlags is the live flag panel: the ScenarioStore's CURRENT
// state, snapshotted at request time — flip it with
// PATCH /api/demo/scenario and reload this payload to watch the panel
// follow. Live values, never fixtures.
type EnvInspectorFlags struct {
	DataMode      string `json:"dataMode"`
	InjectFailure string `json:"injectFailure"`
}

// EnvInspectorPayload is the wire shape of GET /api/demo/env/inspector.
type EnvInspectorPayload struct {
	Mode         string                 `json:"mode"`
	Env          string                 `json:"env"`
	Variables    []EnvInspectorVariable `json:"variables"`
	TwelveFactor []EnvInspectorScoreRow `json:"twelveFactor"`
	Flags        EnvInspectorFlags      `json:"flags"`
	Redaction    string                 `json:"redaction"`
	Reasoning    string                 `json:"reasoning"`
}

// variableRow derives one row from the cfg's resolved value, the
// variable's name, and the live environment: the VALUE is what the
// process booted with (validated, typed, immutable since), and the
// SOURCE re-applies LoadConfig's own rule — a non-blank environment
// value means "environment", anything else means the default was kept.
func variableRow(field, envVar, value, validatedBy string, sensitive bool, getenv func(string) string) EnvInspectorVariable {
	row := EnvInspectorVariable{
		Field:       field,
		EnvVar:      envVar,
		Value:       value,
		Source:      SourceDefault,
		Sensitive:   sensitive,
		ValidatedBy: validatedBy,
	}
	if strings.TrimSpace(getenv(envVar)) != "" {
		row.Source = SourceEnvironment
	}
	if sensitive {
		row.Value = RedactCredential(value)
	}
	return row
}

// twelveFactorScore is the honest walk of the twelve factors against
// this repo, as data the demo renders verbatim: ten hits, two
// deliberate differs — each verdict naming the real machinery that
// earned it.
func twelveFactorScore() []EnvInspectorScoreRow {
	return []EnvInspectorScoreRow{
		{Factor: "I. Codebase", Verdict: "hits",
			How: "one git tree serves every environment; dev/staging/prod differ only in config — the compose files ARE the environments"},
		{Factor: "II. Dependencies", Verdict: "hits",
			How: "go.mod + go.sum declare everything, MVS resolves it, `go mod tidy` prunes it — no implicit system packages"},
		{Factor: "III. Config", Verdict: "hits",
			How: "core.LoadConfig reads GDG_*/GRPC_ADDR from the environment — no config files baked into any image"},
		{Factor: "IV. Backing services", Verdict: "hits",
			How: "postgres/mongo/redis attach as URLs and addresses; the scenario store swaps fake/live adapters behind the same port"},
		{Factor: "V. Build, release, run", Verdict: "hits",
			How: "one build step produces identical bytes (the prod Dockerfile); a release is an image tag; run is config injection"},
		{Factor: "VI. Processes", Verdict: "hits",
			How: "state lives in the datastores; the process holds only caches — the in-memory ScenarioStore is demo machinery, not product state"},
		{Factor: "VII. Port binding", Verdict: "hits",
			How: "net/http listens on GDG_PORT inside the container — the process IS the server, exports HTTP and gRPC"},
		{Factor: "VIII. Concurrency", Verdict: "differs",
			How: "the guide hosts the jobs worker IN the API process so one container teaches everything; a real fleet splits them to scale independently"},
		{Factor: "IX. Disposability", Verdict: "hits",
			How: "bounded 30s boot, SIGTERM drains HTTP and gRPC inside timeouts — restartable at any moment"},
		{Factor: "X. Dev/prod parity", Verdict: "differs",
			How: "identical datastore engine versions in both composes, but the runtimes deliberately diverge — a fat devcontainer toolchain vs a distroless prod image"},
		{Factor: "XI. Logs", Verdict: "hits",
			How: "slog JSON to stdout, always — the container collects the stream; the process never opens a log file"},
		{Factor: "XII. Admin processes", Verdict: "differs",
			How: "goose migrations are EMBEDDED and run at boot — the single-binary trade-off, chosen over a separate migration admin process"},
	}
}

// BuildEnvInspectorPayload shapes the wire payload (pure): the resolved
// config table derived from the LIVE cfg and the live environment, the
// twelve-factor scorecard as data, and the flag panel reflecting the
// scenario store's CURRENT state. Values are what the process booted
// with; attribution is re-read at request time with the same rule
// LoadConfig applies — and sensitive slots are redacted before the
// payload exists, so no code path can serve them.
func BuildEnvInspectorPayload(cfg core.Config, getenv func(string) string, flags core.ScenarioState) EnvInspectorPayload {
	return EnvInspectorPayload{
		Mode: "runtime-derived: every row below was resolved by " +
			"core.LoadConfig when THIS process booted — typed, validated, " +
			"and immutable since; attribution re-reads the same environment " +
			"at request time, and the flag panel snapshots the ScenarioStore " +
			"as it is RIGHT NOW",
		Env: cfg.Env,
		Variables: []EnvInspectorVariable{
			variableRow("Port", "GDG_PORT", strconv.Itoa(cfg.Port),
				"parsed by strconv.Atoi at boot; must be 1–65535 or the boot fails", false, getenv),
			variableRow("Env", "GDG_ENV", cfg.Env,
				"free-form name documented as dev|staging|prod; reported by /api/health", false, getenv),
			variableRow("LogLevel", "GDG_LOG_LEVEL", cfg.LogLevel.String(),
				"parsed by slog.Level.UnmarshalText — one of debug, info, warn, error", false, getenv),
			variableRow("GRPCAddr", "GRPC_ADDR", cfg.GRPCAddr,
				"listened on by net.Listen at boot — an unusable address fails the boot", false, getenv),
			variableRow("DatabaseURL", "GDG_DATABASE_URL", cfg.DatabaseURL,
				"DSN shape is the driver's to validate: the pool dials at boot and the boot fails fast", true, getenv),
			variableRow("MongoURI", "GDG_MONGO_URI", cfg.MongoURI,
				"optional service: boot-connect degrades the console demo, never the boot", true, getenv),
			variableRow("RedisAddr", "GDG_REDIS_ADDR", cfg.RedisAddr,
				"lazy client — no boot ping; an unreachable Redis surfaces as the demo's typed 503", false, getenv),
		},
		TwelveFactor: twelveFactorScore(),
		Flags: EnvInspectorFlags{
			DataMode:      string(flags.DataMode),
			InjectFailure: string(flags.InjectFailure),
		},
		Redaction: "sensitive rows are redacted by policy: a DSN's " +
			"password (userinfo) and its ENTIRE query — where mongo and " +
			"cloud DSNs carry tokens — are never echoed; scheme, username, " +
			"host, port, and path stay so the row still shows where the " +
			"process talks to. The rule is table-tested against postgres " +
			"DSNs, mongo URIs, and query-borne tokens",
		Reasoning: "Read the table as the topic's whole argument in one " +
			"payload: names are the contract, defaults make dev " +
			"zero-config, validation makes misconfiguration a boot failure " +
			"instead of a runtime surprise, and redaction makes the " +
			"debugging habit of SHOWING your config safe by construction. " +
			"The scorecard walks the twelve factors honestly — the two " +
			"'differs' verdicts are deliberate, recorded trade-offs, not " +
			"gaps. The flag panel is the ScenarioStore's live state: flip " +
			"it with PATCH /api/demo/scenario and reload to watch the " +
			"payload follow",
	}
}
