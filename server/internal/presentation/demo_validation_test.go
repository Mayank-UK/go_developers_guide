package presentation

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
)

// The validation topic's demo endpoint, exercised through the same
// middleware chain production runs behind (the family rule that every
// new demo endpoint ships with a Go test). The route lives in a test mux
// here; main.go's production wiring is the composition root's call.

func newValidationArenaServer(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/validation/arena", NewValidationArenaHandler())
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestValidationArenaDemoServesThePayload(t *testing.T) {
	var buf strings.Builder
	server := newValidationArenaServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/validation/arena", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q, want application/json", ct)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	// Spot-check the beats the topic's prose promises; the payload
	// builder's own tests pin the deep shape and the determinism.
	decoded := struct {
		Subject struct {
			Name   string `json:"name"`
			Fields []struct {
				Name     string `json:"name"`
				Required bool   `json:"required"`
			} `json:"fields"`
		} `json:"subject"`
		Layers []struct {
			ID string `json:"id"`
		} `json:"layers"`
		Candidates []struct {
			ID     string `json:"id"`
			Decode struct {
				Status string `json:"status"`
			} `json:"decode"`
			Validate struct {
				Status string `json:"status"`
			} `json:"validate"`
			Response struct {
				Status    int              `json:"status"`
				Envelope  json.RawMessage  `json:"envelope"`
				Validated *json.RawMessage `json:"validated"`
			} `json:"response"`
		} `json:"candidates"`
		Envelope struct {
			CandidateID string `json:"candidateId"`
			Status      int    `json:"status"`
		} `json:"envelope"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.Subject.Name != "FeedbackForm" || len(decoded.Subject.Fields) != 4 {
		t.Errorf("subject drifted: %+v", decoded.Subject)
	}
	if len(decoded.Layers) != 3 || decoded.Layers[0].ID != "decode" || decoded.Layers[1].ID != "rules" {
		t.Errorf("layers drifted: %+v", decoded.Layers)
	}
	// The 422 round-trip beat: the exhibit candidate's envelope is the
	// real validation_failed shape, not a reconstruction.
	if decoded.Envelope.CandidateID != "collect-every-issue" || decoded.Envelope.Status != http.StatusUnprocessableEntity {
		t.Errorf("envelope exhibit drifted: %+v", decoded.Envelope)
	}
	for _, candidate := range decoded.Candidates {
		if candidate.Response.Status != 200 && candidate.Response.Status != 422 {
			t.Errorf("candidate %q status = %d, want 200 or 422", candidate.ID, candidate.Response.Status)
		}
		if candidate.Response.Status == 422 && len(candidate.Response.Envelope) == 0 {
			t.Errorf("candidate %q answers 422 without its envelope", candidate.ID)
		}
	}
}

func TestValidationArenaDemoRejectsNonGet(t *testing.T) {
	var buf strings.Builder
	server := newValidationArenaServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodPost, "/api/demo/validation/arena", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("405 body is not the envelope: %s", rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow header lost: %q", allow)
	}
}
