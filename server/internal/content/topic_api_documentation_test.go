package content

import (
	"strings"
	"testing"
)

// Topic 12 api-documentation — pinned the same way the exemplar pins
// topic 01: exact section order, unique block ids, the inline demo in the
// section that teaches serving the spec, the terminal companion last, the
// family prose floor, the must-source belt, and the snippet ratchet run
// against this topic's own registry. Production registration in
// NewProductionRegistry is the wave-wiring step and is deliberately NOT
// asserted here.

const apiDocumentationID = "api-documentation"

var apiDocumentationSectionIDs = []string{
	"why-machine-readable",
	"openapi-anatomy",
	"spec-first-with-oapi-codegen",
	"annotations-by-comparison",
	"serving-the-spec",
	"drift-tests",
	"beyond-rest",
	"terminal-companion",
}

// minAPIDocumentationProseWords is the family depth mandate: ~3k words
// per topic, floor 2500, counted over text blocks only — list items and
// code never count.
const minAPIDocumentationProseWords = 2500

// builtAPIDocumentation builds the topic directly; the builder must
// assemble every block without a single constructor error.
func builtAPIDocumentation(t *testing.T) Topic {
	t.Helper()
	topic, err := apiDocumentationTopic()
	if err != nil {
		t.Fatalf("api-documentation builder must build cleanly: %v", err)
	}
	return topic
}

// registeredAPIDocumentation validates the builder's output through the
// registry's own rules (the real production gate) on a one-topic registry
// carrying the topic's curriculum group.
func registeredAPIDocumentation(t *testing.T) Topic {
	t.Helper()
	topic := builtAPIDocumentation(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "apis", Title: "APIs", Order: 4}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestAPIDocumentationIsAPIsOrderTwelve(t *testing.T) {
	topic := registeredAPIDocumentation(t)
	if topic.Group != "apis" || topic.Order != 12 {
		t.Errorf("topic group/order = %q/%d, want apis/12", topic.Group, topic.Order)
	}
	if topic.ID != apiDocumentationID {
		t.Errorf("topic id = %q, want %q", topic.ID, apiDocumentationID)
	}
}

func TestAPIDocumentationSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(apiDocumentationSectionIDs))
	for _, section := range builtAPIDocumentation(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(apiDocumentationSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, apiDocumentationSectionIDs)
	}
}

func TestAPIDocumentationBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtAPIDocumentation(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestAPIDocumentationDemoSitsInlineInTheServingSection(t *testing.T) {
	topic := builtAPIDocumentation(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "serving-the-spec" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("serving-the-spec section missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "api:docs-link" {
		t.Errorf("demo blocks = %+v, want exactly api:docs-link", demos)
	}
	// Inline means beside the teaching: the section carries prose, code,
	// and the demo together.
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("serving-the-spec section lacks a %q block; kinds = %v", want, kinds)
		}
	}
}

func TestAPIDocumentationClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtAPIDocumentation(t).Sections
	last := sections[len(sections)-1]
	if last.ID != "terminal-companion" {
		t.Fatalf("last section = %q, want terminal-companion", last.ID)
	}
	codeBlocks := 0
	for _, block := range last.Blocks {
		if code, ok := block.(CodeBlock); ok {
			codeBlocks++
			if code.Language != "bash" || code.Source != "" {
				t.Errorf("terminal block %q must be bash with no source", code.ID)
			}
		}
	}
	if codeBlocks < 5 {
		t.Errorf("terminal companion needs its curl tour, got %d bash blocks", codeBlocks)
	}
}

func TestAPIDocumentationProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtAPIDocumentation(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minAPIDocumentationProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minAPIDocumentationProseWords)
	}
	t.Logf("api-documentation prose word count: %d", words)
}

func TestAPIDocumentationMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source language
	// with an empty source path — fails fast here too, at the topic layer,
	// instead of only in the file-reading ratchet.
	for _, section := range builtAPIDocumentation(t).Sections {
		for _, block := range section.Blocks {
			code, ok := block.(CodeBlock)
			if !ok || !MustSourceLanguages[strings.ToLower(code.Language)] {
				continue
			}
			if strings.TrimSpace(code.Source) == "" {
				t.Errorf("code block %q (language %q) lacks a source path", code.ID, code.Language)
			}
		}
	}
}

func TestAPIDocumentationSnippetSourcesResolve(t *testing.T) {
	// The full snippet ratchet (RATCHET = 0), run against this topic's own
	// registry until production registration lands: every must-source block
	// must quote a real repo file, in order, after normalization.
	topic := builtAPIDocumentation(t)
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "apis", Title: "APIs", Order: 4}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}

func TestAPIDocumentationQuotesTheRealSpecFile(t *testing.T) {
	// The topic's spine is server/api/openapi.yaml — at least three
	// distinct code blocks must source that exact file (yaml is
	// must-source), beside the codegen config, the generated types, the
	// walker, and the drift test.
	sources := make(map[string]int)
	for _, section := range builtAPIDocumentation(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && MustSourceLanguages[strings.ToLower(code.Language)] {
				sources[code.Source]++
			}
		}
	}
	if sources["server/api/openapi.yaml"] < 3 {
		t.Errorf("spec quotes = %d, want at least three blocks sourcing server/api/openapi.yaml: %v",
			sources["server/api/openapi.yaml"], sources)
	}
	for _, required := range []string{
		"server/internal/apispec/cfg.yaml",
		"server/internal/apispec/gen.go",
		"server/internal/apispec/types.gen.go",
		"server/internal/apispec/walker.go",
		"server/internal/presentation/spec_drift_test.go",
	} {
		if sources[required] < 1 {
			t.Errorf("the topic must quote %s; sourced files: %v", required, sources)
		}
	}
}
