// Validation-arena demo payload: a fixed roster of candidate request
// bodies, each judged by the REAL validation machinery this guide teaches.
//
// GET /api/demo/validation/arena serves this payload — the validation
// topic's inline demo. The arena's point is that nothing here is a
// simulation: every candidate body (static exhibit input, chosen at build
// time) is run through layer one — the same encoding/json decode
// discipline the scenario PATCH handler uses, tightened with
// DisallowUnknownFields — and, when decoding succeeds, through layer two —
// the hand-rolled field rules below, quoted by the topic as real code.
// Every decode error string, every field issue, and every 422 envelope is
// the genuine output of that run: the envelopes are produced by the
// repo's own central error writer (core.WriteError) against a request
// context carrying the demo's fixed request id, so the exhibited body is
// field-for-field what a real 422 from this server looks like.
//
// The builder is pure: no inputs, no I/O, no mutable state — the handler
// layer adds only transport.

package demos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"guide/internal/core"
)

// FeedbackForm is the arena's judged request body: a plausible
// create-shaped payload carrying every rule shape a real endpoint meets —
// required text with a length bound, optional text that defaults to its
// zero value, a bounded whole number, and a shape-checked identifier. The
// wire names are camelCase via struct tags, exactly like every request
// struct this server decodes.
type FeedbackForm struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Score int    `json:"score"`
	Email string `json:"email"`
}

// The form's rule constants — the contract in one place, quoted by the
// topic beside the code that enforces it.
const (
	maxTitleRunes = 80
	maxBodyRunes  = 280
	minScore      = 1
	maxScore      = 5
)

// ArenaIssue is one validation failure: which wire field broke which
// rule, and what the field actually carried. A slice of these is the
// details array the 422 envelope rides — structured context a client can
// render per field, not a string to parse.
type ArenaIssue struct {
	Field string `json:"field"`
	Rule  string `json:"rule"`
	Got   string `json:"got"`
}

// ValidateFeedback is layer two, the backbone: plain code over decoded
// fields. Every rule is a readable branch, every failure an ArenaIssue,
// and the branches COLLECT instead of returning early, so one bad
// submission reports every fixable field at once. Present-but-empty is
// data here — "" is judged by the required rule, never conflated with
// absence, because by this point the decoder has already answered
// absence's questions.
func ValidateFeedback(form FeedbackForm) []ArenaIssue {
	var issues []ArenaIssue
	title := strings.TrimSpace(form.Title)
	switch {
	case title == "":
		issues = append(issues, ArenaIssue{
			Field: "title",
			Rule:  "required — present-but-empty is data, and data can be wrong",
			Got:   strconv.Quote(form.Title),
		})
	case utf8.RuneCountInString(title) > maxTitleRunes:
		issues = append(issues, ArenaIssue{
			Field: "title",
			Rule:  "at most 80 runes, trimmed",
			Got:   fmt.Sprintf("%d runes", utf8.RuneCountInString(title)),
		})
	}
	if utf8.RuneCountInString(strings.TrimSpace(form.Body)) > maxBodyRunes {
		issues = append(issues, ArenaIssue{
			Field: "body",
			Rule:  "optional — but at most 280 runes when present",
			Got:   fmt.Sprintf("%d runes", utf8.RuneCountInString(strings.TrimSpace(form.Body))),
		})
	}
	if form.Score < minScore || form.Score > maxScore {
		issues = append(issues, ArenaIssue{
			Field: "score",
			Rule:  "required, whole number 1–5 — absent decodes to 0 and fails the same gate",
			Got:   strconv.Itoa(form.Score),
		})
	}
	local, domain, hasAt := strings.Cut(form.Email, "@")
	if !hasAt || local == "" || domain == "" || strings.Contains(domain, "@") {
		issues = append(issues, ArenaIssue{
			Field: "email",
			Rule:  "required — one @ with non-empty text on both sides (a shape check, not the RFC grammar)",
			Got:   strconv.Quote(form.Email),
		})
	}
	return issues
}

// errBodyNotOneValue is the decode layer's one-value verdict: a body that
// parses as JSON but is not exactly one document (a second object, stray
// trailing tokens) never reaches the field rules.
var errBodyNotOneValue = errors.New("body must contain exactly one JSON object")

// DecodeStrictFeedback is layer one: the decode discipline the scenario
// PATCH handler teaches (presentation/scenario.go), tightened one notch
// for a create-shaped body — unknown fields are REFUSED here, where the
// PATCH body deliberately ignores them. A real handler bounds the read
// first with http.MaxBytesReader; the discipline below then answers
// three questions before any field rule runs: is the body one JSON
// value, is every key a known field, and is every value the field's
// declared type.
func DecodeStrictFeedback(body []byte) (FeedbackForm, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var form FeedbackForm
	if err := decoder.Decode(&form); err != nil {
		return FeedbackForm{}, err
	}
	// Exactly one JSON value: a second Decode must hit EOF, otherwise the
	// body is not one feedback document.
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return FeedbackForm{}, errBodyNotOneValue
	}
	return form, nil
}

// arenaRequestID is the fixed id the exhibit envelopes carry. A real 422
// stamps the request's own id; the arena runs off-request, so it wears
// this one — and the x-request-id header and log line would match it.
const arenaRequestID = "demo-validation-arena"

// envelopeRecorder is the minimal http.ResponseWriter the arena needs to
// run the REAL central error writer off-request: a header map, a body
// buffer, and the status line. WriteError touches nothing else.
type envelopeRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (r *envelopeRecorder) Header() http.Header         { return r.header }
func (r *envelopeRecorder) Write(p []byte) (int, error) { return r.body.Write(p) }
func (r *envelopeRecorder) WriteHeader(status int)      { r.status = status }

// envelopeFor produces a genuine error envelope: it hands the repo's own
// core.WriteError the exact error a handler would build, against a
// request context carrying the demo's request id, and returns the status
// and body that writer produced. Field-for-field, this is what the
// running server answers a 422 with.
func envelopeFor(appErr *core.Error) (int, json.RawMessage) {
	request, err := http.NewRequestWithContext(
		core.WithRequestID(context.Background(), arenaRequestID),
		http.MethodPost, "/api/demo/validation/feedback", nil,
	)
	if err != nil {
		// Unreachable: a fixed legal method and URL cannot fail to build.
		panic("validation arena: cannot build exhibit request: " + err.Error())
	}
	recorder := &envelopeRecorder{header: make(http.Header)}
	core.WriteError(recorder, request, appErr, nil)
	return recorder.status, json.RawMessage(bytes.TrimSpace(recorder.body.Bytes()))
}

// Decode/validate outcome status vocabularies — the wire's closed sets.
const (
	decodeAccepted = "accepted"
	decodeRejected = "rejected"
	rulesValidated = "validated"
	rulesRejected  = "rejected"
	rulesSkipped   = "skipped"
)

// ArenaFieldSpec is one field of the judged form: wire name, Go type, the
// rule in one sentence, and whether the field is required.
type ArenaFieldSpec struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Rule     string `json:"rule"`
	Required bool   `json:"required"`
}

// ArenaSubject names the request struct the arena judges and where it
// lives, with every field's rule stated once.
type ArenaSubject struct {
	Name   string           `json:"name"`
	Source string           `json:"source"`
	Fields []ArenaFieldSpec `json:"fields"`
}

// ArenaLayerCard is one of the three layers, named so the client can
// render the stack the candidates fall through.
type ArenaLayerCard struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Tool    string `json:"tool"`
	Summary string `json:"summary"`
}

// ArenaDecodeOutcome is layer one's verdict on one candidate: accepted,
// or rejected with the decoder's own error type and message — the same
// strings a handler would fold into the 422's details.
type ArenaDecodeOutcome struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ArenaRulesOutcome is layer two's verdict: validated, rejected with the
// collected issues, or skipped because layer one already refused the
// body — fail fast at the boundary means the field rules never see it.
type ArenaRulesOutcome struct {
	Status string       `json:"status"`
	Issues []ArenaIssue `json:"issues,omitempty"`
}

// ArenaResponse is the composed HTTP outcome: 200 with the decoded,
// validated form for accepted candidates; 422 with the REAL envelope —
// produced by core.WriteError — for everything the layers refuse. The
// envelope is omitted when absent, never null.
type ArenaResponse struct {
	Status    int             `json:"status"`
	Envelope  json.RawMessage `json:"envelope,omitempty"`
	Validated *FeedbackForm   `json:"validated,omitempty"`
}

// ArenaCandidate is one exhibit input fully judged: the raw body, what it
// probes, both layers' verdicts, the composed response, and a teaching
// note when the row earns one.
type ArenaCandidate struct {
	ID       string             `json:"id"`
	Label    string             `json:"label"`
	Expects  string             `json:"expects"`
	Body     string             `json:"body"`
	Decode   ArenaDecodeOutcome `json:"decode"`
	Validate ArenaRulesOutcome  `json:"validate"`
	Response ArenaResponse      `json:"response"`
	Note     string             `json:"note,omitempty"`
}

// ArenaTagRow is one field of the library-comparison exhibit: the
// validate tag data, what the tag library would enforce for it, and the
// hand-rolled rule that enforces the same contract here.
type ArenaTagRow struct {
	Field       string `json:"field"`
	Tag         string `json:"tag"`
	LibraryRule string `json:"libraryRule"`
	HandRolled  string `json:"handRolled"`
}

// ArenaComparison is the ecosystem face: the tag-annotated counterpart of
// the judged form, reflected at run time. The tags are INERT — no
// validator package is imported — which is the lesson: struct tags are
// data a library must read, not behavior the compiler checks, while the
// hand-rolled rules above compile and run on their own.
type ArenaComparison struct {
	Library  string        `json:"library"`
	Imported bool          `json:"imported"`
	Rows     []ArenaTagRow `json:"rows"`
	Note     string        `json:"note"`
}

// taggedFeedback is the comparison exhibit's subject: FeedbackForm with
// its contract expressed as go-playground/validator tags instead of
// code. Nothing reads these tags in this repo — they are the shape of
// the library solution, carried so the reflection walk below can show
// tag and rule side by side.
type taggedFeedback struct {
	Title string `json:"title" validate:"required,max=80"`
	Body  string `json:"body" validate:"max=280"`
	Score int    `json:"score" validate:"required,gte=1,lte=5"`
	Email string `json:"email" validate:"required,email"`
}

// ArenaAnatomyRow names one piece of the 422 envelope and why it is
// shaped that way — the topic's error-envelope section, as data.
type ArenaAnatomyRow struct {
	Path  string `json:"path"`
	Value string `json:"value"`
	Why   string `json:"why"`
}

// ArenaEnvelopeShow is the composed 422 exhibit: the real envelope body
// for the collect-every-issue candidate, plus the row-by-row anatomy.
type ArenaEnvelopeShow struct {
	CandidateID string            `json:"candidateId"`
	Status      int               `json:"status"`
	Body        json.RawMessage   `json:"body"`
	Anatomy     []ArenaAnatomyRow `json:"anatomy"`
}

// ValidationArenaPayload is the wire shape of GET
// /api/demo/validation/arena (camelCase) — the validation topic's inline
// demo, calibrated against the fastapi guide's pydantic arena to the Go
// posture: no coercion column exists because encoding/json never coerces
// — a string that should be an int is a decode refusal, not a fixed
// value.
type ValidationArenaPayload struct {
	Subject    ArenaSubject      `json:"subject"`
	Layers     []ArenaLayerCard  `json:"layers"`
	Candidates []ArenaCandidate  `json:"candidates"`
	Comparison ArenaComparison   `json:"comparison"`
	Envelope   ArenaEnvelopeShow `json:"envelope"`
	Reasoning  string            `json:"reasoning"`
}

// arenaCandidateSpec is the static exhibit input: what to send and what
// the row is probing, before judgment runs.
type arenaCandidateSpec struct {
	id      string
	label   string
	expects string
	body    string
	note    string
}

// arenaRoster is the candidate set, in teaching order: the happy
// baseline, the decode layer's four refusals, the rule layer's five
// catches, the zero-value pair, and the collect-every-issue finale.
// Bodies are plain string literals (one built with Repeat for the
// over-long title) so the wire is byte-stable across calls.
func arenaRoster() []arenaCandidateSpec {
	tooLongTitle := strings.Repeat("word ", 17) + "end"
	return []arenaCandidateSpec{
		{
			id:      "valid-complete",
			label:   "A well-formed submission",
			expects: "Every field present, every rule satisfied — the shape every other row deviates from.",
			body:    `{"title":"Loved the goroutines topic","body":"The race demo finally made -race click.","score":5,"email":"reader@example.com"}`,
			note:    "No coercion anywhere: the decoded form echoes the body's values exactly — Go's decoder accepts or refuses, it never fixes.",
		},
		{
			id:      "broken-json-syntax",
			label:   "Truncated JSON",
			expects: "Layer one refuses before any field exists to judge: syntax is the first gate.",
			body:    `{"title": "Dangling`,
		},
		{
			id:      "wrong-type-score",
			label:   "A string where an int lives",
			expects: "The declared type IS a validation rule: \"five\" cannot land in Score int.",
			body:    `{"title":"Fine","body":"","score":"five","email":"reader@example.com"}`,
		},
		{
			id:      "unknown-field-priority",
			label:   "An unknown field",
			expects: "DisallowUnknownFields refuses \"priority\" — strict for create-shaped bodies.",
			body:    `{"title":"Fine","body":"","score":4,"email":"reader@example.com","priority":"urgent"}`,
			note:    "The guide's own PATCH /api/demo/scenario deliberately IGNORES unknown keys (fastapi parity) — strict vs lax is a per-endpoint choice, not a law.",
		},
		{
			id:      "trailing-second-object",
			label:   "Two JSON documents",
			expects: "The one-value discipline: a body is one document, not a stream.",
			body:    `{"title":"Fine","body":"","score":4,"email":"reader@example.com"} {"title":"Again"}`,
		},
		{
			id:      "empty-required-title",
			label:   "Required text, present but empty",
			expects: "\"\" decodes fine — then the required rule judges it. Empty is data, not absence.",
			body:    `{"title":"","body":"","score":5,"email":"reader@example.com"}`,
		},
		{
			id:      "too-long-title",
			label:   "Title over the 80-rune bound",
			expects: "A length bound the decoder cannot express — layer two counts runes.",
			body:    fmt.Sprintf(`{"title":%q,"body":"","score":4,"email":"reader@example.com"}`, tooLongTitle),
			note:    "88 runes where 80 are allowed; the Got column carries the count, not the whole wall of text.",
		},
		{
			id:      "negative-score",
			label:   "Score below the range",
			expects: "A number can be the wrong number: range checks are rule-layer work.",
			body:    `{"title":"Fine","body":"","score":-3,"email":"reader@example.com"}`,
		},
		{
			id:      "bad-email-shape",
			label:   "Email without the @ shape",
			expects: "A shape check the type system cannot state: strings carry syntax too.",
			body:    `{"title":"Fine","body":"","score":4,"email":"not-an-email"}`,
		},
		{
			id:      "absent-required-score",
			label:   "Required score simply omitted",
			expects: "encoding/json cannot tell absent from explicit 0 — the zero-value blur.",
			body:    `{"title":"Fine","body":"","email":"reader@example.com"}`,
			note:    "The blur is real at every layer (a library's required rejects 0 the same way); the repo's ScenarioPatch answers it with POINTER fields — present-only semantics.",
		},
		{
			id:      "absent-optional-body",
			label:   "Optional body omitted",
			expects: "The other side of the blur: an optional field's absence is its zero value, and that is fine.",
			body:    `{"title":"Short and sweet","score":4,"email":"reader@example.com"}`,
			note:    "Body decodes to \"\" — the default — and the response echoes it; optional-with-zero-default is the cheapest optional there is.",
		},
		{
			id:      "collect-every-issue",
			label:   "Three broken fields at once",
			expects: "The collector discipline: one submission, every fixable field reported in one 422.",
			body:    `{"title":"","body":"","score":9,"email":"nope"}`,
			note:    "The envelope exhibit below is THIS candidate's real 422 — three issues in the details array, produced by core.WriteError.",
		},
	}
}

// judge runs one candidate through both layers and composes the
// response, using the repo's own error taxonomy and central writer at
// every refusal — the same three shapes a real handler produces: decode
// failure (details: {body: reason}), rule failure (details: the issue
// array), success (the validated form).
func judge(spec arenaCandidateSpec) ArenaCandidate {
	candidate := ArenaCandidate{
		ID:      spec.id,
		Label:   spec.label,
		Expects: spec.expects,
		Body:    spec.body,
		Note:    spec.note,
	}

	form, err := DecodeStrictFeedback([]byte(spec.body))
	if err != nil {
		candidate.Decode = ArenaDecodeOutcome{
			Status:    decodeRejected,
			ErrorType: fmt.Sprintf("%T", err),
			Error:     err.Error(),
		}
		candidate.Validate = ArenaRulesOutcome{Status: rulesSkipped}
		// The decode-failure 422 mirrors decodeScenarioPatch exactly: a
		// ValidationFailed error whose details carry the body reason.
		status, envelope := envelopeFor(core.ValidationFailed(
			"Invalid feedback body.", map[string]string{"body": err.Error()},
		))
		candidate.Response = ArenaResponse{Status: status, Envelope: envelope}
		return candidate
	}

	candidate.Decode = ArenaDecodeOutcome{Status: decodeAccepted}
	issues := ValidateFeedback(form)
	if len(issues) == 0 {
		candidate.Validate = ArenaRulesOutcome{Status: rulesValidated}
		candidate.Response = ArenaResponse{Status: http.StatusOK, Validated: &form}
		return candidate
	}

	candidate.Validate = ArenaRulesOutcome{Status: rulesRejected, Issues: issues}
	status, envelope := envelopeFor(core.ValidationFailed(
		"Feedback submission failed validation.", issues,
	))
	candidate.Response = ArenaResponse{Status: status, Envelope: envelope}
	return candidate
}

// arenaSubject states the contract once, beside the code that enforces
// it — the field list a client can render without reading Go.
func arenaSubject() ArenaSubject {
	return ArenaSubject{
		Name:   "FeedbackForm",
		Source: "server/internal/domain/demos/validation_arena.go",
		Fields: []ArenaFieldSpec{
			{Name: "title", Type: "string", Rule: "required; trimmed non-blank; at most 80 runes", Required: true},
			{Name: "body", Type: "string", Rule: "optional; at most 280 runes when present; zero value \"\" is the default", Required: false},
			{Name: "score", Type: "int", Rule: "required; whole number 1–5; absent decodes to 0 and fails the same gate", Required: true},
			{Name: "email", Type: "string", Rule: "required; one @ with non-empty text on both sides", Required: true},
		},
	}
}

// arenaLayers is the three-layer stack the topic teaches, one card each.
func arenaLayers() []ArenaLayerCard {
	return []ArenaLayerCard{
		{
			ID:      "decode",
			Title:   "Layer one — decode discipline",
			Tool:    "encoding/json + net/http",
			Summary: "Shape and types: one JSON value, known fields only, values of the declared types, a bounded body. What the struct refuses never becomes a field issue — it IS the refusal.",
		},
		{
			ID:      "rules",
			Title:   "Layer two — hand-rolled field rules",
			Tool:    "plain Go functions",
			Summary: "The backbone: length bounds, ranges, shape and cross-field checks as code you can read, step through, and unit-test — collecting every issue into the details array.",
		},
		{
			ID:      "library",
			Title:   "Layer three — the tag ecosystem",
			Tool:    "go-playground/validator (by comparison)",
			Summary: "Struct tags drive a reflection engine: required,max=80,gte=1,lte=5,email replace the hand-rolled branches at the price of a dependency and tag-language debugging. This guide hand-rolls.",
		},
	}
}

// arenaComparison walks the tagged counterpart's fields via reflect —
// tags are runtime-visible metadata, the closest thing Go has to
// annotations — and pairs each tag with the rule that does the same job
// in code. Imported is false and stays false: no validator dependency
// enters this module for the comparison's sake.
func arenaComparison() ArenaComparison {
	handRolled := map[string]string{
		"title": "trimmed non-blank, at most 80 runes — ValidateFeedback's first switch",
		"body":  "optional; at most 280 runes when present",
		"score": "whole number 1–5 — absent and explicit 0 fail the same gate",
		"email": "one @ with non-empty text on both sides",
	}
	libraryRule := map[string]string{
		"title": "required rejects the zero value, max=80 bounds the rune count — the same contract as tag data",
		"body":  "max=280 only — no required tag, so the zero value passes exactly like the hand-rolled optional",
		"score": "required,gte=1,lte=5 — a numeric range; required still means non-zero, the same blur",
		"email": "email — the library's RFC-flavored grammar, stricter than the arena's one-@ shape check",
	}
	t := reflect.TypeOf(taggedFeedback{})
	rows := make([]ArenaTagRow, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		wire := strings.Split(field.Tag.Get("json"), ",")[0]
		rows = append(rows, ArenaTagRow{
			Field:       wire,
			Tag:         field.Tag.Get("validate"),
			LibraryRule: libraryRule[wire],
			HandRolled:  handRolled[wire],
		})
	}
	return ArenaComparison{
		Library:  "github.com/go-playground/validator/v10",
		Imported: false,
		Rows:     rows,
		Note: "The tags are inert without a library that reads them — no validator package is imported " +
			"here. That is the honest trade: the library buys declarative rules and a rich built-in set, " +
			"and charges a dependency, reflection semantics you must learn (string bounds count runes; " +
			"required means non-zero), and errors you map yourself. The hand-rolled backbone keeps the " +
			"rules in the language you already debug.",
	}
}

// arenaEnvelope exhibits the collect-every-issue candidate's real 422 —
// produced by core.WriteError, not reconstructed — with one anatomy row
// per piece of the shape.
func arenaEnvelope(candidates []ArenaCandidate) ArenaEnvelopeShow {
	var shown *ArenaCandidate
	for i := range candidates {
		if candidates[i].ID == "collect-every-issue" {
			shown = &candidates[i]
			break
		}
	}
	// The roster is a fixed literal that includes the candidate; its
	// absence is a programmer error, not an input condition.
	if shown == nil {
		panic("validation arena: roster lost its collect-every-issue candidate")
	}
	return ArenaEnvelopeShow{
		CandidateID: shown.ID,
		Status:      shown.Response.Status,
		Body:        shown.Response.Envelope,
		Anatomy: []ArenaAnatomyRow{
			{Path: "status line", Value: "422 Unprocessable Entity", Why: "the family's validation status — the express sibling answers 400 for the same bucket; pick one and hold it"},
			{Path: "error.code", Value: "validation_failed", Why: "the taxonomy's snake_case bucket, one of eight — nothing in the app invents its own code or status"},
			{Path: "error.message", Value: "one human sentence", Why: "safe to show any user; internals ride the logs with the request id, never the body"},
			{Path: "error.requestId", Value: arenaRequestID, Why: "the same id as the x-request-id header and the server's warn log — the join key for debugging"},
			{Path: "error.details", Value: "the ArenaIssue array", Why: "structured context: field, rule, got. Decode failures carry {body: reason} instead; omitted entirely when there is nothing to say — never null"},
		},
	}
}

// BuildValidationArenaPayload assembles the whole demo payload from the
// static roster and the real validation machinery. It is pure: same
// call, same bytes, no inputs, no I/O — the handler layer adds only
// transport.
func BuildValidationArenaPayload() ValidationArenaPayload {
	specs := arenaRoster()
	candidates := make([]ArenaCandidate, 0, len(specs))
	for _, spec := range specs {
		candidates = append(candidates, judge(spec))
	}
	return ValidationArenaPayload{
		Subject:    arenaSubject(),
		Layers:     arenaLayers(),
		Candidates: candidates,
		Comparison: arenaComparison(),
		Envelope:   arenaEnvelope(candidates),
		Reasoning: "Twelve candidate bodies, each run through the real two-layer pipeline: the decode " +
			"discipline refuses broken syntax, unknown fields, trailing documents, and wrong types " +
			"(with the decoder's own error in the details); the hand-rolled rules catch everything " +
			"shape and types cannot see — empties, bounds, ranges, and email shape — collecting every " +
			"issue per submission. Every 422 body on this page was produced by the server's central " +
			"error writer, so what you read here is what the wire answers. There is no coercion column: " +
			"encoding/json never coerces, so the fastapi arena's original-to-coerced story collapses " +
			"into accepted-or-refused — the Go boundary is strict by construction.",
	}
}
