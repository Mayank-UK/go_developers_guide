package content

import (
	"strings"
	"testing"
)

// The matcher suite ports the fastapi guide's test_snippet_matcher.py
// (Go sources where the originals were Python) plus the Go-guide elision
// edge cases: every case pins the family algorithm semantics.

const goSource = `package demo

import "os"

func Configure(name string) string {
	// full-line comment in the source
	value := os.Getenv(name)
	return strings.ToLower(value) // normalize case
}
`

func match(t *testing.T, source, snippet, language string) (bool, string) {
	t.Helper()
	return MatchSnippet(source, snippet, language)
}

func mustMatch(t *testing.T, source, snippet, language string) {
	t.Helper()
	if ok, reason := match(t, source, snippet, language); !ok {
		t.Fatalf("expected a match, got: %s", reason)
	}
}

func mustReject(t *testing.T, source, snippet, language string) string {
	t.Helper()
	ok, reason := match(t, source, snippet, language)
	if ok {
		t.Fatalf("expected a rejection, snippet matched: %q", snippet)
	}
	if reason == "" {
		t.Fatal("rejection must carry a reason")
	}
	return reason
}

func TestExactLinesMatch(t *testing.T) {
	mustMatch(t, goSource, "value := os.Getenv(name)\nreturn strings.ToLower(value)", "go")
}

func TestBlanksAndReindentationAreTolerated(t *testing.T) {
	mustMatch(t, goSource,
		"func Configure(name string) string {\n\n\t\treturn strings.ToLower(value)", "go")
}

func TestFullLineCommentsAreStripped(t *testing.T) {
	mustMatch(t, goSource,
		"// teaching prose the source does not have\nreturn strings.ToLower(value)", "go")
}

func TestTrailingTeachingCommentsAreStripped(t *testing.T) {
	mustMatch(t, goSource,
		"value := os.Getenv(name)  // read once per call", "go")
}

func TestElisionsAtStartMiddleAndEnd(t *testing.T) {
	snippet := "// …\n" +
		"package demo\n" +
		"// ... rest of the file header\n" +
		"func Configure(name string) string {\n" +
		"// …\n" +
		"return strings.ToLower(value)\n" +
		"// …\n"
	mustMatch(t, goSource, snippet, "go")
}

func TestConsecutiveElisionsAndZeroLineSpan(t *testing.T) {
	// Two elisions in a row, and an elision between two adjacent source
	// lines (a zero-line gap) — both are just comment lines that normalize
	// away.
	mustMatch(t, goSource,
		"package demo\n// …\n// …\nimport \"os\"", "go")
	mustMatch(t, goSource,
		"value := os.Getenv(name)\n// …\nreturn strings.ToLower(value)", "go")
}

func TestGoBlockCommentsAndRawStrings(t *testing.T) {
	// An inline block comment leaves the surrounding tokens AND spaces:
	// "f(a /* x */, b)" normalizes to "f(a , b)" — the family matcher never
	// collapses whitespace, so the snippet quotes what is left.
	source := "/* header */\n" +
		"const doc = `a // not a comment`\n" +
		"const url = \"https://api.example.com/v1\" // base\n" +
		"var x = 1 + 2 /* inline note */\n" +
		"var y = f(a /* x */, b)\n"
	mustMatch(t, source, "const doc = `a // not a comment`\nvar x = 1 + 2", "go")
	mustMatch(t, source, "const url = \"https://api.example.com/v1\"", "go")
	mustMatch(t, source, "var y = f(a , b)", "go")
}

func TestTSLineCommentsAndURLStrings(t *testing.T) {
	source := "const base = 'https://api.example.com/v1';\n" +
		"const timeout = 5_000; // milliseconds\n" +
		"fetch(base, { signal });\n"
	mustMatch(t, source,
		"const base = 'https://api.example.com/v1';\nconst timeout = 5_000;\nfetch(base, { signal });",
		"ts")
}

func TestSQLDashCommentsNeedABoundary(t *testing.T) {
	source := "UPDATE counters SET total = total--  -- decrement\n" +
		"-- full-line comment\n" +
		"WHERE id = $1;\n"
	mustMatch(t, source, "UPDATE counters SET total = total--\nWHERE id = $1;", "sql")
}

func TestProtoLineAndBlockComments(t *testing.T) {
	source := "// the contract\n" +
		"syntax = \"proto3\";\n" +
		"\n" +
		"message Post {\n" +
		"  string slug = 1;  // the id\n" +
		"  /* repeated tags */ repeated string tags = 5;\n" +
		"}\n"
	mustMatch(t, source,
		"syntax = \"proto3\";\nmessage Post {\n  string slug = 1;\n  repeated string tags = 5;\n}",
		"proto")
	mustMatch(t, source, "syntax = \"proto3\";\n// …\nmessage Post {", "proto")
}

func TestYAMLAndTOMLHashAfterWhitespace(t *testing.T) {
	source := "services:\n  server: # the api\n    port: 8100\n    key: '#literal'\n"
	mustMatch(t, source, "services:\n  server:\n    port: 8100\n    key: '#literal'", "yaml")
	mustMatch(t, "go = \"1.27.3\"  # pinned\n", "go = \"1.27.3\"", "toml")
}

func TestXMLAndHTMLCommentsAndMultilineBlocks(t *testing.T) {
	source := "<!-- header -->\n" +
		"<server>\n" +
		"  <port>8100</port>\n" +
		"</server>\n" +
		"<!-- a long comment\n" +
		"     spanning lines -->\n" +
		"<tail>fin</tail>\n"
	mustMatch(t, source, "<server>\n<port>8100</port>\n</server>\n<tail>fin</tail>", "xml")
	mustMatch(t, source, "<!-- teaching note -->\n<tail>fin</tail>", "html")
}

func TestJSONIsBlankStripOnly(t *testing.T) {
	source := "{\n  \"name\": \"guide\",\n  \"go\": \"1.27.3\"\n}"
	mustMatch(t, source, "{\n\n  \"name\": \"guide\",\n}", "json")
	// A // "comment" is content in json — it must NOT match a source lacking it.
	reason := mustReject(t, source, "\"name\": \"guide\",\n// teaching note", "json")
	if !strings.Contains(reason, "not found in source") {
		t.Errorf("reason %q does not name the unmatched line", reason)
	}
}

func TestCRLFSourceMatchesLFSnippet(t *testing.T) {
	source := "package demo\r\n\r\nfunc A() {}\r\n\r\nfunc B() {}\r\n"
	mustMatch(t, source, "func A() {}\nfunc B() {}", "go")
}

func TestUnknownLanguageIsBlankStripOnly(t *testing.T) {
	source := "pull the image\ncurl the endpoint\n"
	mustMatch(t, source, "pull the image\ncurl the endpoint", "dockerfile")
	mustReject(t, source, "pull the image\n# a yaml comment", "dockerfile")
}

func TestReorderedLinesFail(t *testing.T) {
	mustReject(t, goSource, "return strings.ToLower(value)\npackage demo", "go")
}

func TestMissingLineFailsWithReason(t *testing.T) {
	reason := mustReject(t, goSource, "value := os.LookupEnv(name)", "go")
	if !strings.Contains(reason, "not found in source") {
		t.Errorf("reason %q does not say the line was not found", reason)
	}
}

func TestWrongLanguageTokenIsContentNotComment(t *testing.T) {
	// A "#" comment inside a go snippet is content — go strips only //.
	mustReject(t, goSource, "package demo\n# a yaml-style comment inside a go snippet", "go")
}

func TestAllCommentSnippetFails(t *testing.T) {
	ok, reason := match(t, goSource, "// …\n// only elisions", "go")
	if ok || !strings.Contains(reason, "empty after normalization") {
		t.Errorf("all-comment snippet: ok=%v reason=%q", ok, reason)
	}
}

func TestReasonCapsLongLines(t *testing.T) {
	long := "value := " + strings.Repeat("x", 200)
	reason := mustReject(t, goSource, long, "go")
	if !strings.Contains(reason, "…") || len(reason) > 200 {
		t.Errorf("reason not capped readably: %q (len %d)", reason, len(reason))
	}
}
