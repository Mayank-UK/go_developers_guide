// Package demos holds the payload builders behind the guide's live demo
// endpoints: pure functions over real guide objects — no I/O, no mutable
// state — so the thin presentation handlers have nothing to do but map the
// result onto the wire. This module is also the SUBJECT of the go-language
// topic's runtime-types demo: the topic quotes it as real code, and the
// demo reflects on the very objects it builds.
//
// TEACHING-EXCEPTION (layering): the family rule is presentation →
// domain ← data, and this file lives in domain/demos while importing the
// content registry — the exhibit reflects on the REAL content types the
// server serves (Topic, Section, the block kinds), not copies. The same
// exemption the family grants teaching-exhibit handlers, marked like
// its siblings.
package demos

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"guide/internal/content"
	"guide/internal/core"
)

// repeat is the demo's enforcement exhibit: a fully typed function the
// compiler checks at every call site. The wrong-typed call repeat(3.0, 2)
// does not merely misbehave — it does not compile, which is exactly the
// contrast with dynamically typed languages this demo teaches. The payload
// reports the toolchain's own verdict on that call, verbatim.
func repeat(text string, times int) string {
	return strings.Repeat(text, times)
}

// compilerVerdict is quoted verbatim from `go build` (go1.27) for the call
// repeat(3.0, 2) — the demo cannot run a rejected call by definition, so
// the payload carries the compiler's words instead.
const compilerVerdict = "cannot use 3.0 (untyped float constant 3) as string value in argument to repeat"

// The two payload faces' teaching notes — the sentences that read the
// tables below. The client renders prose, the server teaches it: these
// live here (not hardcoded in the view) so the teaching rides the same
// wire contract as the data it explains.
const fieldsNote = "The tag column is read at runtime by reflect — it is where " +
	"this API's camelCase wire names live, and how encoding/json finds them."

const assertionsNote = "The comma-ok row reports false instead of panicking — " +
	"the dynamic type is data the runtime carries, not a guess it commits to."

// TypeRow is one value inspected through reflect: what the runtime knows
// about a real guide object's type.
type TypeRow struct {
	Expression string `json:"expression"`
	Type       string `json:"type"`
	Kind       string `json:"kind"`
	PkgPath    string `json:"pkgPath"`
	SizeBytes  int    `json:"sizeBytes"`
}

// FieldRow is one struct field the reflect walk reports, with the JSON tag
// the wire actually serves — proof that struct tags are runtime-visible
// metadata, the closest thing Go has to annotations.
type FieldRow struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	JSONTag string `json:"jsonTag"`
}

// AssertionRow is one dynamic-type lookup on an interface value: a type
// switch case (or comma-ok assertion) against a real content block.
type AssertionRow struct {
	Expression  string `json:"expression"`
	DynamicType string `json:"dynamicType"`
	OK          bool   `json:"ok"`
	BlockID     string `json:"blockID"`
}

// ZeroValueRow is one declared variable at its zero value, with the
// reflect-confirmed proof that it really is zero.
type ZeroValueRow struct {
	Declaration string `json:"declaration"`
	Type        string `json:"type"`
	Display     string `json:"display"`
	IsZero      bool   `json:"isZero"`
}

// EnforcementRun is the compile-time proof: the call the compiler rejects,
// its verdict, and what that leaves for runtime to do (nothing).
type EnforcementRun struct {
	Function        string `json:"function"`
	Signature       string `json:"signature"`
	Call            string `json:"call"`
	CompilerVerdict string `json:"compilerVerdict"`
	RuntimeOutcome  string `json:"runtimeOutcome"`
}

// BoundaryCheck is the runtime counterpoint: encoding/json IS a type
// checker — at the decode boundary, on the same struct the API serves.
type BoundaryCheck struct {
	DecodeTarget string `json:"decodeTarget"`
	BadInput     string `json:"badInput"`
	Outcome      string `json:"outcome"`
	ErrorType    string `json:"errorType"`
	Message      string `json:"message"`
}

// RuntimeTypesPayload is the wire shape of GET /api/demo/go/runtime-types
// (camelCase JSON) — the go-language topic's inline demo, adapted from the
// fastapi guide's py:runtime-types to what "types at runtime" means in Go.
// FieldsNote and AssertionsNote are the tables' teaching sentences, served
// beside the rows they read (the client renders them, never authors them).
type RuntimeTypesPayload struct {
	Inspections    []TypeRow      `json:"inspections"`
	Fields         []FieldRow     `json:"fields"`
	Assertions     []AssertionRow `json:"assertions"`
	ZeroValues     []ZeroValueRow `json:"zeroValues"`
	Enforcement    EnforcementRun `json:"enforcement"`
	Boundary       BoundaryCheck  `json:"boundary"`
	FieldsNote     string         `json:"fieldsNote"`
	AssertionsNote string         `json:"assertionsNote"`
}

// typeRow reflects one value. reflect.TypeOf's parameter is any — the
// empty interface — and what it hands back is the value's DYNAMIC type:
// exactly what an interface value carries at runtime.
func typeRow(expression string, value any) TypeRow {
	t := reflect.TypeOf(value)
	return TypeRow{
		Expression: expression,
		Type:       t.String(),
		Kind:       t.Kind().String(),
		PkgPath:    t.PkgPath(),
		SizeBytes:  int(t.Size()),
	}
}

// inspections walks real guide values of deliberately different kinds.
func inspections() []TypeRow {
	return []TypeRow{
		typeRow(`content.Topic{}`, content.Topic{}),
		typeRow(`content.Section{}`, content.Section{}),
		typeRow(`core.Error{}`, core.Error{}),
		typeRow(`content.NewTextBlock`, content.NewTextBlock),
		typeRow(`[]content.Block{}`, []content.Block{}),
		typeRow(`map[string]bool{}`, map[string]bool{}),
		typeRow(`"go-language"`, "go-language"),
		typeRow(`43`, 43),
		typeRow(`reflect.TypeOf("")`, reflect.TypeOf("")),
	}
}

// fields reflects the very struct the detail route serves on the wire:
// every field, its type, and the json tag that names it in API responses.
func fields() []FieldRow {
	t := reflect.TypeOf(content.Section{})
	rows := make([]FieldRow, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		rows = append(rows, FieldRow{
			Name:    field.Name,
			Type:    field.Type.String(),
			JSONTag: field.Tag.Get("json"),
		})
	}
	return rows
}

// blockIDBySwitch runs the same type switch the content model's blockIDOf
// runs (that one is unexported — the seal, not a secret). The demo walks
// its own copy over real blocks to show what an interface value carries.
func blockIDBySwitch(block content.Block) string {
	switch b := block.(type) {
	case content.TextBlock:
		return b.ID
	case content.CodeBlock:
		return b.ID
	case content.DemoBlock:
		return b.ID
	case content.ListBlock:
		return b.ID
	default:
		return ""
	}
}

// assertions loads four REAL blocks (constructors and all) into one
// []content.Block, then shows what each interface slot holds at runtime —
// plus one comma-ok assertion that does not match, which reports false
// instead of panicking.
func assertions() []AssertionRow {
	// Constructor errors are impossible on these static, known-valid
	// arguments; the zero values would surface instantly in the rows below.
	text, _ := content.NewTextBlock("demo:text", "One paragraph of prose.")
	code, _ := content.NewCodeBlock("demo:code", "bash", "", "", "curl localhost:8100/api/health")
	demo, _ := content.NewDemoBlock("go:runtime-types", "Types at runtime", "")
	list, _ := content.NewListBlock("demo:list", false, []string{"One — a.", "Two — b.", "Three — c."})
	blocks := []content.Block{text, code, demo, list}

	rows := make([]AssertionRow, 0, len(blocks)+1)
	for i, block := range blocks {
		rows = append(rows, AssertionRow{
			Expression:  fmt.Sprintf("blocks[%d].(type)", i),
			DynamicType: reflect.TypeOf(block).String(),
			OK:          true,
			BlockID:     blockIDBySwitch(block),
		})
	}
	_, ok := blocks[0].(content.CodeBlock)
	rows = append(rows, AssertionRow{
		Expression:  "blocks[0].(content.CodeBlock)",
		DynamicType: "content.CodeBlock",
		OK:          ok,
		BlockID:     "",
	})
	return rows
}

// zeroDisplay renders the zero value of a type the way Go source writes
// it — the table's "what you get if you just declare it" column.
func zeroDisplay(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return `""`
	case reflect.Bool:
		return "false"
	case reflect.Struct:
		return "{}"
	default:
		// Every remaining kind — numbers, slices, maps, pointers, funcs —
		// zeroes to a zero number or a nil reference.
		return "nil or 0"
	}
}

// zeroValues declares variables of real guide types and NEVER initializes
// them: every one is already its zero value, and reflect's IsZero confirms.
func zeroValues() []ZeroValueRow {
	var (
		id       string
		order    int
		present  bool
		sections []content.Section
		byID     map[string]content.Topic
		topic    content.Topic
	)
	rows := []ZeroValueRow{
		{Declaration: "var id string", Type: reflect.TypeOf(id).String(), Display: zeroDisplay(reflect.TypeOf(id)), IsZero: reflect.ValueOf(id).IsZero()},
		{Declaration: "var order int", Type: reflect.TypeOf(order).String(), Display: zeroDisplay(reflect.TypeOf(order)), IsZero: reflect.ValueOf(order).IsZero()},
		{Declaration: "var present bool", Type: reflect.TypeOf(present).String(), Display: zeroDisplay(reflect.TypeOf(present)), IsZero: reflect.ValueOf(present).IsZero()},
		{Declaration: "var sections []content.Section", Type: reflect.TypeOf(sections).String(), Display: zeroDisplay(reflect.TypeOf(sections)), IsZero: reflect.ValueOf(sections).IsZero()},
		{Declaration: "var byID map[string]content.Topic", Type: reflect.TypeOf(byID).String(), Display: zeroDisplay(reflect.TypeOf(byID)), IsZero: reflect.ValueOf(byID).IsZero()},
		{Declaration: "var topic content.Topic", Type: reflect.TypeOf(topic).String(), Display: zeroDisplay(reflect.TypeOf(topic)), IsZero: reflect.ValueOf(topic).IsZero()},
	}
	return rows
}

// enforcement pairs the rejected call with its verbatim compiler verdict,
// and proves the correctly-typed call is the only one that can run.
func enforcement() EnforcementRun {
	value := repeat("go", 3)
	return EnforcementRun{
		Function:        "repeat",
		Signature:       "func repeat(text string, times int) string",
		Call:            "repeat(3.0, 2)",
		CompilerVerdict: compilerVerdict,
		RuntimeOutcome: fmt.Sprintf(
			"none — the program containing the call never builds; the typed call repeat(\"go\", 3) returns %q", value,
		),
	}
}

// boundary hands encoding/json a number where content.TextBlock's ID field
// declares a string. The decoder refuses — runtime type checking, exactly
// where untyped outside data meets a typed struct.
func boundary() BoundaryCheck {
	var block content.TextBlock
	err := json.Unmarshal([]byte(`{"kind":"text","id":42,"text":"Contraband"}`), &block)
	if err == nil {
		// Unreachable while encoding/json keeps its contract: a number
		// never lands in a string field. If the contract ever breaks, a
		// panic naming this demo is worth more than a silently wrong table.
		panic("runtime-types demo: json accepted a number into a string field")
	}
	return BoundaryCheck{
		DecodeTarget: "content.TextBlock",
		BadInput:     `{"kind":"text","id":42,"text":"Contraband"}`,
		Outcome:      "rejected",
		ErrorType:    fmt.Sprintf("%T", err),
		Message:      err.Error(),
	}
}

// BuildRuntimeTypesPayload assembles the whole demo payload from real
// guide objects. It is pure: same call, same payload, no inputs, no I/O —
// the handler layer adds only transport.
func BuildRuntimeTypesPayload() RuntimeTypesPayload {
	return RuntimeTypesPayload{
		Inspections:    inspections(),
		Fields:         fields(),
		Assertions:     assertions(),
		ZeroValues:     zeroValues(),
		Enforcement:    enforcement(),
		Boundary:       boundary(),
		FieldsNote:     fieldsNote,
		AssertionsNote: assertionsNote,
	}
}
