package demos

import (
	"reflect"
	"strings"
	"testing"
)

// The middleware-pipeline demo's payload is computed live from a real
// in-process run of the exhibit chain; these tests pin the facts the
// middleware topic's prose claims, so a change to the exhibit that breaks
// the choreography fails HERE first, next to the claim.

func TestMiddlewarePipelineChainMirrorsTheProductionOrder(t *testing.T) {
	chain := BuildMiddlewarePipelinePayload().Chain
	if len(chain) != 4 {
		t.Fatalf("chain = %d layers, want 4", len(chain))
	}
	wantNames := []string{"request-id", "request-logger", "recover", "terminal handler"}
	for i, layer := range chain {
		if layer.Position != i || layer.Name != wantNames[i] {
			t.Errorf("layer %d = %+v, want position %d name %q", i, layer, i, wantNames[i])
		}
		if layer.Role == "" || layer.Mirrors == "" {
			t.Errorf("layer %q needs both a role and the production layer it mirrors", layer.Name)
		}
	}
	if chain[0].Mirrors != "presentation.RequestID" || chain[2].Mirrors != "presentation.Recover" {
		t.Errorf("chain must mirror the real wiring order: %+v", chain)
	}
}

func TestMiddlewarePipelineTraceIsTheOnion(t *testing.T) {
	events := BuildMiddlewarePipelinePayload().Trace
	if len(events) != 7 {
		t.Fatalf("trace = %d events, want 7 (3 before + 1 handler + 3 after)", len(events))
	}
	type beat struct{ layer, phase string }
	want := []beat{
		{"request-id", phaseBefore},
		{"request-logger", phaseBefore},
		{"recover", phaseBefore},
		{"terminal handler", phaseHandler},
		{"recover", phaseAfter},
		{"request-logger", phaseAfter},
		{"request-id", phaseAfter},
	}
	for i, event := range events {
		if event.Sequence != i {
			t.Errorf("event %d carries sequence %d — append order IS the sequence", i, event.Sequence)
		}
		if event.Layer != want[i].layer || event.Phase != want[i].phase {
			t.Errorf("event %d = %s/%s, want %s/%s", i, event.Layer, event.Phase, want[i].layer, want[i].phase)
		}
		if event.Note == "" {
			t.Errorf("event %d has no teaching note", i)
		}
	}
	// The first before-beat is the one place the id is legitimately absent:
	// request-id has not assigned it yet. Empty is DATA there.
	if events[0].RequestIDSeen != "" {
		t.Errorf("request-id's before-beat must see no id yet, got %q", events[0].RequestIDSeen)
	}
}

func TestMiddlewarePipelineRequestIDPropagatesAcrossLayers(t *testing.T) {
	beat := BuildMiddlewarePipelinePayload().RequestID
	if beat.HeaderName != "x-request-id" || beat.Inbound != pipelineDemoRequestID {
		t.Errorf("request-id beat drifted: %+v", beat)
	}
	if beat.ResponseHeader != pipelineDemoRequestID || !beat.SameIDEverywhere {
		t.Errorf("the id must survive every layer and the response header: %+v", beat)
	}
	// Every layer past request-id observed it: 6 of the 7 trace beats.
	if len(beat.SeenAtLayers) != 6 {
		t.Errorf("seen-at-layers = %v, want the 6 beats after request-id's before-beat", beat.SeenAtLayers)
	}
	for _, seen := range beat.SeenAtLayers {
		if strings.TrimSpace(seen) == "" {
			t.Errorf("seen-at-layers entry %q is malformed", seen)
		}
	}
}

func TestMiddlewarePipelineRequestSideAndResponseSideOrdersMirror(t *testing.T) {
	payload := BuildMiddlewarePipelinePayload()
	if strings.Join(payload.RequestSideOrder, ",") != "request-id,request-logger,recover,terminal handler" {
		t.Errorf("request side = %v", payload.RequestSideOrder)
	}
	if strings.Join(payload.ResponseSideOrder, ",") != "terminal handler,recover,request-logger,request-id" {
		t.Errorf("response side = %v", payload.ResponseSideOrder)
	}
}

func TestMiddlewarePipelineRecoversTheDeliberatePanic(t *testing.T) {
	beat := BuildMiddlewarePipelinePayload().PanicRecovery
	if beat.Status != 500 {
		t.Errorf("panic status = %d, want 500", beat.Status)
	}
	if !strings.Contains(beat.Body, `"code":"internal"`) ||
		!strings.Contains(beat.Body, `"requestId":"`+pipelineDemoRequestID+`"`) {
		t.Errorf("panic body is not the internal envelope with the request id: %s", beat.Body)
	}
	if !strings.Contains(beat.LoggedLine, `msg="panic recovered"`) ||
		!strings.Contains(beat.LoggedLine, "request_id="+pipelineDemoRequestID) ||
		!strings.Contains(beat.LoggedLine, "assignment to entry in nil map") {
		t.Errorf("panic log line drifted: %q", beat.LoggedLine)
	}
	if strings.Contains(beat.LoggedLine, "time=") {
		t.Errorf("captured log line must have its time= field removed: %q", beat.LoggedLine)
	}
	if !beat.RequestIDKept {
		t.Error("the 500 response must still carry the request-id header stamped before the stack ran")
	}
	// The panic run's unwind order: recover answers first, logger and
	// request-id still get their after-beats.
	if len(beat.Trace) != 7 {
		t.Fatalf("panic trace = %d events, want 7", len(beat.Trace))
	}
	if beat.Trace[4].Layer != "recover" || beat.Trace[4].Phase != phaseAfter ||
		!strings.Contains(beat.Trace[4].Note, "converted") {
		t.Errorf("panic trace's recover after-beat drifted: %+v", beat.Trace[4])
	}
	if beat.Trace[6].Layer != "request-id" || beat.Trace[6].Phase != phaseAfter {
		t.Errorf("request-id must still run its after-beat after a recovered panic: %+v", beat.Trace[6])
	}
}

func TestMiddlewarePipelineTimeoutBeatFiresAndJoins(t *testing.T) {
	beat := BuildMiddlewarePipelinePayload().TimeoutBeat
	if beat.Wrapper != "http.TimeoutHandler" {
		t.Errorf("wrapper = %q", beat.Wrapper)
	}
	if beat.BudgetMs != 30 || beat.HandlerSleepMs != 120 {
		t.Errorf("budget/sleep arithmetic = %d/%d, want 30/120", beat.BudgetMs, beat.HandlerSleepMs)
	}
	if beat.ObservedStatus != 503 {
		t.Errorf("timeout status = %d, want the wrapper's own 503", beat.ObservedStatus)
	}
	if !strings.Contains(beat.Body, "Timeout") {
		t.Errorf("timeout body is not the wrapper's canned page: %q", beat.Body)
	}
	if !beat.SlowHandlerFinished {
		t.Error("the abandoned slow handler must finish within the bounded join — no goroutine leak")
	}
	if beat.Shape == "" {
		t.Error("the timeout beat must describe its shape in prose")
	}
}

func TestMiddlewarePipelinePayloadIsPure(t *testing.T) {
	first, second := BuildMiddlewarePipelinePayload(), BuildMiddlewarePipelinePayload()
	if !reflect.DeepEqual(first, second) {
		t.Error("builder must be pure: two calls returned different payloads")
	}
}
