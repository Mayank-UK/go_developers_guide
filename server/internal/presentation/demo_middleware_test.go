package presentation

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
)

// The middleware topic's demo endpoint, exercised through the same
// middleware chain production runs behind (the family rule that every new
// demo endpoint ships with a Go test) — which is also the demo's own
// subject: the endpoint is served BY the very chain it teaches. Routes
// live in this test's own mux; the main.go wiring lands with the wave's
// shared wiring, not here.

// newMiddlewareDemoServer builds the demo route behind the production chain.
func newMiddlewareDemoServer() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/middleware/pipeline", NewMiddlewarePipelineHandler())
	var buf strings.Builder
	logger := newTestLogger(&buf)
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestMiddlewarePipelineDemoServesThePayload(t *testing.T) {
	server := newMiddlewareDemoServer()

	rec := doRequest(server, http.MethodGet, "/api/demo/middleware/pipeline", "")
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
		Chain []struct {
			Name     string `json:"name"`
			Position int    `json:"position"`
			Mirrors  string `json:"mirrors"`
		} `json:"chain"`
		Trace []struct {
			Layer    string `json:"layer"`
			Phase    string `json:"phase"`
			Sequence int    `json:"sequence"`
		} `json:"trace"`
		RequestID struct {
			Inbound          string `json:"inbound"`
			ResponseHeader   string `json:"responseHeader"`
			SameIDEverywhere bool   `json:"sameIDEverywhere"`
		} `json:"requestID"`
		PanicRecovery struct {
			Status        int    `json:"status"`
			RequestIDKept bool   `json:"requestIDKept"`
			LoggedLine    string `json:"loggedLine"`
		} `json:"panicRecovery"`
		TimeoutBeat struct {
			ObservedStatus      int  `json:"observedStatus"`
			SlowHandlerFinished bool `json:"slowHandlerFinished"`
		} `json:"timeoutBeat"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if len(decoded.Chain) != 4 || decoded.Chain[0].Name != "request-id" ||
		decoded.Chain[0].Mirrors != "presentation.RequestID" {
		t.Errorf("chain drifted: %+v", decoded.Chain)
	}
	if len(decoded.Trace) != 7 || decoded.Trace[0].Layer != "request-id" ||
		decoded.Trace[3].Phase != "handler" || decoded.Trace[6].Layer != "request-id" {
		t.Errorf("trace drifted: %+v", decoded.Trace)
	}
	if !decoded.RequestID.SameIDEverywhere || decoded.RequestID.ResponseHeader != decoded.RequestID.Inbound {
		t.Errorf("request-id beat drifted: %+v", decoded.RequestID)
	}
	if decoded.PanicRecovery.Status != 500 || !decoded.PanicRecovery.RequestIDKept ||
		!strings.Contains(decoded.PanicRecovery.LoggedLine, "panic recovered") {
		t.Errorf("panic beat drifted: %+v", decoded.PanicRecovery)
	}
	if decoded.TimeoutBeat.ObservedStatus != 503 || !decoded.TimeoutBeat.SlowHandlerFinished {
		t.Errorf("timeout beat drifted: %+v", decoded.TimeoutBeat)
	}
}

func TestMiddlewarePipelineDemoRejectsNonGet(t *testing.T) {
	server := newMiddlewareDemoServer()

	rec := doRequest(server, http.MethodPost, "/api/demo/middleware/pipeline", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("405 body is not the envelope: %s", rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow header lost: %q", allow)
	}
}
