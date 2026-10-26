package demos

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The methods report is OBSERVED, not claimed: every row the payload
// carries is the exhibit's own behavior read back off a recorder, and
// these tests re-verify each claim independently — a wrong status in the
// prose or the payload is a factual bug, and it fails here first.

func TestReqresMethodsExhibitAnswersEveryMethodTruthfully(t *testing.T) {
	// The claims the topic's prose makes: method → status → the headers
	// that define the lesson. Driven straight at the exhibit handler.
	claims := []struct {
		method       string
		status       int
		wantHeaders  map[string]string
		wantBodyRule func(string) bool
	}{
		{http.MethodGet, http.StatusOK,
			map[string]string{"Content-Type": "application/json", "Cache-Control": "no-store", "Set-Cookie": "reqres_visit=1; Path=/api/demo/reqres; HttpOnly; SameSite=Lax"},
			func(body string) bool { return body == `{"resource":"methods-exhibit","state":"steady"}` }},
		{http.MethodPost, http.StatusCreated,
			map[string]string{"Location": ReqresExhibitPath + "/17", "Content-Type": "application/json"},
			func(body string) bool { return body == `{"id":17,"state":"created"}` }},
		{http.MethodPut, http.StatusOK,
			map[string]string{"Content-Type": "application/json"},
			func(body string) bool { return body == `{"id":17,"state":"replaced"}` }},
		{http.MethodPatch, http.StatusNoContent, nil,
			func(body string) bool { return body == "" }},
		{http.MethodDelete, http.StatusAccepted,
			map[string]string{"Content-Type": "application/json"},
			func(body string) bool { return body == `{"id":17,"state":"deletion scheduled"}` }},
		{http.MethodHead, http.StatusOK,
			map[string]string{"Content-Type": "application/json", "Set-Cookie": "reqres_visit=1; Path=/api/demo/reqres; HttpOnly; SameSite=Lax"},
			func(body string) bool { return body == "" }},
		{http.MethodOptions, http.StatusNoContent,
			map[string]string{"Allow": ReqresAllowValue},
			func(body string) bool { return body == "" }},
		{"TRACE", http.StatusMethodNotAllowed,
			map[string]string{"Allow": ReqresAllowValue},
			func(body string) bool { return strings.Contains(body, `"error":"TRACE"`) }},
	}
	for _, claim := range claims {
		recorder := httptest.NewRecorder()
		ServeReqresExhibit(recorder, httptest.NewRequest(claim.method, ReqresExhibitPath, nil))
		if recorder.Code != claim.status {
			t.Errorf("%s status = %d, want %d", claim.method, recorder.Code, claim.status)
		}
		for name, want := range claim.wantHeaders {
			if got := recorder.Header().Get(name); got != want {
				t.Errorf("%s header %s = %q, want %q", claim.method, name, got, want)
			}
		}
		if !claim.wantBodyRule(recorder.Body.String()) {
			t.Errorf("%s body = %q, want the claimed shape", claim.method, recorder.Body.String())
		}
	}
}

func TestReqresMethodsPayloadRowsMatchTheExhibit(t *testing.T) {
	payload := BuildReqresMethodsPayload()
	if len(payload.Methods) != 8 {
		t.Fatalf("methods = %d rows, want 8 (seven served + the truthful 405)", len(payload.Methods))
	}
	if payload.Allow != "GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE" {
		t.Errorf("allow = %q", payload.Allow)
	}

	// Independent re-verification: each payload row against a fresh run.
	for _, row := range payload.Methods {
		recorder := httptest.NewRecorder()
		ServeReqresExhibit(recorder, httptest.NewRequest(row.Method, ReqresExhibitPath, nil))
		if row.Status != recorder.Code || row.Body != recorder.Body.String() {
			t.Errorf("row %s drifted from the exhibit: row=%d/%q exhibit=%d/%q",
				row.Method, row.Status, row.Body, recorder.Code, recorder.Body.String())
		}
		if row.StatusText != http.StatusText(recorder.Code) {
			t.Errorf("row %s statusText = %q", row.Method, row.StatusText)
		}
		if row.BodyShape == "" || len(row.Headers) == 0 {
			t.Errorf("row %s incomplete: %+v", row.Method, row)
		}
	}

	// The rows that MUST be body-less by protocol rule.
	for _, method := range []string{"PATCH", "HEAD", "OPTIONS"} {
		for _, row := range payload.Methods {
			if row.Method == method && row.Body != "" {
				t.Errorf("%s must answer without a body; got %q", method, row.Body)
			}
		}
	}
}

func TestReqresMethodsSSEBeatStreamsAndFlushesPerEvent(t *testing.T) {
	beat := BuildReqresMethodsPayload().SSE

	if beat.ContentType != "text/event-stream" {
		t.Errorf("SSE content type = %q", beat.ContentType)
	}
	if len(beat.Events) != reqresSSEEventCount {
		t.Fatalf("SSE events = %d, want %d", len(beat.Events), reqresSSEEventCount)
	}
	for i, event := range beat.Events {
		if event.Name != "beat" {
			t.Errorf("event %d name = %q, want beat", i, event.Name)
		}
		want := `{"sequence":` + string(rune('0'+i+1)) + `,"state":"tick"}`
		if event.Data != want {
			t.Errorf("event %d data = %q, want %q", i, event.Data, want)
		}
	}
	if beat.Flushes != reqresSSEEventCount {
		t.Errorf("flushes = %d, want one per event (%d)", beat.Flushes, reqresSSEEventCount)
	}
	if !strings.Contains(beat.Framing, "blank line") {
		t.Errorf("framing note = %q", beat.Framing)
	}

	// Re-observe independently: the raw body must be the canonical wire
	// form — event line, data line, blank line — repeated per event, with
	// no timing values anywhere in it.
	recorder := &reqresCountingRecorder{ResponseRecorder: httptest.NewRecorder()}
	writeReqresSSEBeat(recorder)
	var raw strings.Builder
	for sequence := 1; sequence <= reqresSSEEventCount; sequence++ {
		fmt.Fprintf(&raw, "event: beat\ndata: {\"sequence\":%d,\"state\":\"tick\"}\n\n", sequence)
	}
	if recorder.Body.String() != raw.String() {
		t.Errorf("raw SSE body = %q, want %q", recorder.Body.String(), raw.String())
	}
	if recorder.flushes != reqresSSEEventCount {
		t.Errorf("independent flush count = %d, want %d", recorder.flushes, reqresSSEEventCount)
	}
}

func TestReqresMethodsPayloadIsByteForByteDeterministic(t *testing.T) {
	first, err := json.Marshal(BuildReqresMethodsPayload())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	second, err := json.Marshal(BuildReqresMethodsPayload())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(first) != string(second) {
		t.Error("two builds must serialize identically — no clocks, no maps in iteration order")
	}
	for _, want := range []string{
		`"resource":"/api/demo/reqres/methods/exhibit"`,
		`"status":204`, `"status":201`, `"status":202`, `"status":405`,
		`"contentType":"text/event-stream"`,
	} {
		if !strings.Contains(string(first), want) {
			t.Errorf("payload JSON missing %s", want)
		}
	}
}
