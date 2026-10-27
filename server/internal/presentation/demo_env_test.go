package presentation

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The env inspector endpoint through the production middleware chain.
// The tests pin the three honesty legs the topic teaches — redaction on
// the WIRE (a credential-bearing DSN must not survive the served
// bytes), attribution truthfulness (a var present in the injected
// environment reads "environment", absent reads "default"), and flag
// liveness (a scenario PATCH is reflected by the next GET, then
// restored) — plus the 405 contract.

// newEnvInspectorServer builds the inspector through the production
// chain over an injectable environment, so tests never mutate the
// process environment.
func newEnvInspectorServer(getenv func(string) string, store *core.ScenarioStore) http.Handler {
	logger := newTestLogger(&strings.Builder{})
	handler := NewEnvInspectorHandler(core.Config{
		Port:        8100,
		Env:         "test",
		DatabaseURL: "postgres://postgres:wiring-secret@postgres:5432/guide?sslmode=disable",
		MongoURI:    "mongodb://mongo:27017",
		RedisAddr:   "redis:6379",
	}, store, logger)
	handler.getenv = getenv
	mux := http.NewServeMux()
	mux.Handle("GET "+demos.EnvInspectorPath, handler)
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestEnvInspectorServesTheDerivedConfigHonestly(t *testing.T) {
	store := core.NewScenarioStore()
	server := newEnvInspectorServer(func(key string) string {
		if key == "GDG_PORT" {
			return "9001"
		}
		if key == "GDG_DATABASE_URL" {
			return "postgres://postgres:test-credential@db:5432/guide"
		}
		return ""
	}, store)

	rec := doRequest(server, http.MethodGet, demos.EnvInspectorPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	var payload struct {
		Mode      string `json:"mode"`
		Env       string `json:"env"`
		Variables []struct {
			Field       string `json:"field"`
			EnvVar      string `json:"envVar"`
			Value       string `json:"value"`
			Source      string `json:"source"`
			Sensitive   bool   `json:"sensitive"`
			ValidatedBy string `json:"validatedBy"`
		} `json:"variables"`
		TwelveFactor []struct {
			Factor  string `json:"factor"`
			Verdict string `json:"verdict"`
			How     string `json:"how"`
		} `json:"twelveFactor"`
		Flags struct {
			DataMode      string `json:"dataMode"`
			InjectFailure string `json:"injectFailure"`
		} `json:"flags"`
		Redaction string `json:"redaction"`
		Reasoning string `json:"reasoning"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("payload not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}

	if payload.Env != "test" {
		t.Errorf("env = %q, want the cfg's resolved env", payload.Env)
	}
	sources := make(map[string]string)
	values := make(map[string]string)
	for _, row := range payload.Variables {
		sources[row.EnvVar] = row.Source
		values[row.EnvVar] = row.Value
	}
	if sources["GDG_PORT"] != "environment" || values["GDG_PORT"] != "8100" {
		t.Errorf("GDG_PORT = %q/%q — attribution says environment, value stays what the process BOOTED with (cfg, not the live env)", sources["GDG_PORT"], values["GDG_PORT"])
	}
	if sources["GDG_DATABASE_URL"] != "environment" {
		t.Errorf("GDG_DATABASE_URL source = %q, want environment", sources["GDG_DATABASE_URL"])
	}
	for _, envVar := range []string{"GDG_ENV", "GDG_LOG_LEVEL", "GRPC_ADDR", "GDG_MONGO_URI", "GDG_REDIS_ADDR"} {
		if sources[envVar] != "default" {
			t.Errorf("%s source = %q, want default", envVar, sources[envVar])
		}
	}
	if len(payload.TwelveFactor) != 12 {
		t.Errorf("scorecard rows = %d, want 12", len(payload.TwelveFactor))
	}
	if payload.Flags.DataMode != "fake" || payload.Flags.InjectFailure != "none" {
		t.Errorf("flags = %+v, want the store's defaults", payload.Flags)
	}

	// The redaction proof, on the wire: the handler was wired with a
	// DSN carrying a password, and neither the password nor the query
	// may appear anywhere in the served bytes.
	body := rec.Body.String()
	for _, leaked := range []string{"wiring-secret", "sslmode=disable"} {
		if strings.Contains(body, leaked) {
			t.Errorf("served payload leaks %q: %s", leaked, body)
		}
	}
	if !strings.Contains(body, "postgres://postgres@postgres:5432/guide") {
		t.Errorf("DatabaseURL row must show the redacted DSN: %s", body)
	}
	if !strings.Contains(payload.Redaction, "never echoed") {
		t.Errorf("redaction policy must be stated on the payload: %q", payload.Redaction)
	}
}

func TestEnvInspectorFlagPanelFollowsALivePatch(t *testing.T) {
	store := core.NewScenarioStore()
	server := newEnvInspectorServer(func(string) string { return "" }, store)

	flagsAt := func() (string, string) {
		t.Helper()
		rec := doRequest(server, http.MethodGet, demos.EnvInspectorPath, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var payload struct {
			Flags struct {
				DataMode      string `json:"dataMode"`
				InjectFailure string `json:"injectFailure"`
			} `json:"flags"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return payload.Flags.DataMode, payload.Flags.InjectFailure
	}

	if mode, failure := flagsAt(); mode != "fake" || failure != "none" {
		t.Fatalf("panel starts at %s/%s, want fake/none", mode, failure)
	}
	live, latency := core.DataModeLive, core.FailureLatency
	if _, err := store.Update(core.ScenarioPatch{DataMode: &live, InjectFailure: &latency}); err != nil {
		t.Fatalf("patch scenario: %v", err)
	}
	if mode, failure := flagsAt(); mode != "live" || failure != "latency" {
		t.Errorf("panel after PATCH = %s/%s, want live/latency — the panel must reflect the CURRENT store", mode, failure)
	}
	fake, none := core.DataModeFake, core.FailureNone
	if _, err := store.Update(core.ScenarioPatch{DataMode: &fake, InjectFailure: &none}); err != nil {
		t.Fatalf("restore scenario: %v", err)
	}
	if mode, failure := flagsAt(); mode != "fake" || failure != "none" {
		t.Errorf("panel after restore = %s/%s, want fake/none", mode, failure)
	}
}

func TestEnvInspectorRejectsNonGet(t *testing.T) {
	server := newEnvInspectorServer(func(string) string { return "" }, core.NewScenarioStore())
	rec := doRequest(server, http.MethodPost, demos.EnvInspectorPath, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rec.Code)
	}
}
