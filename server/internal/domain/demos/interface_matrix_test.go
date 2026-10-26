package demos

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// The matrix's contract: every ✓/✗ the payload serves is a REAL method-set
// fact. These tests pin the full truth table the topic's prose claims, so a
// Go upgrade or a repo refactor that changes any method set fails HERE,
// next to the claim — the matrix may drift only loudly.

func ifaceMatrixPayload() InterfaceMatrixPayload {
	return BuildInterfaceMatrixPayload()
}

// wantIfaceMatrix is the pinned truth table: row expression → interface →
// verdict. Column order: fmt.Stringer, error, io.Reader, http.Handler,
// content.Block.
var wantIfaceMatrix = map[string][5]bool{
	`content.TextBlock{}`: {false, false, false, false, true},
	`content.CodeBlock{}`: {false, false, false, false, true},
	`content.Topic{}`:     {false, false, false, false, false},
	`*core.Error`:         {false, true, false, false, false},
	`core.Error`:          {false, false, false, false, false},
	`*bytes.Buffer`:       {true, false, true, false, false},
	`*strings.Reader`:     {false, false, true, false, false},
	`time.Duration`:       {true, false, false, false, false},
	`http.HandlerFunc`:    {false, false, false, true, false},
	`*core.ScenarioStore`: {false, false, false, false, false},
	`content.Registry`:    {false, false, false, false, false},
}

func TestIfaceMatrixColumnsAreTheFiveInterfaces(t *testing.T) {
	cols := ifaceMatrixPayload().Interfaces
	if len(cols) != 5 {
		t.Fatalf("columns = %d, want 5", len(cols))
	}
	want := []struct{ name, method, pkg string }{
		{"fmt.Stringer", "String() string", "fmt"},
		{"error", "Error() string", "builtin"},
		{"io.Reader", "Read([]byte) (int, error)", "io"},
		{"http.Handler", "ServeHTTP(http.ResponseWriter, *http.Request)", "net/http"},
		{"content.Block", "blockKind() string", "guide/internal/content"},
	}
	for i, w := range want {
		if cols[i].Name != w.name || cols[i].Method != w.method || cols[i].Pkg != w.pkg {
			t.Errorf("column %d = %+v, want %s / %s / %s", i, cols[i], w.name, w.method, w.pkg)
		}
	}
}

func TestIfaceMatrixVerdictsMatchThePinnedTruthTable(t *testing.T) {
	rows := ifaceMatrixPayload().Rows
	if len(rows) != len(wantIfaceMatrix) {
		t.Fatalf("rows = %d, want %d", len(rows), len(wantIfaceMatrix))
	}
	for _, row := range rows {
		want, ok := wantIfaceMatrix[row.Expression]
		if !ok {
			t.Errorf("unexpected row %q", row.Expression)
			continue
		}
		if len(row.Cells) != 5 {
			t.Fatalf("row %q has %d cells, want 5", row.Expression, len(row.Cells))
		}
		for i, cell := range row.Cells {
			if cell.Satisfied != want[i] {
				t.Errorf("%s × %s = %v, want %v", row.Expression, cell.Iface, cell.Satisfied, want[i])
			}
		}
	}
}

func TestIfaceMatrixMethodSetsAreTheRealOnes(t *testing.T) {
	rows := ifaceMatrixPayload().Rows
	byExpression := make(map[string]SatisfactionRow, len(rows))
	for _, row := range rows {
		byExpression[row.Expression] = row
	}
	wantMethods := map[string][]string{
		`*core.Error`:         {"Error", "Is", "Unwrap"},
		`core.Error`:          {},
		`*core.ScenarioStore`: {"Snapshot", "Update"},
		`content.Registry`:    {"Groups", "Summaries", "TopicByID"},
		`http.HandlerFunc`:    {"ServeHTTP"},
		`content.Topic{}`:     {},
	}
	for expression, want := range wantMethods {
		row, ok := byExpression[expression]
		if !ok {
			t.Fatalf("row %q missing", expression)
		}
		if strings.Join(row.Methods, ",") != strings.Join(want, ",") {
			t.Errorf("%s methods = %v, want %v", expression, row.Methods, want)
		}
	}
}

func TestIfaceMatrixReflectIsBlindToTheSealedBlockMethod(t *testing.T) {
	// The juxtaposition the topic's prose claims: content.TextBlock
	// satisfies content.Block (compile-time proven, Implements-verified)
	// yet reflect lists ZERO methods for it — blockKind is unexported, and
	// reflect's method listing only ever sees exported methods.
	rows := ifaceMatrixPayload().Rows
	for _, row := range rows {
		if row.Expression != `content.TextBlock{}` {
			continue
		}
		if len(row.Methods) != 0 {
			t.Errorf("TextBlock exported methods = %v, want none (blockKind is unexported)", row.Methods)
		}
		blockCell := row.Cells[len(row.Cells)-1]
		if blockCell.Iface != "content.Block" || !blockCell.Satisfied {
			t.Errorf("TextBlock × content.Block = %+v, want satisfied", blockCell)
		}
	}
	if _, ok := reflect.TypeOf(struct{}{}).MethodByName("blockKind"); ok {
		t.Error("sanity: method probe misbehaved")
	}
}

func TestIfaceMatrixEmbeddingsSatisfyByPromotion(t *testing.T) {
	rows := ifaceMatrixPayload().Embeddings
	if len(rows) != 3 {
		t.Fatalf("embeddings = %d rows, want 3", len(rows))
	}
	for _, row := range rows {
		if !row.Satisfied {
			t.Errorf("embedding %q must satisfy %s — promotion feeds the method set", row.Expression, row.Iface)
		}
	}
	if rows[0].Type != "demos.taggedReader" || rows[0].Iface != "io.Reader" {
		t.Errorf("first embedding = %+v, want demos.taggedReader × io.Reader", rows[0])
	}
	if rows[1].Type != "demos.delegatingHandler" || rows[1].Iface != "http.Handler" {
		t.Errorf("second embedding = %+v, want demos.delegatingHandler × http.Handler", rows[1])
	}
	if rows[2].Type != "*bytes.Buffer" || rows[2].Iface != "io.ReadWriter" {
		t.Errorf("third embedding = %+v, want *bytes.Buffer × io.ReadWriter", rows[2])
	}
}

func TestIfaceMatrixTypeSwitchRowsProveFirstMatchWins(t *testing.T) {
	rows := ifaceMatrixPayload().TypeSwitch
	if len(rows) != 6 {
		t.Fatalf("type switch = %d rows, want 6", len(rows))
	}
	byInput := make(map[string]SwitchCaseRow, len(rows))
	for _, row := range rows {
		byInput[row.Input] = row
	}
	if r := byInput[`content.Topic{}`]; r.MatchedCase != "default" || r.Result != "unhandled content.Topic" {
		t.Errorf("default case = %+v, want unhandled content.Topic", r)
	}
	if r := byInput[`time.Duration`]; r.MatchedCase != "case fmt.Stringer" || r.Result != "fmt.Stringer case — 1m30s" {
		t.Errorf("duration case = %+v, want the Stringer hit with 1m30s", r)
	}
	// *bytes.Buffer satisfies BOTH io.Reader and fmt.Stringer; the case
	// order settles it, and the row must say so.
	buffer := byInput[`*bytes.Buffer`]
	if buffer.MatchedCase != "case io.Reader" || !strings.Contains(buffer.Note, "first match wins") {
		t.Errorf("buffer case = %+v, want io.Reader first + the order note", buffer)
	}
	if r := byInput[`*core.Error`]; !strings.Contains(r.Result, "not_found") {
		t.Errorf("error case = %+v, want the kind in the result", r)
	}
}

func TestIfaceMatrixCompileProofsOccurInThisFileAndPinTrueCells(t *testing.T) {
	source, err := os.ReadFile("interface_matrix.go")
	if err != nil {
		t.Fatalf("read own source: %v", err)
	}
	normalize := func(line string) string { return strings.Join(strings.Fields(line), " ") }
	fileLines := make(map[string]bool)
	for _, line := range strings.Split(string(source), "\n") {
		fileLines[normalize(line)] = true
	}

	// True cells, computed live, keyed by the proof's pins suffix.
	rows := ifaceMatrixPayload().Rows
	trueCells := make(map[string]bool)
	for _, row := range rows {
		for _, cell := range row.Cells {
			if cell.Satisfied {
				trueCells[cell.Iface+" × "+row.Expression] = true
			}
		}
	}

	// Cells proven outside the matrix proper: the embedding exhibits
	// (promotion) and the composed io.ReadWriter row — each pinned by its
	// own test below/above.
	pinnedElsewhere := map[string]bool{
		"io.ReadWriter × *bytes.Buffer": true,
	}

	proofs := ifaceMatrixPayload().CompileProofs
	if len(proofs) != 10 {
		t.Fatalf("compile proofs = %d, want 10", len(proofs))
	}
	for _, proof := range proofs {
		if !fileLines[normalize(proof.Line)] {
			t.Errorf("proof line not in interface_matrix.go: %q", proof.Line)
		}
		cell := proof.Pins
		if strings.HasSuffix(cell, " (promoted)") || pinnedElsewhere[cell] {
			continue // embedding exhibits — pinned by the embeddings tests
		}
		if !trueCells[cell] {
			t.Errorf("proof %q pins %q, but that matrix cell did not compute true", proof.Line, proof.Pins)
		}
	}
}

func TestIfaceMatrixPayloadIsPure(t *testing.T) {
	first, second := BuildInterfaceMatrixPayload(), BuildInterfaceMatrixPayload()
	if !reflect.DeepEqual(first, second) {
		t.Error("builder must be pure: two calls returned different payloads")
	}
}
