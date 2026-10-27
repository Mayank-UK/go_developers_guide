package demos

import (
	"encoding/json"
	"strings"
	"testing"

	"guide/internal/core"
)

// The env inspector's payload is derived, never typed by hand: the
// table below pins the three honesty legs the topic teaches —
// redaction completeness (no credential survives anywhere in the
// payload), attribution truthfulness (the source column re-applies
// LoadConfig's own rule, vars set AND unset), and flag-panel liveness
// (a store flip is reflected by the very next build, then restored).

func TestRedactCredentialNeverEchoesCredentials(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "postgres DSN with password and query",
			raw:  "postgres://postgres:guide@postgres:5432/guide?sslmode=disable",
			want: "postgres://postgres@postgres:5432/guide",
		},
		{
			name: "postgres DSN with special-character password",
			raw:  "postgres://u:p%40ss%3Aw0rd@db.internal:5432/app",
			want: "postgres://u@db.internal:5432/app",
		},
		{
			name: "mongo URI with credentials and authSource",
			raw:  "mongodb://guideUser:guidePass@mongo:27017/?authSource=admin",
			want: "mongodb://guideUser@mongo:27017/",
		},
		{
			name: "cloud-style DSN with a token in the query",
			raw:  "postgres://app@broker.example.com:5432/db?password=hunter2&sslmode=require",
			want: "postgres://app@broker.example.com:5432/db",
		},
		{
			name: "mongo URI without credentials keeps its shape",
			raw:  "mongodb://mongo:27017",
			want: "mongodb://mongo:27017",
		},
		{
			name: "postgres DSN, username only, no password",
			raw:  "postgres://postgres@localhost:5432/guide",
			want: "postgres://postgres@localhost:5432/guide",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RedactCredential(tc.raw)
			if got != tc.want {
				t.Errorf("RedactCredential(%q) = %q, want %q", tc.raw, got, tc.want)
			}
			// The credential halves of the input must not survive — checked
			// against the raw input so percent-encoded forms are held to the
			// same standard.
			for _, secret := range []string{"guide@", "guidePass", "hunter2", "p%40ss%3Aw0rd"} {
				if strings.Contains(tc.raw, secret) && strings.Contains(got, secret) {
					t.Errorf("redacted %q still carries %q", got, secret)
				}
			}
		})
	}
}

func TestRedactCredentialHidesUnparseableStringsEntirely(t *testing.T) {
	t.Parallel()

	// A string that does not parse as a URL could carry a credential in
	// any grammar — the policy refuses to echo any of it.
	for _, raw := range []string{
		"not a url at all",
		"host=postgres password=guide port=5432",
		"\x7f control characters",
	} {
		if got := RedactCredential(raw); got != hidden {
			t.Errorf("RedactCredential(%q) = %q, want the hidden marker", raw, got)
		}
	}
}

func TestBuildEnvInspectorPayloadAttributionFollowsLoadConfigsRule(t *testing.T) {
	t.Parallel()

	cfg, err := core.LoadConfig(func(key string) string {
		if key == "GDG_PORT" {
			return "9001"
		}
		if key == "GDG_DATABASE_URL" {
			return "postgres://postgres:supersecret@db:5432/guide?sslmode=require"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	payload := BuildEnvInspectorPayload(cfg, func(key string) string {
		if key == "GDG_PORT" {
			return "9001"
		}
		if key == "GDG_DATABASE_URL" {
			return "postgres://postgres:supersecret@db:5432/guide?sslmode=require"
		}
		return ""
	}, core.NewScenarioStore().Snapshot())

	byVar := make(map[string]EnvInspectorVariable, len(payload.Variables))
	for _, row := range payload.Variables {
		byVar[row.EnvVar] = row
	}

	// Set vars are attributed to the environment; every other var fell
	// back to its default — the exact rule LoadConfig applies.
	if row := byVar["GDG_PORT"]; row.Source != SourceEnvironment || row.Value != "9001" {
		t.Errorf("GDG_PORT row = %+v, want source environment value 9001", row)
	}
	for _, envVar := range []string{"GDG_ENV", "GDG_LOG_LEVEL", "GRPC_ADDR", "GDG_MONGO_URI", "GDG_REDIS_ADDR"} {
		if row := byVar[envVar]; row.Source != SourceDefault {
			t.Errorf("%s row source = %q, want default (var unset)", envVar, row.Source)
		}
	}

	// Whitespace-only counts as unset — LoadConfig trims before deciding.
	payload = BuildEnvInspectorPayload(cfg, func(string) string { return "   " },
		core.NewScenarioStore().Snapshot())
	for _, row := range payload.Variables {
		if row.Source != SourceDefault {
			t.Errorf("whitespace-only environment must attribute to default, got %q for %s", row.Source, row.EnvVar)
		}
	}
}

func TestBuildEnvInspectorPayloadRedactsSensitiveSlots(t *testing.T) {
	t.Parallel()

	cfg := core.Config{
		Port:        8100,
		Env:         "test",
		DatabaseURL: "postgres://postgres:trustno1@postgres:5432/guide?sslmode=disable",
		MongoURI:    "mongodb://u:sekret@mongo:27017/?authSource=admin",
		RedisAddr:   "redis:6379",
	}
	payload := BuildEnvInspectorPayload(cfg, func(string) string { return "" },
		core.NewScenarioStore().Snapshot())

	// The whole payload — not just the value fields — must be free of
	// the credentials, and the sensitive rows must carry their redacted
	// shapes. JSON-encode so nothing escapes through a field tag or note.
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, secret := range []string{"trustno1", "sekret", "authSource=admin", "sslmode=disable"} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("payload leaks %q: %s", secret, encoded)
		}
	}
	for _, row := range payload.Variables {
		if !row.Sensitive {
			continue
		}
		if row.EnvVar == "GDG_DATABASE_URL" && row.Value != "postgres://postgres@postgres:5432/guide" {
			t.Errorf("DatabaseURL redaction = %q", row.Value)
		}
		if row.EnvVar == "GDG_MONGO_URI" && row.Value != "mongodb://u@mongo:27017/" {
			t.Errorf("MongoURI redaction = %q", row.Value)
		}
	}
	// Non-sensitive rows keep their plain values — the policy redacts the
	// two URL slots, not the table.
	foundPlain := 0
	for _, row := range payload.Variables {
		if !row.Sensitive {
			foundPlain++
		}
	}
	if foundPlain != 5 {
		t.Errorf("non-sensitive rows = %d, want 5 (port, env, log level, grpc addr, redis addr)", foundPlain)
	}
}

func TestBuildEnvInspectorFlagPanelReflectsTheLiveStore(t *testing.T) {
	t.Parallel()

	store := core.NewScenarioStore()
	cfg := core.Config{Env: "test"}
	payload := BuildEnvInspectorPayload(cfg, func(string) string { return "" }, store.Snapshot())
	if payload.Flags.DataMode != string(core.DataModeFake) || payload.Flags.InjectFailure != string(core.FailureNone) {
		t.Errorf("default panel = %+v, want fake/none", payload.Flags)
	}

	// Flip the store and rebuild: the panel follows the CURRENT state —
	// live values, not fixtures. Restore afterwards, as the smoke tour does.
	live, latency := core.DataModeLive, core.FailureLatency
	flipped, err := store.Update(core.ScenarioPatch{DataMode: &live, InjectFailure: &latency})
	if err != nil {
		t.Fatalf("patch scenario: %v", err)
	}
	payload = BuildEnvInspectorPayload(cfg, func(string) string { return "" }, store.Snapshot())
	if payload.Flags.DataMode != string(flipped.DataMode) || payload.Flags.InjectFailure != string(flipped.InjectFailure) {
		t.Errorf("panel after flip = %+v, want live/latency", payload.Flags)
	}
	fake, none := core.DataModeFake, core.FailureNone
	if _, err := store.Update(core.ScenarioPatch{DataMode: &fake, InjectFailure: &none}); err != nil {
		t.Fatalf("restore scenario: %v", err)
	}
	payload = BuildEnvInspectorPayload(cfg, func(string) string { return "" }, store.Snapshot())
	if payload.Flags.DataMode != string(core.DataModeFake) || payload.Flags.InjectFailure != string(core.FailureNone) {
		t.Errorf("panel after restore = %+v, want fake/none", payload.Flags)
	}
}

func TestBuildEnvInspectorScorecardWalksAllTwelveFactorsHonestly(t *testing.T) {
	t.Parallel()

	payload := BuildEnvInspectorPayload(core.Config{}, func(string) string { return "" },
		core.NewScenarioStore().Snapshot())

	if len(payload.TwelveFactor) != 12 {
		t.Fatalf("scorecard has %d rows, want all 12 factors", len(payload.TwelveFactor))
	}
	verdicts := map[string]int{}
	for i, row := range payload.TwelveFactor {
		if row.Factor == "" || row.How == "" {
			t.Errorf("factor row %d carries an empty name or how: %+v", i, row)
		}
		switch row.Verdict {
		case "hits", "differs":
			verdicts[row.Verdict]++
		default:
			t.Errorf("factor %q verdict %q is not hits/differs", row.Factor, row.Verdict)
		}
	}
	if verdicts["differs"] == 0 {
		t.Error("the walk must record its deliberate differences, not claim perfection")
	}
	if verdicts["hits"]+verdicts["differs"] != 12 {
		t.Errorf("verdicts counted %d of 12", verdicts["hits"]+verdicts["differs"])
	}
}

func TestBuildEnvInspectorCarriesAllSevenVariables(t *testing.T) {
	t.Parallel()

	payload := BuildEnvInspectorPayload(core.Config{}, func(string) string { return "" },
		core.NewScenarioStore().Snapshot())
	want := []string{
		"GDG_PORT", "GDG_ENV", "GDG_LOG_LEVEL", "GRPC_ADDR",
		"GDG_DATABASE_URL", "GDG_MONGO_URI", "GDG_REDIS_ADDR",
	}
	if len(payload.Variables) != len(want) {
		t.Fatalf("variables = %d rows, want %d", len(payload.Variables), len(want))
	}
	for i, envVar := range want {
		if payload.Variables[i].EnvVar != envVar {
			t.Errorf("row %d = %q, want %q", i, payload.Variables[i].EnvVar, envVar)
		}
		if payload.Variables[i].Field == "" || payload.Variables[i].ValidatedBy == "" {
			t.Errorf("row %q lacks its field name or validation note", envVar)
		}
	}
}
