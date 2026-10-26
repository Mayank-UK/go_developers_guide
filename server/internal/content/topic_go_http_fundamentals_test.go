package content

import (
	"strings"
	"testing"
)

// Topic 04 go-http-fundamentals pattern pins, on the BUILDER (via
// NewRegistry with the framework group) — registration in
// NewProductionRegistry is the master's wave-close step, so these tests
// pin the topic's contract without it: exact section order, unique block
// ids, the inline demo beside its teaching, the bash terminal companion
// last, the family prose floor, and — belt to the snippet-sources ratchet —
// every must-source block carrying a source whose file the snippet really
// matches. That last belt runs the real violations walker against this
// topic's own registry, so the quotes are verified NOW, not only once
// registration lands.

const goHTTPFundamentalsID = "go-http-fundamentals"

var goHTTPFundamentalsSectionIDs = []string{
	"setup",
	"routing-patterns",
	"handler-anatomy",
	"middleware-chain",
	"life-of-a-request",
	"error-first-steps",
	"stdlib-vs-frameworks",
	"pprof-expvar",
	"dev-workflow",
	"terminal-companion",
}

// minGoHTTPFundamentalsProseWords is the family depth mandate: ~3k words
// per topic, floor 2500, counted over text blocks only.
const minGoHTTPFundamentalsProseWords = 2500

func goHTTPFundamentalsTestTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := goHTTPFundamentalsTopic()
	if err != nil {
		t.Fatalf("build go-http-fundamentals topic: %v", err)
	}
	return topic
}

func goHTTPFundamentalsTestRegistry(t *testing.T) Registry {
	t.Helper()
	registry, err := NewRegistry(
		[]Topic{goHTTPFundamentalsTestTopic(t)},
		[]TopicGroup{{ID: "framework", Title: "Framework", Order: 2}},
	)
	if err != nil {
		t.Fatalf("test registry: %v", err)
	}
	return registry
}

func TestGoHTTPFundamentalsTopicIsFrameworkOrderFour(t *testing.T) {
	topic := goHTTPFundamentalsTestTopic(t)
	if topic.Group != "framework" || topic.Order != 4 {
		t.Errorf("topic group/order = %q/%d, want framework/4", topic.Group, topic.Order)
	}
	registry := goHTTPFundamentalsTestRegistry(t)
	topic, ok := registry.TopicByID(goHTTPFundamentalsID)
	if !ok {
		t.Fatalf("topic %q must exist in its test registry", goHTTPFundamentalsID)
	}
	for _, summary := range registry.Summaries() {
		if summary.ID == goHTTPFundamentalsID && summary.SectionCount != len(goHTTPFundamentalsSectionIDs) {
			t.Errorf("sectionCount = %d, want %d", summary.SectionCount, len(goHTTPFundamentalsSectionIDs))
		}
	}
}

func TestGoHTTPFundamentalsSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(goHTTPFundamentalsSectionIDs))
	for _, section := range goHTTPFundamentalsTestTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(goHTTPFundamentalsSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, goHTTPFundamentalsSectionIDs)
	}
}

func TestGoHTTPFundamentalsBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range goHTTPFundamentalsTestTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestGoHTTPFundamentalsDemoBlockSitsInlineInHandlerAnatomySection(t *testing.T) {
	topic := goHTTPFundamentalsTestTopic(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "handler-anatomy" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("handler-anatomy section missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "gohttp:app-info" {
		t.Errorf("demo blocks = %+v, want exactly gohttp:app-info", demos)
	}
	// Inline means beside the teaching: the section carries prose, code,
	// and the demo together.
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("handler-anatomy section lacks a %q block; kinds = %v", want, kinds)
		}
	}
}

func TestGoHTTPFundamentalsClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := goHTTPFundamentalsTestTopic(t).Sections
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
	if codeBlocks == 0 {
		t.Error("terminal companion needs its curl tour")
	}
}

func TestGoHTTPFundamentalsProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range goHTTPFundamentalsTestTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minGoHTTPFundamentalsProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minGoHTTPFundamentalsProseWords)
	}
	t.Logf("go-http-fundamentals prose word count: %d", words)
}

func TestGoHTTPFundamentalsMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range goHTTPFundamentalsTestTopic(t).Sections {
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

// The real belt: the snippet-sources walker against this topic's own
// registry. Registration in NewProductionRegistry lands with the master's
// wave close; this runs the identical verification NOW, so a quote that
// drifted from its source file fails here first.
func TestGoHTTPFundamentalsQuotesMatchTheirSourceFiles(t *testing.T) {
	registry := goHTTPFundamentalsTestRegistry(t)
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("source-quote violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}
