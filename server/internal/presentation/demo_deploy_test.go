package presentation

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The deployment stack endpoint through the production middleware
// chain. The tests pin the demo's honesty legs: the payload served
// from the REAL artifacts carries per-artifact provenance and the
// probe contract (kubelet GETs the real /api/health and /api/ready),
// the absent-repo case degrades to the typed 503 (never a guess, never
// a 500), a broken artifact set is an honest internal error, and the
// method contract holds.

// newDeployStackServer builds the endpoint through the production
// chain over the given artifacts directory.
func newDeployStackServer(artifactsDir string) http.Handler {
	logger := newTestLogger(&strings.Builder{})
	mux := http.NewServeMux()
	mux.Handle("GET "+demos.DeployStackPath, NewDeployStackHandler(artifactsDir, logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestDeployStackServesTheRealArtifacts(t *testing.T) {
	dir := demos.LocateDeployDir(".")
	if dir == "" {
		t.Fatal("the repo's deploy/ artifacts must be locatable from the test working directory")
	}
	server := newDeployStackServer(dir)

	rec := doRequest(server, http.MethodGet, demos.DeployStackPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	var payload struct {
		Mode    string `json:"mode"`
		Compose struct {
			Source   string `json:"source"`
			Project  string `json:"project"`
			Services []struct {
				Name    string   `json:"name"`
				Image   string   `json:"image"`
				Depends []string `json:"dependsOn"`
			} `json:"services"`
		} `json:"compose"`
		Kubernetes []struct {
			File  string `json:"file"`
			Kind  string `json:"kind"`
			Facts []struct {
				Label string `json:"label"`
				Value string `json:"value"`
			} `json:"facts"`
		} `json:"kubernetes"`
		SecretScan struct {
			Status  string   `json:"status"`
			Checked []string `json:"checked"`
		} `json:"secretScan"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("payload not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if payload.Compose.Source != "deploy/docker-compose.prod.yml" {
		t.Errorf("compose provenance = %q", payload.Compose.Source)
	}
	if len(payload.Compose.Services) != 5 {
		t.Errorf("compose services = %d, want the five stack members", len(payload.Compose.Services))
	}
	deploymentFacts := ""
	for _, manifest := range payload.Kubernetes {
		if !strings.HasSuffix(manifest.File, "deployment.json") {
			continue
		}
		for _, fact := range manifest.Facts {
			deploymentFacts += fact.Value + "\n"
		}
	}
	for _, want := range []string{"/api/health", "/api/ready"} {
		if !strings.Contains(deploymentFacts, want) {
			t.Errorf("deployment facts must name the real probe endpoint %q", want)
		}
	}
	if payload.SecretScan.Status != "clean" {
		t.Errorf("secret scan = %q, want clean", payload.SecretScan.Status)
	}
}

func TestDeployStackDegradesHonestlyWithoutTheRepo(t *testing.T) {
	for name, dir := range map[string]string{
		"unresolved directory":        "",
		"directory without artifacts": t.TempDir(),
	} {
		server := newDeployStackServer(dir)
		rec := doRequest(server, http.MethodGet, demos.DeployStackPath, "")
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: status = %d, want 503: %s", name, rec.Code, rec.Body.String())
			continue
		}
		var envelope struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("%s: envelope decode: %v", name, err)
		}
		if envelope.Error.Code != string(core.KindServiceUnavailable) {
			t.Errorf("%s: code = %q, want service_unavailable", name, envelope.Error.Code)
		}
	}
}

func TestDeployStackRefusesABrokenArtifactSet(t *testing.T) {
	// A directory holding artifacts whose secret is NOT a placeholder:
	// the builder refuses to serve, and the refusal is an honest
	// internal error naming the slot — never a payload with the
	// credential on it.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, demos.DeployK8sDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, demos.DeployComposeFile), []byte("name: broken\nservices: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	leaked := "ewogICJhcGlWZXJzaW9uIjogInYxIiwKICAia2luZCI6ICJTZWNyZXQiLAogICJtZXRhZGF0YSI6IHsibmFtZSI6ICJndWlkZS1zZWNyZXQifSwKICAiZGF0YSI6IHsiR0RHX0RBVEFCQVNFX1VSTCI6ICJwbGFpbnRleHQtY3JlZGVudGlhbCJ9Cn0K"
	if err := os.WriteFile(filepath.Join(dir, demos.DeployK8sDir, "secret.json"), []byte(leaked), 0o644); err != nil {
		t.Fatal(err)
	}
	server := newDeployStackServer(dir)
	rec := doRequest(server, http.MethodGet, demos.DeployStackPath, "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 — the builder's refusal must surface as an internal error: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "plaintext-credential") {
		t.Error("the refused value itself must not ride the error envelope to the wire")
	}
}

func TestDeployStackRejectsNonGet(t *testing.T) {
	dir := demos.LocateDeployDir(".")
	if dir == "" {
		t.Fatal("deploy/ artifacts must be locatable")
	}
	server := newDeployStackServer(dir)
	rec := doRequest(server, http.MethodPost, demos.DeployStackPath, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rec.Code)
	}
}
