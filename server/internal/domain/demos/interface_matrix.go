// Topic 05 interfaces-and-composition's live demo: a satisfaction MATRIX —
// real guide types (and the stdlib's canon) crossed with the interfaces
// they may or may not satisfy, every verdict computed live from the actual
// method sets via reflect, never authored. The payload also carries the
// embedding exhibits (satisfaction by promotion), a type switch whose case
// order matters, and the compile-time `var _ Iface = Type{}` proofs that
// pin the same facts at build time. The topic quotes this file; pure and
// deterministic by construction: same call, same payload, zero inputs.
package demos

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"

	"guide/internal/content"
	"guide/internal/core"
)

// Compile-time satisfaction proofs: each line pins one TRUE cell of the
// matrix — the compiler is the checker, and for the two exhibit types the
// proof is that PROMOTED methods feed the method set. reflect computes the
// live verdicts below; these lines hold the same facts at build time.
var (
	_ fmt.Stringer  = time.Duration(0)
	_ error         = (*core.Error)(nil)
	_ io.Reader     = (*bytes.Buffer)(nil)
	_ io.ReadWriter = (*bytes.Buffer)(nil)
	_ io.Reader     = (*strings.Reader)(nil)
	_ http.Handler  = http.HandlerFunc(nil)
	_ content.Block = content.TextBlock{}
	_ content.Block = content.CodeBlock{}
	_ io.Reader     = taggedReader{}
	_ http.Handler  = delegatingHandler{}
)

// InterfaceSpec is one column of the matrix: an interface, the single
// method (rendered from the real reflect.Type) that defines it, and where
// it comes from.
type InterfaceSpec struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	Pkg    string `json:"pkg"`
	Note   string `json:"note"`
}

// MatrixCell is one row × column verdict. Satisfied is computed live from
// the row type's actual method set — never authored.
type MatrixCell struct {
	Iface     string `json:"iface"`
	Satisfied bool   `json:"satisfied"`
}

// SatisfactionRow is one concrete type under inspection: its expression,
// its reflect type/kind, its exported method set (reflect cannot list
// unexported methods — the note on the content.Block column), and one
// verdict per column.
type SatisfactionRow struct {
	Expression string       `json:"expression"`
	Type       string       `json:"type"`
	Kind       string       `json:"kind"`
	Methods    []string     `json:"methods"`
	Cells      []MatrixCell `json:"cells"`
}

// EmbeddingRow is one composition exhibit: a type that satisfies an
// interface purely through PROMOTED methods from an embedded value, or (in
// the third row) an interface composed by embedding two others.
type EmbeddingRow struct {
	Expression string `json:"expression"`
	Embeds     string `json:"embeds"`
	Type       string `json:"type"`
	Iface      string `json:"iface"`
	Satisfied  bool   `json:"satisfied"`
	Note       string `json:"note"`
}

// SwitchCaseRow is one input through classify: which case matched, and
// what came back.
type SwitchCaseRow struct {
	Input       string `json:"input"`
	MatchedCase string `json:"matchedCase"`
	Result      string `json:"result"`
	Note        string `json:"note,omitempty"`
}

// CompileProof is one `var _ Iface = Type{}` line: the source text (this
// file) and the matrix cell it pins. The demos test verifies every line
// really occurs in this file and every pinned cell really computed true.
type CompileProof struct {
	Line string `json:"line"`
	Pins string `json:"pins"`
}

// InterfaceMatrixPayload is the wire shape of GET /api/demo/iface/matrix
// (camelCase JSON) — topic 05's inline demo.
type InterfaceMatrixPayload struct {
	Interfaces    []InterfaceSpec   `json:"interfaces"`
	Rows          []SatisfactionRow `json:"rows"`
	Embeddings    []EmbeddingRow    `json:"embeddings"`
	TypeSwitch    []SwitchCaseRow   `json:"typeSwitch"`
	CompileProofs []CompileProof    `json:"compileProofs"`
}

// taggedReader adds a tag to a *strings.Reader by embedding it: Read is
// promoted from the embedded pointer, so taggedReader satisfies io.Reader
// without declaring a single method — the whole *strings.Reader method set
// arrives with it.
type taggedReader struct {
	*strings.Reader
	//lint:ignore U1000 the tag is quoted verbatim by topic 05's embed-promotion snippet; removing or using it is a master-routed re-quote, not a silent edit
	tag string
}

// delegatingHandler forwards ServeHTTP to whatever handler it embeds —
// satisfaction by promotion from an embedded INTERFACE, again with zero
// methods of its own.
type delegatingHandler struct {
	http.Handler
}

// matrixColumn ties a column's display spec to the real reflect.Type it
// checks against — the payload never trusts a hand-written verdict.
type matrixColumn struct {
	name string
	typ  reflect.Type
	pkg  string
	note string
}

// matrixColumns is the interface axis: two stdlib canons, the builtin
// error, the server world's http.Handler, and the repo's own sealed
// content.Block.
func matrixColumns() []matrixColumn {
	return []matrixColumn{
		{
			name: "fmt.Stringer",
			typ:  reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
			pkg:  "fmt",
			note: "One method, String() string — fmt's whole formatting world keys off it.",
		},
		{
			name: "error",
			typ:  reflect.TypeOf((*error)(nil)).Elem(),
			pkg:  "builtin",
			note: "The language's own interface: Error() string, the errors-as-values contract.",
		},
		{
			name: "io.Reader",
			typ:  reflect.TypeOf((*io.Reader)(nil)).Elem(),
			pkg:  "io",
			note: "The canon: Read([]byte) (int, error) carries half the stdlib on its back.",
		},
		{
			name: "http.Handler",
			typ:  reflect.TypeOf((*http.Handler)(nil)).Elem(),
			pkg:  "net/http",
			note: "One method, the entire HTTP layer of this server.",
		},
		{
			name: "content.Block",
			typ:  reflect.TypeOf((*content.Block)(nil)).Elem(),
			pkg:  "guide/internal/content",
			note: "blockKind() string is unexported — only package content can implement it. " +
				"reflect lists exported methods only, so this column's verdicts come from the full " +
				"method-set check; the compiler holds the same fact via var _.",
		},
	}
}

// signatureOf renders an interface's single method the way Go source
// writes it — "Read([]byte) (int, error)" — from the real reflect.Type,
// not from a stored string.
func signatureOf(iface reflect.Type) string {
	method := iface.Method(0)
	render := func(t reflect.Type) string {
		return strings.ReplaceAll(t.String(), "uint8", "byte")
	}
	params := make([]string, 0, method.Type.NumIn())
	for i := 0; i < method.Type.NumIn(); i++ {
		params = append(params, render(method.Type.In(i)))
	}
	results := make([]string, 0, method.Type.NumOut())
	for i := 0; i < method.Type.NumOut(); i++ {
		results = append(results, render(method.Type.Out(i)))
	}
	signature := method.Name + "(" + strings.Join(params, ", ") + ")"
	switch len(results) {
	case 0:
	case 1:
		signature += " " + results[0]
	default:
		signature += " (" + strings.Join(results, ", ") + ")"
	}
	return signature
}

// interfaceSpecs maps the columns to their wire spec, method signatures
// rendered from the real types.
func interfaceSpecs() []InterfaceSpec {
	cols := matrixColumns()
	specs := make([]InterfaceSpec, 0, len(cols))
	for _, col := range cols {
		specs = append(specs, InterfaceSpec{
			Name:   col.name,
			Method: signatureOf(col.typ),
			Pkg:    col.pkg,
			Note:   col.note,
		})
	}
	return specs
}

// satisfactionRow computes one row: the value's type, kind, exported
// method set, and one live Implements verdict per column.
func satisfactionRow(expression string, value any, cols []matrixColumn) SatisfactionRow {
	typ := reflect.TypeOf(value)
	methods := make([]string, 0, typ.NumMethod())
	for i := 0; i < typ.NumMethod(); i++ {
		methods = append(methods, typ.Method(i).Name)
	}
	sort.Strings(methods)
	cells := make([]MatrixCell, 0, len(cols))
	for _, col := range cols {
		cells = append(cells, MatrixCell{Iface: col.name, Satisfied: typ.Implements(col.typ)})
	}
	return SatisfactionRow{
		Expression: expression,
		Type:       typ.String(),
		Kind:       typ.Kind().String(),
		Methods:    methods,
		Cells:      cells,
	}
}

// matrixRows is the type axis: the content model's own blocks, the error
// type twice (pointer and value — the method-set lesson), the stdlib
// canons, and two real guide types whose rich method sets satisfy none of
// the five columns.
func matrixRows(cols []matrixColumn) []SatisfactionRow {
	return []SatisfactionRow{
		satisfactionRow(`content.TextBlock{}`, content.TextBlock{}, cols),
		satisfactionRow(`content.CodeBlock{}`, content.CodeBlock{}, cols),
		satisfactionRow(`content.Topic{}`, content.Topic{}, cols),
		satisfactionRow(`*core.Error`, &core.Error{}, cols),
		satisfactionRow(`core.Error`, core.Error{}, cols),
		satisfactionRow(`*bytes.Buffer`, &bytes.Buffer{}, cols),
		satisfactionRow(`*strings.Reader`, &strings.Reader{}, cols),
		satisfactionRow(`time.Duration`, time.Duration(0), cols),
		satisfactionRow(`http.HandlerFunc`, http.HandlerFunc(nil), cols),
		satisfactionRow(`*core.ScenarioStore`, &core.ScenarioStore{}, cols),
		satisfactionRow(`content.Registry`, content.Registry{}, cols),
	}
}

// embeddings carries the promotion exhibits. Every Satisfied verdict is
// computed with the same Implements check the matrix uses — promotion is
// not a special case to the type system, which is the point.
func embeddings() []EmbeddingRow {
	readerIface := reflect.TypeOf((*io.Reader)(nil)).Elem()
	handlerIface := reflect.TypeOf((*http.Handler)(nil)).Elem()
	readWriterIface := reflect.TypeOf((*io.ReadWriter)(nil)).Elem()
	return []EmbeddingRow{
		{
			Expression: "type taggedReader struct { *strings.Reader; tag string }",
			Embeds:     "*strings.Reader (concrete type)",
			Type:       reflect.TypeOf(taggedReader{}).String(),
			Iface:      "io.Reader",
			Satisfied:  reflect.TypeOf(taggedReader{}).Implements(readerIface),
			Note: "Read arrives by promotion — taggedReader declares zero methods, " +
				"and its method set is the embedded pointer's entire one.",
		},
		{
			Expression: "type delegatingHandler struct { http.Handler }",
			Embeds:     "http.Handler (interface)",
			Type:       reflect.TypeOf(delegatingHandler{}).String(),
			Iface:      "http.Handler",
			Satisfied:  reflect.TypeOf(delegatingHandler{}).Implements(handlerIface),
			Note: "An embedded interface promotes its whole method set, so the outer " +
				"struct satisfies it while owning nothing but the field.",
		},
		{
			Expression: "type ReadWriter interface { Reader; Writer }",
			Embeds:     "io.Reader + io.Writer (interface in interface)",
			Type:       reflect.TypeOf(&bytes.Buffer{}).String(),
			Iface:      "io.ReadWriter",
			Satisfied:  reflect.TypeOf(&bytes.Buffer{}).Implements(readWriterIface),
			Note: "Interfaces embed interfaces too: two one-method sides compose a " +
				"two-method whole, and *bytes.Buffer satisfies it with the same methods " +
				"that satisfied each half.",
		},
	}
}

// classify is the demo's type switch: concrete cases first, interface
// cases after, default loud. Case order matters — a value matching two
// cases belongs to the FIRST one.
func classify(v any) string {
	switch x := v.(type) {
	case content.TextBlock:
		return "text block " + x.ID
	case *core.Error:
		return fmt.Sprintf("error %s: %s", x.Kind, x.Message)
	case io.Reader:
		return fmt.Sprintf("io.Reader case — dynamic type %T", x)
	case fmt.Stringer:
		return "fmt.Stringer case — " + x.String()
	default:
		return fmt.Sprintf("unhandled %T", x)
	}
}

// switchRows feeds classify real values in a deliberate order: two
// concrete hits, a value that matches TWO cases (the first-match proof),
// a Stringer-only hit, and a fall-through to default.
func switchRows() []SwitchCaseRow {
	text, _ := content.NewTextBlock("demo:text", "One paragraph of prose.")
	notFound := core.Newf(core.KindNotFound, "Topic %q not found", "nope")
	return []SwitchCaseRow{
		{
			Input:       `content.TextBlock{ID: "demo:text"}`,
			MatchedCase: "case content.TextBlock",
			Result:      classify(text),
		},
		{
			Input:       `*core.Error`,
			MatchedCase: "case *core.Error",
			Result:      classify(notFound),
		},
		{
			Input:       `*strings.Reader`,
			MatchedCase: "case io.Reader",
			Result:      classify(strings.NewReader("")),
		},
		{
			Input:       `*bytes.Buffer`,
			MatchedCase: "case io.Reader",
			Result:      classify(bytes.NewBufferString("")),
			Note: "*bytes.Buffer satisfies fmt.Stringer too — cases run top to bottom, " +
				"and the first match wins.",
		},
		{
			Input:       `time.Duration`,
			MatchedCase: "case fmt.Stringer",
			Result:      classify(90 * time.Second),
		},
		{
			Input:       `content.Topic{}`,
			MatchedCase: "default",
			Result:      classify(content.Topic{}),
		},
	}
}

// compileProofs carries the var _ lines above as data, each naming the
// matrix cell it pins. The demos test cross-checks both directions.
func compileProofs() []CompileProof {
	return []CompileProof{
		{Line: "_ fmt.Stringer  = time.Duration(0)", Pins: "fmt.Stringer × time.Duration"},
		{Line: "_ error         = (*core.Error)(nil)", Pins: "error × *core.Error"},
		{Line: "_ io.Reader     = (*bytes.Buffer)(nil)", Pins: "io.Reader × *bytes.Buffer"},
		{Line: "_ io.ReadWriter = (*bytes.Buffer)(nil)", Pins: "io.ReadWriter × *bytes.Buffer"},
		{Line: "_ io.Reader     = (*strings.Reader)(nil)", Pins: "io.Reader × *strings.Reader"},
		{Line: "_ http.Handler  = http.HandlerFunc(nil)", Pins: "http.Handler × http.HandlerFunc"},
		{Line: "_ content.Block = content.TextBlock{}", Pins: "content.Block × content.TextBlock{}"},
		{Line: "_ content.Block = content.CodeBlock{}", Pins: "content.Block × content.CodeBlock{}"},
		{Line: "_ io.Reader     = taggedReader{}", Pins: "io.Reader × taggedReader (promoted)"},
		{Line: "_ http.Handler  = delegatingHandler{}", Pins: "http.Handler × delegatingHandler (promoted)"},
	}
}

// BuildInterfaceMatrixPayload assembles the whole demo payload. Pure:
// zero inputs, no I/O, every verdict computed from real method sets —
// the handler layer adds only transport.
func BuildInterfaceMatrixPayload() InterfaceMatrixPayload {
	cols := matrixColumns()
	return InterfaceMatrixPayload{
		Interfaces:    interfaceSpecs(),
		Rows:          matrixRows(cols),
		Embeddings:    embeddings(),
		TypeSwitch:    switchRows(),
		CompileProofs: compileProofs(),
	}
}
