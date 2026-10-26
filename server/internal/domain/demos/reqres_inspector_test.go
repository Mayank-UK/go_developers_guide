package demos

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

// The inspector's facts are computed live from the decoder; these tests
// pin the verdicts the request-response topic's prose claims, so an
// encoding/json behavior change fails HERE first, next to the claim. The
// pinned strings were captured from the running decoder (go 1.27) —
// they are the toolchain's own words, not ours.

func TestReqresInspectorExhibitsCarryDeterministicVerdicts(t *testing.T) {
	payload := BuildReqresInspectorPayload(ReqresInspectorFacts{
		Method: "GET", Path: "/api/demo/reqres/inspector",
	})

	if len(payload.Exhibits) != 4 {
		t.Fatalf("exhibits = %d rows, want the four pinned bodies", len(payload.Exhibits))
	}
	byLabel := make(map[string]ReqresDecodeRow, len(payload.Exhibits))
	for _, row := range payload.Exhibits {
		byLabel[row.Label] = row
	}

	// 1. The valid body: accepted, fully decoded, nothing rejected.
	valid := byLabel["valid body"]
	if valid.Outcome != "accepted" || valid.Error != "" || valid.ErrorType != "" {
		t.Errorf("valid body = %+v, want accepted with no error", valid)
	}
	if valid.Decoded != `title="Read the body as a stream" tags=["request" "json"]` {
		t.Errorf("valid decoded state = %q", valid.Decoded)
	}

	// 2. Unknown field BEFORE a wrong-typed field: the FIRST error wins —
	// the unknown field is reported, the later type error never happens,
	// and the field decoded before the offense stays set (partial state).
	unknown := byLabel["unknown field, then a type error — first error wins"]
	if unknown.Outcome != "rejected" {
		t.Fatalf("unknown-field exhibit = %+v, want rejected", unknown)
	}
	if unknown.Error != `json: unknown field "urgent"` {
		t.Errorf("unknown-field error = %q, want the decoder's first error verbatim", unknown.Error)
	}
	if unknown.ErrorType != "*errors.errorString" {
		t.Errorf("unknown-field error type = %q, want *errors.errorString", unknown.ErrorType)
	}
	if unknown.Decoded != `title="Valid title" tags=[]` {
		t.Errorf("unknown-field decoded state = %q, want the pre-offense field preserved", unknown.Decoded)
	}

	// 3. Truncated mid-array: unexpected EOF, and NOTHING decoded — the
	// decoder scans one complete JSON value before any field lands.
	truncated := byLabel["truncated mid-array"]
	if truncated.Outcome != "rejected" || truncated.Error != "unexpected EOF" {
		t.Errorf("truncated exhibit = %+v, want rejected with unexpected EOF", truncated)
	}
	if !truncated.UnexpectedEOF {
		t.Error("truncated exhibit must flag unexpectedEOF (checked with errors.Is)")
	}
	if truncated.Decoded != `title="" tags=[]` {
		t.Errorf("truncated decoded state = %q, want the zero value (scan failed before decode)", truncated.Decoded)
	}

	// 4. Wrong-typed field: the typed error names the field and the Go
	// type, verbatim.
	wrongType := byLabel["wrong-typed field"]
	if wrongType.Outcome != "rejected" || wrongType.Field != "title" {
		t.Errorf("wrong-typed exhibit = %+v, want rejected with field \"title\"", wrongType)
	}
	if wrongType.ErrorType != "*json.UnmarshalTypeError" {
		t.Errorf("wrong-typed error type = %q, want *json.UnmarshalTypeError", wrongType.ErrorType)
	}
	if wrongType.Error != "json: cannot unmarshal number into Go struct field reqresBody.title of type string" {
		t.Errorf("wrong-typed error = %q", wrongType.Error)
	}
}

func TestReqresInspectorDisciplineRejectsTrailingSecondValue(t *testing.T) {
	// The payload's four exhibits do not include a two-document body; the
	// discipline's exactly-one-value rule is pinned here instead.
	verdict := decodeReqresBody([]byte(`{"title":"Two documents","tags":[]} {"title":"second"}`))
	if !verdict.Trailing {
		t.Fatal("a body with a second JSON value must be flagged as trailing")
	}
	row := reqresRowFromVerdict("trailing", "…", verdict)
	if row.Outcome != "rejected" || row.Error == "" {
		t.Errorf("trailing row = %+v, want rejected with the one-value rule named", row)
	}
}

func TestReqresInspectorTruncationIsCheckedWithErrorsIs(t *testing.T) {
	verdict := decodeReqresBody([]byte(reqresTruncatedBody))
	if !errors.Is(verdict.Err, io.ErrUnexpectedEOF) {
		t.Errorf("truncated body error %v must satisfy errors.Is(err, io.ErrUnexpectedEOF)", verdict.Err)
	}
	if verdict.UnexpectedEOF != true {
		t.Error("the verdict must carry the check's result, not a string match")
	}
}

func TestReqresInspectorLiveBodyBeatsReflectTheBodySent(t *testing.T) {
	// No body (the usual GET): absence stated, never guessed around.
	empty := BuildReqresInspectorPayload(ReqresInspectorFacts{Method: "GET", Path: "/p"})
	if empty.LiveBody.Sent || empty.LiveBody.Outcome != "none" || empty.LiveBody.Decoded != "" {
		t.Errorf("no-body live beat = %+v, want sent=false outcome=none", empty.LiveBody)
	}
	if empty.Request.BodyBytes != 0 || empty.Request.BodyLimit != MaxReqresBodyBytes {
		t.Errorf("request facts = %+v, want bodyBytes 0 and the cap reported", empty.Request)
	}

	// A body on the GET: the same discipline, same verdicts as the exhibits.
	live := BuildReqresInspectorPayload(ReqresInspectorFacts{
		Method: "GET", Path: "/p", ContentType: "application/json",
		Body: []byte(`{"title":"sent along","tags":["live"]}`),
	})
	if !live.LiveBody.Sent || live.LiveBody.Outcome != "accepted" {
		t.Fatalf("live body beat = %+v, want sent and accepted", live.LiveBody)
	}
	if live.LiveBody.Decoded != `title="sent along" tags=["live"]` {
		t.Errorf("live decoded state = %q", live.LiveBody.Decoded)
	}
	if live.Request.ContentType != "application/json" || live.Request.BodyBytes != 38 {
		t.Errorf("request facts = %+v", live.Request)
	}

	// A body the cap truncated: the flag rides the request facts and the
	// truncated bytes still get their honest (rejected) verdict.
	big := BuildReqresInspectorPayload(ReqresInspectorFacts{
		Method: "GET", Path: "/p", Body: []byte(`{"title":"cut short","tags":["a"`),
		BodyLimitExceeded: true,
	})
	if !big.Request.BodyLimitExceeded {
		t.Error("the exceeded flag must survive into the payload")
	}
	if big.LiveBody.Outcome != "rejected" || !big.LiveBody.UnexpectedEOF {
		t.Errorf("truncated live body = %+v, want rejected unexpectedEOF", big.LiveBody)
	}
}

func TestReqresInspectorPayloadMarshalsCamelCase(t *testing.T) {
	raw, err := json.Marshal(BuildReqresInspectorPayload(ReqresInspectorFacts{Method: "GET", Path: "/p"}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		`"request"`, `"discipline"`, `"exhibits"`, `"liveBody"`,
		`"bodyLimit":4096`, `"errorType":"*json.UnmarshalTypeError"`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("payload JSON missing %s: %s", want, raw)
		}
	}
}
