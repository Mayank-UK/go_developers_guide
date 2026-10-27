package presentation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"guide/internal/core"
	"guide/internal/domain/demos"
	domi18n "guide/internal/domain/i18n"
)

// The topic 27 endpoint over a fake story runner — the REAL console
// story (embedded catalogs and all) has its own suite in
// internal/data/i18n. What is pinned here is the handler's thinness:
// the request's own Accept-Language is handed to the runner verbatim
// (and an absent header is reported as the fixture source), the story
// payload shape serves as-is from the builder, and a runner failure
// maps to the internal envelope with a request id.

// fakeI18nRunner captures what the handler handed it and answers a
// minimal-but-honest run.
type fakeI18nRunner struct {
	gotHeader string
	gotSource string
	err       error
}

func (f *fakeI18nRunner) RunNegotiatorStory(_ context.Context, header, headerSource string) (demos.I18nNegotiatorRun, error) {
	f.gotHeader, f.gotSource = header, headerSource
	if f.err != nil {
		return demos.I18nNegotiatorRun{}, f.err
	}
	return demos.I18nNegotiatorRun{
		Header:       header,
		HeaderSource: headerSource,
		Candidates:   []demos.I18nCandidateRow{{Range: "de-at", Quality: 1}, {Range: "en", Quality: 0.8}},
		Selected:     "de",
		Via:          domi18n.ViaBase,
		Chain:        []demos.I18nChainRow{{Range: "de-at", Quality: 1, Outcome: domi18n.ViaBase, Matched: "de"}},
		Greetings: []demos.I18nMessageRow{
			{Tag: "en", Text: "Welcome back, Ada!", Served: "en", Found: true},
			{Tag: "de", Text: "Willkommen zurück, Ada!", Served: "de", Found: true},
		},
		MissingKey: demos.I18nMissingKeyRow{Key: "demo.never_translated", Text: "demo.never_translated", LogLine: "logged"},
	}, nil
}

func newI18nDemoServer(t *testing.T, runner *fakeI18nRunner) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/i18n/negotiator", NewI18nNegotiatorHandler(runner, newTestLogger(&strings.Builder{})))
	return Chain(RequestID, RequestLogger(newTestLogger(&strings.Builder{})), Recover(newTestLogger(&strings.Builder{})), EnvelopeMuxErrors(newTestLogger(&strings.Builder{})))(mux)
}

func doI18nRequest(server http.Handler, acceptLanguage string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/demo/i18n/negotiator", nil)
	if acceptLanguage != "" {
		req.Header.Set("Accept-Language", acceptLanguage)
	}
	server.ServeHTTP(rec, req)
	return rec
}

func TestI18nNegotiatorHandsTheRequestOwnHeaderToTheRunner(t *testing.T) {
	runner := &fakeI18nRunner{}
	server := newI18nDemoServer(t, runner)

	rec := doI18nRequest(server, "de-AT, en;q=0.8")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if runner.gotHeader != "de-AT, en;q=0.8" {
		t.Errorf("runner header = %q, want the request's own verbatim", runner.gotHeader)
	}
	if runner.gotSource != "request" {
		t.Errorf("runner source = %q, want request", runner.gotSource)
	}
}

func TestI18nNegotiatorReportsTheFixtureWhenNoHeaderCame(t *testing.T) {
	runner := &fakeI18nRunner{}
	server := newI18nDemoServer(t, runner)

	rec := doI18nRequest(server, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if runner.gotHeader != "" {
		t.Errorf("runner header = %q, want empty (the console substitutes)", runner.gotHeader)
	}
	if runner.gotSource != "fixture" {
		t.Errorf("runner source = %q, want fixture", runner.gotSource)
	}
}

func TestI18nNegotiatorServesTheStoryShape(t *testing.T) {
	server := newI18nDemoServer(t, &fakeI18nRunner{})
	rec := doI18nRequest(server, "de")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Mechanism string `json:"mechanism"`
		Beats     []struct {
			Beat    string `json:"beat"`
			Verdict string `json:"verdict"`
			Facts   []any  `json:"facts"`
		} `json:"beats"`
		Proofs    []any  `json:"proofs"`
		Reasoning string `json:"reasoning"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("story shape: %v", err)
	}
	if payload.Mechanism != "i18n-negotiator" || len(payload.Beats) != 7 {
		t.Errorf("mechanism = %q, beats = %d", payload.Mechanism, len(payload.Beats))
	}
	if payload.Beats[0].Facts == nil || len(payload.Proofs) == 0 || payload.Reasoning == "" {
		t.Error("payload must carry non-nil facts, proofs, and the reasoning paragraph")
	}
}

func TestI18nNegotiatorMapsRunnerFailureToInternal(t *testing.T) {
	server := newI18nDemoServer(t, &fakeI18nRunner{err: errors.New("boom")})
	rec := doI18nRequest(server, "de")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != string(core.KindInternal) || envelope.Error.RequestID == "" {
		t.Errorf("envelope = %+v", envelope.Error)
	}
}
