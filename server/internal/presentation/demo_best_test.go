package presentation

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"guide/internal/core"
	postsdata "guide/internal/data/posts"
	"guide/internal/domain/demos"
	"guide/internal/domain/posts"
)

// The best-practices checklist endpoint through the production
// middleware chain. The handler under test is the REAL one: its
// private probe mux mounts the REAL health, scenario, and posts
// handlers behind the REAL chain, so the live rows are judged from
// requests that actually executed. The module directory is empty in
// these tests (the source tree is not the point here), so the exec and
// file rows degrade to their honest fails — itself part of what the
// suite pins.

// newBestChecklistHandler builds the real handler over the in-memory
// posts substrate, mirroring the composition root's wiring.
func newBestChecklistHandler(t *testing.T) *BestChecklistHandler {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	repo := postsdata.NewInMemoryRepository()
	return NewBestChecklistHandler(
		core.Config{Port: 8100, GRPCAddr: ":8102", Env: "test"},
		core.NewScenarioStore(),
		posts.NewGetPostUseCase(repo),
		posts.NewCreatePostUseCase(repo),
		"",
		logger,
	)
}

// waitBestChecklistPanel starts the audit and polls until the snapshot
// exists (bounded — the probes are in-process and instant; a real tool
// run never happens with an empty module dir).
func waitBestChecklistPanel(t *testing.T, handler *BestChecklistHandler) demos.BestChecklistPanel {
	t.Helper()
	handler.Start()
	deadline := time.Now().Add(5 * time.Second)
	for {
		handler.mu.Lock()
		panel := handler.panel
		handler.mu.Unlock()
		if panel != nil {
			return *panel
		}
		if time.Now().After(deadline) {
			t.Fatal("the boot audit never completed inside its bound")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// bestChecklistDecode fetches and decodes the panel through the chain.
func bestChecklistDecode(t *testing.T, server http.Handler) map[string]any {
	t.Helper()
	rec := doRequest(server, http.MethodGet, demos.BestChecklistPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 even when checks degrade: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("payload not JSON: %v\n%s", err, rec.Body.String())
	}
	return payload
}

// bestChecklistRows flattens the panel's rows by id.
func bestChecklistRows(payload map[string]any) map[string]map[string]any {
	rows := make(map[string]map[string]any)
	for _, area := range payload["areas"].([]any) {
		for _, item := range area.(map[string]any)["items"].([]any) {
			row := item.(map[string]any)
			rows[row["id"].(string)] = row
		}
	}
	return rows
}

func TestBestChecklistDemoServesTheBootAudit(t *testing.T) {
	handler := newBestChecklistHandler(t)
	server := bestChecklistRouteOnly(handler)

	panel := waitBestChecklistPanel(t, handler)
	if panel.Summary.Failed == 0 || panel.Summary.Manual != 3 {
		t.Errorf("summary = %+v, want the exec/file degradations counted honestly", panel.Summary)
	}

	payload := bestChecklistDecode(t, server)
	rows := bestChecklistRows(payload)

	// The live rows all pass: real handlers, real chain, real recorders.
	for _, id := range []string{
		"health-contract",
		"error-envelope-on-miss",
		"boundary-validation",
		"scenario-contract",
		"one-config-source",
	} {
		row := rows[id]
		if row["verdict"] != "pass" || row["checkKind"] != "live" {
			t.Errorf("live row %q = %v, want a real pass", id, row)
		}
		if len(row["evidence"].([]any)) == 0 {
			t.Errorf("live row %q carries no evidence", id)
		}
	}

	// The exec rows degrade honestly: no source tree, no invented green.
	for _, id := range []string{"gofmt-clean", "ratchet-green", "race-stance"} {
		row := rows[id]
		if row["verdict"] != "fail" {
			t.Errorf("exec row %q = %v, want an honest fail without the tree", id, row)
		}
	}
	if rows["stack-as-files"]["verdict"] != "manual" {
		t.Error("the compose row must degrade to manual when the repo root is not visible")
	}

	summary := payload["summary"].(map[string]any)
	if summary["items"].(float64) != 15 {
		t.Errorf("items = %v, want 15 rows", summary["items"])
	}
	if len(payload["notDones"].([]any)) != 4 {
		t.Error("the anti-checklist rides the payload")
	}
}

// TestBestChecklistEveryMachineRowCarriesRealEvidence is the endpoint
// honesty pin: on the served wire, every machine verdict carries
// non-empty evidence and every manual row carries its why.
func TestBestChecklistEveryMachineRowCarriesRealEvidence(t *testing.T) {
	handler := newBestChecklistHandler(t)
	waitBestChecklistPanel(t, handler)
	payload := bestChecklistDecode(t, bestChecklistRouteOnly(handler))

	for _, area := range payload["areas"].([]any) {
		for _, item := range area.(map[string]any)["items"].([]any) {
			row := item.(map[string]any)
			if row["checkKind"] == "manual" {
				if row["verdict"] != "manual" || row["whyManual"] == nil || row["whyManual"].(string) == "" {
					t.Errorf("manual row %v lacks its why", row)
				}
				continue
			}
			verdict := row["verdict"].(string)
			if verdict != "pass" && verdict != "fail" {
				t.Errorf("row %v verdict %q is neither pass nor fail", row["id"], verdict)
			}
			evidence, _ := row["evidence"].([]any)
			if len(evidence) == 0 {
				t.Errorf("row %v carries a %q verdict with no evidence", row["id"], verdict)
			}
		}
	}
}

// TestBestChecklistServesIdenticalBodiesOnceCached pins the boot-cache
// contract on the wire: after the snapshot exists, two GETs replay
// byte-identical bodies — the audit is a fact about boot, not about
// requests.
func TestBestChecklistServesIdenticalBodiesOnceCached(t *testing.T) {
	handler := newBestChecklistHandler(t)
	waitBestChecklistPanel(t, handler)
	server := bestChecklistRouteOnly(handler)

	first := doRequest(server, http.MethodGet, demos.BestChecklistPath, "")
	second := doRequest(server, http.MethodGet, demos.BestChecklistPath, "")
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("status = %d/%d, want 200/200", first.Code, second.Code)
	}
	if first.Body.String() != second.Body.String() {
		t.Error("two GETs after the boot audit must serve byte-identical panels")
	}
}

// TestBestChecklistServesAnHonestPlaceholderWhileComputing pins the
// pre-snapshot window: shape-valid JSON with empty (never null)
// collections, so the client renders a panel, not a broken screen.
func TestBestChecklistServesAnHonestPlaceholderWhileComputing(t *testing.T) {
	handler := newBestChecklistHandler(t)
	// Deliberately no Start: the first request triggers the audit and
	// must observe the placeholder, never a hang or a 500.
	server := bestChecklistRouteOnly(handler)
	rec := doRequest(server, http.MethodGet, demos.BestChecklistPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 while computing", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("placeholder not JSON: %v", err)
	}
	if _, ok := payload["areas"].([]any); !ok {
		t.Errorf("placeholder areas = %T, want a JSON array (never null)", payload["areas"])
	}
	if _, ok := payload["notDones"].([]any); !ok {
		t.Errorf("placeholder notDoses = %T, want a JSON array (never null)", payload["notDones"])
	}
}

// bestChecklistRouteOnly mounts the handler behind the production
// middleware order — the same chain the composition root serves.
func bestChecklistRouteOnly(handler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET "+demos.BestChecklistPath, handler)
	logger := newTestLogger(&strings.Builder{})
	return Chain(RequestID, RequestLogger(logger), Recover(logger), EnvelopeMuxErrors(logger))(mux)
}
