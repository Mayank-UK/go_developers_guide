package demos

import (
	"reflect"
	"testing"
)

// The grpc-console demo's own pins: the payload is DETERMINISTIC (two
// builds, byte-identical reports — the whole in-process stack resets per
// call and no wall-clock text leaks into rows), it genuinely exercises
// all four method kinds through the generated stubs (each kind's rows are
// populated by real RPC results, including the NotFound miss and the
// DeadlineExceeded deadline), and the page-token walk behaves like a
// cursor protocol.

func TestGRPCConsolePayloadIsDeterministic(t *testing.T) {
	first := BuildGRPCConsolePayload()
	second := BuildGRPCConsolePayload()
	if !reflect.DeepEqual(first, second) {
		t.Fatal("two builds of the grpc-console payload must be identical — wall-clock or map-order drift in a report")
	}
}

func TestGRPCConsoleCoversAllFourMethodKinds(t *testing.T) {
	payload := BuildGRPCConsolePayload()
	kinds := map[string]*GRPCCallReport{}
	for i := range payload.Calls {
		call := &payload.Calls[i]
		kinds[call.Kind] = call
	}
	for _, want := range []string{"unary", "server-stream", "client-stream", "bidi"} {
		call, ok := kinds[want]
		if !ok {
			t.Errorf("no %q report in the payload — all four kinds must be exercised", want)
			continue
		}
		if len(call.Results) == 0 {
			t.Errorf("the %q report has no result rows — kinds must be demonstrated by REAL RPC results", want)
		}
		if call.Method == "" || call.Pattern == "" || call.Summary == "" {
			t.Errorf("the %q report is missing method/pattern/summary: %+v", want, call)
		}
	}
}

func TestGRPCConsoleObservesTheThreeTaughtStatuses(t *testing.T) {
	payload := BuildGRPCConsolePayload()
	observed := map[string]bool{}
	for _, call := range payload.Calls {
		observed[call.StatusCode] = true
		if call.StatusCode == "OK" && call.StatusNumber != 0 {
			t.Errorf("OK must carry numeric 0, got %d", call.StatusNumber)
		}
	}
	for _, want := range []string{"OK", "NotFound", "DeadlineExceeded"} {
		if !observed[want] {
			t.Errorf("status %s never observed — the demo must record real codes as data", want)
		}
	}
	for _, call := range payload.Calls {
		if call.Kind == "unary" && call.Method == "/guide.posts.v1.PostsService/GetPost" && call.StatusCode == "DeadlineExceeded" {
			if call.StatusNumber != 4 {
				t.Errorf("DeadlineExceeded numeric = %d, want 4", call.StatusNumber)
			}
		}
	}
}

func TestGRPCConsoleNotFoundCarriesTheServerMessage(t *testing.T) {
	payload := BuildGRPCConsolePayload()
	for _, call := range payload.Calls {
		if call.StatusCode != "NotFound" {
			continue
		}
		found := false
		for _, result := range call.Results {
			if result.OK {
				continue
			}
			if result.Label == "observed status" && len(result.Detail) > 0 {
				found = true
			}
		}
		if !found {
			t.Errorf("the NotFound report must carry the observed status row with the server's message: %+v", call.Results)
		}
	}
}

func TestGRPCConsoleListWalksPagesByToken(t *testing.T) {
	payload := BuildGRPCConsolePayload()
	var list *GRPCCallReport
	for i := range payload.Calls {
		if payload.Calls[i].Kind == "server-stream" {
			list = &payload.Calls[i]
		}
	}
	if list == nil {
		t.Fatal("no server-stream report")
	}
	if list.RoundTrips != 2 {
		t.Errorf("round trips = %d, want 2 (the page-1 call and the cursor's page)", list.RoundTrips)
	}
	var cursorRows, exhaustedRows int
	for _, result := range list.Results {
		if len(result.Label) >= len("cursor after page") && result.Label[:len("cursor after page")] == "cursor after page" {
			if result.Detail[0] == '"' {
				cursorRows++
			} else {
				exhaustedRows++
			}
		}
	}
	if cursorRows != 1 || exhaustedRows != 1 {
		t.Errorf("cursor rows = %d quoted + %d empty, want 1 + 1 (a cursor after page 1, exhaustion after page 2)", cursorRows, exhaustedRows)
	}
}

func TestGRPCConsoleBidiSurvivesTheRejectedItem(t *testing.T) {
	payload := BuildGRPCConsolePayload()
	var bidi *GRPCCallReport
	for i := range payload.Calls {
		if payload.Calls[i].Kind == "bidi" {
			bidi = &payload.Calls[i]
		}
	}
	if bidi == nil {
		t.Fatal("no bidi report")
	}
	var rejected, applied int
	for _, result := range bidi.Results {
		if !result.OK {
			rejected++
		} else if len(result.Label) >= 4 && result.Label[:4] == "ack " {
			applied++
		}
	}
	if rejected != 1 || applied < 2 {
		t.Errorf("bidi acks = %d applied / %d rejected, want at least 2 applied and exactly 1 rejected (per-item acks, stream survives)", applied, rejected)
	}
	if bidi.StatusCode != "OK" {
		t.Errorf("bidi overall status = %s, want OK — a rejected item is data, not a failed RPC", bidi.StatusCode)
	}
}

func TestGRPCConsoleTransportFactsAreTheRealStack(t *testing.T) {
	payload := BuildGRPCConsolePayload()
	facts := payload.Transport
	if facts.ProtoFile != "server/api/proto/posts.proto" ||
		facts.Service != "guide.posts.v1.PostsService" ||
		facts.GeneratedGo != "server/internal/grpcapi/pb" ||
		facts.CodegenCmd != "buf generate" ||
		facts.DialTarget != "passthrough:///bufnet" {
		t.Errorf("transport facts drifted from the real stack: %+v", facts)
	}
}
