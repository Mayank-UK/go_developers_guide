package demos

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

// The arena's contract: every verdict on the wire is the REAL machinery's
// output. These tests pin the full truth table the topic's prose claims —
// which layer refuses which candidate, with which error strings, and which
// envelope — so a Go upgrade or a refactor of the decode discipline, the
// rules, or core.WriteError that changes any outcome fails HERE, next to
// the claim. Every expectation below was derived by running the builder,
// not by guessing.

// wantArenaRow pins one candidate's verdicts: decode status, rules
// status, composed HTTP status.
var wantArenaRows = map[string]struct {
	decode   string
	rules    string
	response int
}{
	"valid-complete":         {decodeAccepted, rulesValidated, http.StatusOK},
	"broken-json-syntax":     {decodeRejected, rulesSkipped, http.StatusUnprocessableEntity},
	"wrong-type-score":       {decodeRejected, rulesSkipped, http.StatusUnprocessableEntity},
	"unknown-field-priority": {decodeRejected, rulesSkipped, http.StatusUnprocessableEntity},
	"trailing-second-object": {decodeRejected, rulesSkipped, http.StatusUnprocessableEntity},
	"empty-required-title":   {decodeAccepted, rulesRejected, http.StatusUnprocessableEntity},
	"too-long-title":         {decodeAccepted, rulesRejected, http.StatusUnprocessableEntity},
	"negative-score":         {decodeAccepted, rulesRejected, http.StatusUnprocessableEntity},
	"bad-email-shape":        {decodeAccepted, rulesRejected, http.StatusUnprocessableEntity},
	"absent-required-score":  {decodeAccepted, rulesRejected, http.StatusUnprocessableEntity},
	"absent-optional-body":   {decodeAccepted, rulesValidated, http.StatusOK},
	"collect-every-issue":    {decodeAccepted, rulesRejected, http.StatusUnprocessableEntity},
}

// wantArenaOrder is the roster's teaching order.
var wantArenaOrder = []string{
	"valid-complete",
	"broken-json-syntax",
	"wrong-type-score",
	"unknown-field-priority",
	"trailing-second-object",
	"empty-required-title",
	"too-long-title",
	"negative-score",
	"bad-email-shape",
	"absent-required-score",
	"absent-optional-body",
	"collect-every-issue",
}

func arenaPayload() ValidationArenaPayload {
	return BuildValidationArenaPayload()
}

func TestValidationArenaRosterIsTheTeachingOrder(t *testing.T) {
	candidates := arenaPayload().Candidates
	if len(candidates) != len(wantArenaOrder) {
		t.Fatalf("candidates = %d, want %d", len(candidates), len(wantArenaOrder))
	}
	for i, candidate := range candidates {
		if candidate.ID != wantArenaOrder[i] {
			t.Errorf("candidate %d = %q, want %q", i, candidate.ID, wantArenaOrder[i])
		}
	}
}

func TestValidationArenaVerdictsMatchThePinnedTruthTable(t *testing.T) {
	for _, candidate := range arenaPayload().Candidates {
		want, ok := wantArenaRows[candidate.ID]
		if !ok {
			t.Errorf("unexpected candidate %q", candidate.ID)
			continue
		}
		if candidate.Decode.Status != want.decode {
			t.Errorf("%s decode = %q, want %q", candidate.ID, candidate.Decode.Status, want.decode)
		}
		if candidate.Validate.Status != want.rules {
			t.Errorf("%s rules = %q, want %q", candidate.ID, candidate.Validate.Status, want.rules)
		}
		if candidate.Response.Status != want.response {
			t.Errorf("%s response = %d, want %d", candidate.ID, candidate.Response.Status, want.response)
		}
		// The one-value invariant: a refused decode means the field rules
		// never ran, and a 422 always carries its envelope.
		if want.decode == decodeRejected && len(candidate.Validate.Issues) != 0 {
			t.Errorf("%s: decode refused but issues reported", candidate.ID)
		}
		if want.response == http.StatusUnprocessableEntity && len(candidate.Response.Envelope) == 0 {
			t.Errorf("%s answers 422 without an envelope", candidate.ID)
		}
		if want.response == http.StatusOK && (candidate.Response.Envelope != nil || candidate.Response.Validated == nil) {
			t.Errorf("%s answers 200 without the validated form (and with an envelope)", candidate.ID)
		}
	}
}

func TestValidationArenaDecodeErrorsAreTheRealOnes(t *testing.T) {
	candidates := arenaPayload().Candidates
	byID := make(map[string]ArenaCandidate, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	wants := map[string]struct{ errType, err string }{
		"broken-json-syntax":     {"*errors.errorString", "unexpected EOF"},
		"wrong-type-score":       {"*json.UnmarshalTypeError", "json: cannot unmarshal string into Go struct field FeedbackForm.score of type int"},
		"unknown-field-priority": {"*errors.errorString", `json: unknown field "priority"`},
		"trailing-second-object": {"*errors.errorString", "body must contain exactly one JSON object"},
	}
	for id, want := range wants {
		got := byID[id].Decode
		if got.ErrorType != want.errType || got.Error != want.err {
			t.Errorf("%s decode error = %s / %q, want %s / %q", id, got.ErrorType, got.Error, want.errType, want.err)
		}
	}
}

func TestValidationArenaCollectsEveryIssuePerSubmission(t *testing.T) {
	payload := arenaPayload()
	var multi *ArenaCandidate
	for i := range payload.Candidates {
		if payload.Candidates[i].ID == "collect-every-issue" {
			multi = &payload.Candidates[i]
			break
		}
	}
	if multi == nil {
		t.Fatal("collect-every-issue candidate missing")
	}
	want := []ArenaIssue{
		{Field: "title", Rule: "required — present-but-empty is data, and data can be wrong", Got: "\"\""},
		{Field: "score", Rule: "required, whole number 1–5 — absent decodes to 0 and fails the same gate", Got: "9"},
		{Field: "email", Rule: "required — one @ with non-empty text on both sides (a shape check, not the RFC grammar)", Got: "\"nope\""},
	}
	if !reflect.DeepEqual(multi.Validate.Issues, want) {
		t.Errorf("collected issues = %+v, want %+v", multi.Validate.Issues, want)
	}
}

func TestValidationArenaEmptyRequiredIsDataNotAbsence(t *testing.T) {
	// The calibration beat: "" decodes cleanly, then the required rule
	// rejects it — and the zero-value blur's other side: an absent
	// OPTIONAL field validates with its zero default.
	for _, candidate := range arenaPayload().Candidates {
		switch candidate.ID {
		case "empty-required-title":
			if len(candidate.Validate.Issues) != 1 || candidate.Validate.Issues[0].Field != "title" {
				t.Errorf("empty-required-title issues = %+v", candidate.Validate.Issues)
			}
		case "absent-required-score":
			if len(candidate.Validate.Issues) != 1 || candidate.Validate.Issues[0].Got != "0" {
				t.Errorf("absent-required-score issues = %+v, want the zero-value blur", candidate.Validate.Issues)
			}
		case "absent-optional-body":
			if candidate.Validate.Status != rulesValidated || candidate.Response.Validated == nil ||
				candidate.Response.Validated.Body != "" {
				t.Errorf("absent-optional-body = %+v, want validated with the zero default", candidate.Response)
			}
		}
	}
}

func TestValidationArenaEnvelopesAreTheRealWriterShape(t *testing.T) {
	// Field-for-field against core's envelope: {error:{code,message,
	// requestId,details}} — the same writer production 422s ride.
	type errorBody struct {
		Code      string          `json:"code"`
		Message   string          `json:"message"`
		RequestID string          `json:"requestId"`
		Details   json.RawMessage `json:"details"`
	}
	type envelope struct {
		Error errorBody `json:"error"`
	}
	for _, candidate := range arenaPayload().Candidates {
		if candidate.Response.Status != http.StatusUnprocessableEntity {
			continue
		}
		var got envelope
		if err := json.Unmarshal(candidate.Response.Envelope, &got); err != nil {
			t.Fatalf("%s envelope does not parse: %v\n%s", candidate.ID, err, candidate.Response.Envelope)
		}
		if got.Error.Code != "validation_failed" || got.Error.RequestID != arenaRequestID {
			t.Errorf("%s envelope code/requestId = %q/%q", candidate.ID, got.Error.Code, got.Error.RequestID)
		}
		// Decode refusals carry {body: reason}; rule refusals carry the
		// issue array — both verbatim from core.WriteError.
		if candidate.Decode.Status == decodeRejected {
			var body map[string]string
			if err := json.Unmarshal(got.Error.Details, &body); err != nil || body["body"] != candidate.Decode.Error {
				t.Errorf("%s decode-failure details = %s", candidate.ID, got.Error.Details)
			}
		} else {
			var issues []ArenaIssue
			if err := json.Unmarshal(got.Error.Details, &issues); err != nil ||
				!reflect.DeepEqual(issues, candidate.Validate.Issues) {
				t.Errorf("%s rule-failure details = %s, want the reported issues", candidate.ID, got.Error.Details)
			}
		}
	}
}

func TestValidationArenaExhibitIsTheMultiErrorCandidatesEnvelope(t *testing.T) {
	payload := arenaPayload()
	if payload.Envelope.CandidateID != "collect-every-issue" || payload.Envelope.Status != http.StatusUnprocessableEntity {
		t.Errorf("exhibit = %+v, want collect-every-issue at 422", payload.Envelope)
	}
	var candidate *ArenaCandidate
	for i := range payload.Candidates {
		if payload.Candidates[i].ID == payload.Envelope.CandidateID {
			candidate = &payload.Candidates[i]
			break
		}
	}
	if candidate == nil {
		t.Fatal("exhibit names a candidate the roster does not carry")
	}
	if string(payload.Envelope.Body) != string(candidate.Response.Envelope) {
		t.Errorf(
			"exhibit body must be the candidate's own 422 envelope:\nexhibit:   %s\ncandidate: %s",
			payload.Envelope.Body, candidate.Response.Envelope,
		)
	}
	if len(payload.Envelope.Anatomy) != 5 {
		t.Errorf("anatomy rows = %d, want 5", len(payload.Envelope.Anatomy))
	}
}

func TestValidationArenaComparisonIsHonestAboutTheLibrary(t *testing.T) {
	comparison := arenaPayload().Comparison
	if comparison.Imported {
		t.Error("the comparison must not claim the library is imported")
	}
	if comparison.Library != "github.com/go-playground/validator/v10" {
		t.Errorf("library = %q", comparison.Library)
	}
	wantTags := map[string]string{
		"title": "required,max=80",
		"body":  "max=280",
		"score": "required,gte=1,lte=5",
		"email": "required,email",
	}
	if len(comparison.Rows) != len(wantTags) {
		t.Fatalf("comparison rows = %d, want %d", len(comparison.Rows), len(wantTags))
	}
	for _, row := range comparison.Rows {
		if row.Tag != wantTags[row.Field] {
			t.Errorf("%s tag = %q, want %q", row.Field, row.Tag, wantTags[row.Field])
		}
		if row.LibraryRule == "" || row.HandRolled == "" {
			t.Errorf("%s comparison row lacks its pairing: %+v", row.Field, row)
		}
	}
}

func TestValidationArenaSubjectAndLayersStateTheContract(t *testing.T) {
	payload := arenaPayload()
	if payload.Subject.Name != "FeedbackForm" ||
		payload.Subject.Source != "server/internal/domain/demos/validation_arena.go" ||
		len(payload.Subject.Fields) != 4 {
		t.Errorf("subject drifted: %+v", payload.Subject)
	}
	required := 0
	for _, field := range payload.Subject.Fields {
		if field.Required {
			required++
		}
	}
	if required != 3 {
		t.Errorf("required fields = %d, want 3 (body is the optional one)", required)
	}
	if len(payload.Layers) != 3 || payload.Layers[0].ID != "decode" || payload.Layers[1].ID != "rules" || payload.Layers[2].ID != "library" {
		t.Errorf("layers drifted: %+v", payload.Layers)
	}
}

func TestValidationArenaIsDeterministic(t *testing.T) {
	if !reflect.DeepEqual(BuildValidationArenaPayload(), BuildValidationArenaPayload()) {
		t.Error("two builds must produce byte-identical payloads")
	}
}
