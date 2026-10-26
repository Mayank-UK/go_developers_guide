package presentation

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
)

// The webhooks topic's demo endpoint, exercised through the same
// middleware chain production runs behind (the family rule that every new
// demo endpoint ships with a Go test). The route lives in a test mux
// here; main.go's production wiring is the composition root's call.

func newWebhookSimulatorServer(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/api/webhook-simulator", NewWebhookSimulatorHandler())
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestWebhookSimulatorDemoServesThePayload(t *testing.T) {
	var buf strings.Builder
	server := newWebhookSimulatorServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/api/webhook-simulator", "")
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
		Config struct {
			Algorithm        string `json:"algorithm"`
			SignatureHeader  string `json:"signatureHeader"`
			ToleranceSeconds int    `json:"toleranceSeconds"`
			MaxAttempts      int    `json:"maxAttempts"`
		} `json:"config"`
		Signing struct {
			Body   string `json:"body"`
			Issued struct {
				Timestamp string `json:"timestamp"`
				Signature string `json:"signature"`
			} `json:"issued"`
			Cases []struct {
				ID       string `json:"id"`
				Verified bool   `json:"verified"`
				Reason   string `json:"reason"`
			} `json:"cases"`
		} `json:"signing"`
		Deliveries []struct {
			DeliveryID string `json:"deliveryId"`
			Outcome    string `json:"outcome"`
			DeadLetter *struct {
				Queue      string `json:"queue"`
				Replayable bool   `json:"replayable"`
			} `json:"deadLetter"`
		} `json:"deliveries"`
		Receiver struct {
			AnsweredStatus int `json:"answeredStatus"`
		} `json:"receiver"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if decoded.Config.Algorithm != "HMAC-SHA256" || decoded.Config.SignatureHeader != "X-Guide-Signature" {
		t.Errorf("config drifted: %+v", decoded.Config)
	}
	if decoded.Config.ToleranceSeconds != 300 || decoded.Config.MaxAttempts != 5 {
		t.Errorf("knobs drifted: %+v", decoded.Config)
	}
	// The four verdicts: one verified, three rejections with reasons.
	if len(decoded.Signing.Cases) != 4 || !decoded.Signing.Cases[0].Verified {
		t.Errorf("signature cases drifted: %+v", decoded.Signing.Cases)
	}
	for _, rejected := range decoded.Signing.Cases[1:] {
		if rejected.Verified || rejected.Reason == "" {
			t.Errorf("case %q must be rejected with a reason", rejected.ID)
		}
	}
	// The two ledgers: recovery on attempt 3, dead-letter after 5.
	if len(decoded.Deliveries) != 2 {
		t.Fatalf("deliveries = %d, want 2", len(decoded.Deliveries))
	}
	if decoded.Deliveries[0].Outcome != "delivered" || decoded.Deliveries[0].DeadLetter != nil {
		t.Errorf("first ledger = %+v, want delivered with no DLQ record", decoded.Deliveries[0])
	}
	if decoded.Deliveries[1].Outcome != "dead-lettered" || decoded.Deliveries[1].DeadLetter == nil ||
		!decoded.Deliveries[1].DeadLetter.Replayable {
		t.Errorf("second ledger = %+v, want dead-lettered and replayable", decoded.Deliveries[1])
	}
	if decoded.Receiver.AnsweredStatus != http.StatusOK {
		t.Errorf("receiver answered %d, want the immediate 200", decoded.Receiver.AnsweredStatus)
	}
}

func TestWebhookSimulatorDemoRejectsNonGet(t *testing.T) {
	var buf strings.Builder
	server := newWebhookSimulatorServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodPost, "/api/demo/api/webhook-simulator", "")
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
