package demos

import (
	"reflect"
	"strings"
	"testing"

	"guide/internal/content"
)

// The payload is computed live from real guide objects; these tests pin
// the facts the go-language topic's prose claims, so a refactor that
// changes what reflect sees fails HERE first, next to the claim.

func payload() RuntimeTypesPayload {
	return BuildRuntimeTypesPayload()
}

func TestInspectionsCoverTheTeachingKinds(t *testing.T) {
	rows := payload().Inspections
	if len(rows) != 9 {
		t.Fatalf("inspections = %d rows, want 9", len(rows))
	}
	kindOf := func(expression string) string {
		for _, row := range rows {
			if row.Expression == expression {
				return row.Kind
			}
		}
		t.Fatalf("no inspection row for %q", expression)
		return ""
	}
	wantKind := map[string]string{
		`content.Topic{}`:      "struct",
		`content.NewTextBlock`: "func",
		`[]content.Block{}`:    "slice",
		`map[string]bool{}`:    "map",
		`"go-language"`:        "string",
		`43`:                   "int",
		`reflect.TypeOf("")`:   "ptr",
	}
	for expression, kind := range wantKind {
		if got := kindOf(expression); got != kind {
			t.Errorf("%s kind = %q, want %q", expression, got, kind)
		}
	}
	for _, row := range rows {
		if row.Type == "" || row.SizeBytes <= 0 {
			t.Errorf("row %q incomplete: %+v", row.Expression, row)
		}
	}
}

func TestInspectionsNameTheGuidePackages(t *testing.T) {
	rows := payload().Inspections
	var topicPkg, errPkg string
	for _, row := range rows {
		switch row.Expression {
		case `content.Topic{}`:
			topicPkg = row.PkgPath
		case `core.Error{}`:
			errPkg = row.PkgPath
		}
	}
	if topicPkg != "guide/internal/content" || errPkg != "guide/internal/core" {
		t.Errorf("pkg paths = %q / %q, want the guide's content and core packages", topicPkg, errPkg)
	}
}

func TestFieldsWalkTheRealSectionStruct(t *testing.T) {
	rows := payload().Fields
	if len(rows) != reflect.TypeOf(content.Section{}).NumField() || len(rows) != 4 {
		t.Fatalf("fields = %d rows, want Section's 4 fields", len(rows))
	}
	want := []FieldRow{
		{Name: "ID", Type: "string", JSONTag: "id"},
		{Name: "Title", Type: "string", JSONTag: "title"},
		{Name: "Lead", Type: "string", JSONTag: "lead,omitempty"},
		{Name: "Blocks", Type: "[]content.Block", JSONTag: "blocks"},
	}
	for i, row := range rows {
		if row != want[i] {
			t.Errorf("field %d = %+v, want %+v", i, row, want[i])
		}
	}
}

func TestAssertionsResolveRealBlocksAndMissHonestly(t *testing.T) {
	rows := payload().Assertions
	if len(rows) != 5 {
		t.Fatalf("assertions = %d rows, want 4 switches + 1 miss", len(rows))
	}
	wantTypes := []string{
		"content.TextBlock", "content.CodeBlock", "content.DemoBlock", "content.ListBlock",
	}
	for i, want := range wantTypes {
		if rows[i].DynamicType != want || !rows[i].OK {
			t.Errorf("assertion %d = %+v, want dynamic %s ok=true", i, rows[i], want)
		}
	}
	if rows[2].BlockID != "go:runtime-types" {
		t.Errorf("demo block id = %q, want go:runtime-types", rows[2].BlockID)
	}
	if rows[4].OK {
		t.Errorf("comma-ok assertion on a TextBlock slot must report ok=false: %+v", rows[4])
	}
}

func TestZeroValuesAreAllReallyZero(t *testing.T) {
	rows := payload().ZeroValues
	if len(rows) != 6 {
		t.Fatalf("zero values = %d rows, want 6", len(rows))
	}
	for _, row := range rows {
		if !row.IsZero || row.Display == "" || row.Type == "" {
			t.Errorf("zero row incomplete: %+v", row)
		}
	}
}

func TestEnforcementCarriesTheRealCompilerVerdict(t *testing.T) {
	run := payload().Enforcement
	if run.Function != "repeat" || run.Signature != "func repeat(text string, times int) string" {
		t.Errorf("enforcement names the wrong exhibit: %+v", run)
	}
	if !strings.HasPrefix(run.CompilerVerdict, "cannot use 3.0") ||
		!strings.Contains(run.CompilerVerdict, "as string value in argument to repeat") {
		t.Errorf("compiler verdict drifted from the toolchain's words: %q", run.CompilerVerdict)
	}
	if !strings.Contains(run.RuntimeOutcome, `"gogogo"`) {
		t.Errorf("typed call proof missing gogogo: %q", run.RuntimeOutcome)
	}
}

func TestBoundaryIsTheJsonTypeRejection(t *testing.T) {
	check := payload().Boundary
	if check.Outcome != "rejected" || check.ErrorType != "*json.UnmarshalTypeError" {
		t.Errorf("boundary = %+v, want json rejection via *json.UnmarshalTypeError", check)
	}
	if !strings.Contains(check.Message, "cannot unmarshal number") ||
		!strings.Contains(check.Message, "TextBlock.id") {
		t.Errorf("boundary message does not name the field: %q", check.Message)
	}
}

// The tables' teaching sentences are server-taught prose riding the
// payload — the client renders them, never authors them. These tests pin
// the contract so the prose cannot silently disappear from the wire.
func TestTeachingNotesAreServedWithTheTables(t *testing.T) {
	p := payload()
	if !strings.Contains(p.FieldsNote, "read at runtime by reflect") ||
		!strings.Contains(p.FieldsNote, "encoding/json") {
		t.Errorf("fieldsNote drifted from the json-tag teaching: %q", p.FieldsNote)
	}
	if !strings.Contains(p.AssertionsNote, "false instead of panicking") ||
		!strings.Contains(p.AssertionsNote, "dynamic type") {
		t.Errorf("assertionsNote drifted from the comma-ok teaching: %q", p.AssertionsNote)
	}
}

func TestPayloadIsPure(t *testing.T) {
	first, second := BuildRuntimeTypesPayload(), BuildRuntimeTypesPayload()
	if !reflect.DeepEqual(first, second) {
		t.Error("builder must be pure: two calls returned different payloads")
	}
}
