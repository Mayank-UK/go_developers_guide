// Request-inspector demo payload: the request side of topic 09, made
// observable — the json.Decoder discipline a Go handler owes every body.
//
// GET /api/demo/reqres/inspector serves this payload. The thin handler
// extracts the live request's facts (method, path, content type, body
// bytes capped at MaxReqresBodyBytes) and hands them to this pure
// builder. The builder's core is decodeReqresBody: the streaming-decode
// discipline — json.NewDecoder over the body, DisallowUnknownFields,
// first-error reporting, and exactly one JSON value per body — run over
// four static exhibit bodies (valid, unknown-field, truncated,
// wrong-typed) and over whatever body the live request actually carried.
// Every verdict is deterministic: same body in, same verdict out, with
// the decoded (possibly partial) state rendered alongside the error.
package demos

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// MaxReqresBodyBytes caps the body the inspector's handler will read: the
// same sane bound the scenario PATCH handler enforces. A body larger than
// this is truncated for reporting and flagged — hostile input is a fact
// worth echoing, never a reason to allocate without limit.
const MaxReqresBodyBytes = 4 << 10

// reqresBody is the typed body the decoder discipline targets: small,
// ordinary, exactly the shape a real endpoint would declare.
type reqresBody struct {
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

// The four static exhibit bodies — pinned constants so the payload's rows
// are byte-for-byte identical on every call, and the reader can see the
// exact input next to its verdict.
const (
	reqresValidBody        = `{"title":"Read the body as a stream","tags":["request","json"]}`
	reqresUnknownFieldBody = `{"title":"Valid title","urgent":true,"tags":"not-a-list"}`
	reqresTruncatedBody    = `{"title":"complete","tags":["a"`
	reqresWrongTypeBody    = `{"title":42,"tags":[]}`
)

// reqresVerdict is one decode run's raw outcome: the decoded value (which
// may be partial — see the exhibit notes), the FIRST error, whether that
// error was truncation (checked with errors.Is, never by string), and
// whether a second JSON value followed the first.
type reqresVerdict struct {
	Value         reqresBody
	Err           error
	UnexpectedEOF bool
	Trailing      bool
}

// decodeReqresBody is the discipline itself, in the shape a handler
// writes it: stream the body through a decoder, reject unknown fields,
// stop at the first offense, and require exactly one JSON value.
func decodeReqresBody(body []byte) reqresVerdict {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var value reqresBody
	if err := decoder.Decode(&value); err != nil {
		return reqresVerdict{
			Value:         value,
			Err:           err,
			UnexpectedEOF: errors.Is(err, io.ErrUnexpectedEOF),
		}
	}
	// The stream must now be exhausted: a second decodable value means the
	// body held two documents, and the FIRST one alone is not the request.
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return reqresVerdict{Value: value, Trailing: true}
	}
	return reqresVerdict{Value: value}
}

// ReqresDecodeRow is one exhibit body's verdict as the payload reports it:
// the input verbatim, the outcome, the first error verbatim with its
// concrete Go type, the offending field when the error names one, and the
// decoded state — partial state included, because what the decoder left
// in the target is part of the contract.
type ReqresDecodeRow struct {
	Label         string `json:"label"`
	Body          string `json:"body"`
	Outcome       string `json:"outcome"`
	Error         string `json:"error,omitempty"`
	ErrorType     string `json:"errorType,omitempty"`
	Field         string `json:"field,omitempty"`
	UnexpectedEOF bool   `json:"unexpectedEOF,omitempty"`
	Decoded       string `json:"decoded"`
}

// ReqresLiveRequest is the live request's own facts as the handler
// extracted them — everything here came off the wire with this very GET.
type ReqresLiveRequest struct {
	Method            string `json:"method"`
	Path              string `json:"path"`
	ContentType       string `json:"contentType,omitempty"`
	BodyBytes         int    `json:"bodyBytes"`
	BodyLimit         int    `json:"bodyLimit"`
	BodyLimitExceeded bool   `json:"bodyLimitExceeded,omitempty"`
}

// ReqresDiscipline states the rules every body is held to, as facts the
// client can render beside the verdicts.
type ReqresDiscipline struct {
	Decoder       string `json:"decoder"`
	UnknownFields string `json:"unknownFields"`
	Trailing      string `json:"trailing"`
	FirstError    string `json:"firstError"`
}

// ReqresLiveDecode is the same discipline applied to the body THIS
// request carried (a GET may legally carry one; usually it carries none).
type ReqresLiveDecode struct {
	Sent          bool   `json:"sent"`
	Body          string `json:"body,omitempty"`
	Outcome       string `json:"outcome"`
	Error         string `json:"error,omitempty"`
	ErrorType     string `json:"errorType,omitempty"`
	Field         string `json:"field,omitempty"`
	UnexpectedEOF bool   `json:"unexpectedEOF,omitempty"`
	Decoded       string `json:"decoded,omitempty"`
}

// ReqresInspectorPayload is the wire shape of GET /api/demo/reqres/inspector
// (camelCase JSON) — the request-response topic's request-side demo.
type ReqresInspectorPayload struct {
	Request    ReqresLiveRequest `json:"request"`
	Discipline ReqresDiscipline  `json:"discipline"`
	Exhibits   []ReqresDecodeRow `json:"exhibits"`
	LiveBody   ReqresLiveDecode  `json:"liveBody"`
}

// reqresDecodedState renders the decoded (possibly partial) value the way
// Go source would write it — present-but-empty is data here: a field the
// decoder never reached reports its zero value, honestly.
func reqresDecodedState(value reqresBody) string {
	return fmt.Sprintf("title=%q tags=%q", value.Title, value.Tags)
}

// reqresRowFromVerdict shapes one verdict into its payload row, shared by
// the static exhibits and the live body.
func reqresRowFromVerdict(label, body string, verdict reqresVerdict) ReqresDecodeRow {
	row := ReqresDecodeRow{
		Label:         label,
		Body:          body,
		Decoded:       reqresDecodedState(verdict.Value),
		UnexpectedEOF: verdict.UnexpectedEOF,
	}
	switch {
	case verdict.Trailing:
		row.Outcome = "rejected"
		row.Error = "body must contain exactly one JSON value (trailing content after the first)"
	case verdict.Err != nil:
		row.Outcome = "rejected"
		row.Error = verdict.Err.Error()
		row.ErrorType = fmt.Sprintf("%T", verdict.Err)
		var typeErr *json.UnmarshalTypeError
		if errors.As(verdict.Err, &typeErr) {
			row.Field = typeErr.Field
		}
	default:
		row.Outcome = "accepted"
	}
	return row
}

// reqresExhibits runs the discipline over the four pinned bodies. Each row
// teaches one verdict: the valid body accepted; the unknown field rejected
// BEFORE the later type error (first error wins) with the earlier field
// already decoded; the truncated body rejected with NOTHING decoded — the
// decoder scans one complete JSON value before any field lands; the
// wrong-typed field rejected by *json.UnmarshalTypeError naming the field.
func reqresExhibits() []ReqresDecodeRow {
	return []ReqresDecodeRow{
		reqresRowFromVerdict("valid body", reqresValidBody, decodeReqresBody([]byte(reqresValidBody))),
		reqresRowFromVerdict("unknown field, then a type error — first error wins",
			reqresUnknownFieldBody, decodeReqresBody([]byte(reqresUnknownFieldBody))),
		reqresRowFromVerdict("truncated mid-array", reqresTruncatedBody, decodeReqresBody([]byte(reqresTruncatedBody))),
		reqresRowFromVerdict("wrong-typed field", reqresWrongTypeBody, decodeReqresBody([]byte(reqresWrongTypeBody))),
	}
}

// reqresLiveDecode applies the same discipline to the body the handler
// extracted from this very request. No body (the usual GET) reports
// outcome "none" — absence stated, never guessed around.
func reqresLiveDecode(facts ReqresInspectorFacts) ReqresLiveDecode {
	if len(facts.Body) == 0 {
		return ReqresLiveDecode{Sent: false, Outcome: "none"}
	}
	row := reqresRowFromVerdict("this request's body", string(facts.Body), decodeReqresBody(facts.Body))
	return ReqresLiveDecode{
		Sent:          true,
		Body:          row.Body,
		Outcome:       row.Outcome,
		Error:         row.Error,
		ErrorType:     row.ErrorType,
		Field:         row.Field,
		UnexpectedEOF: row.UnexpectedEOF,
		Decoded:       row.Decoded,
	}
}

// ReqresInspectorFacts is the plain data the thin handler extracts from
// one live request — the builder never touches the request itself.
type ReqresInspectorFacts struct {
	Method            string
	Path              string
	ContentType       string
	Body              []byte
	BodyLimitExceeded bool
}

// BuildReqresInspectorPayload assembles the inspector payload from the
// handler's facts. It is pure: plain data in, payload out, no I/O — same
// facts, same payload, every time.
func BuildReqresInspectorPayload(facts ReqresInspectorFacts) ReqresInspectorPayload {
	return ReqresInspectorPayload{
		Request: ReqresLiveRequest{
			Method:            facts.Method,
			Path:              facts.Path,
			ContentType:       facts.ContentType,
			BodyBytes:         len(facts.Body),
			BodyLimit:         MaxReqresBodyBytes,
			BodyLimitExceeded: facts.BodyLimitExceeded,
		},
		Discipline: ReqresDiscipline{
			Decoder:       "json.NewDecoder over the request body",
			UnknownFields: "rejected — DisallowUnknownFields",
			Trailing:      "exactly one JSON value — a second Decode must return io.EOF",
			FirstError:    "reported verbatim; decoding stops at the first offense",
		},
		Exhibits: reqresExhibits(),
		LiveBody: reqresLiveDecode(facts),
	}
}
