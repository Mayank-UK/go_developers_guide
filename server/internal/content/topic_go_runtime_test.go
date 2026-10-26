package content

import (
	"strings"
	"testing"
)

// Topic 02 go-runtime is the first topic to copy the fixed per-topic
// pattern end-to-end after the 3.1 exemplar: these tests pin the same
// contract shape — exact section order, unique block ids, the inline demo
// in the section that teaches it (channels-and-select quotes AND serves
// the real worker pool), the bash terminal companion last, the family
// prose floor, and (belt to the snippet-sources ratchet in
// snippet_sources_test.go, which verifies every quoted line against the
// real file) no must-source block without a source path.

const goRuntimeID = "go-runtime"

var goRuntimeSectionIDs = []string{
	"scheduler",
	"goroutines-vs-threads",
	"channels-and-select",
	"memory-model-and-escape",
	"garbage-collector",
	"toolchain",
	"gomaxprocs-in-containers",
	"static-binary",
	"terminal-companion",
}

// minGoRuntimeProseWords is the family depth mandate: ~3k words per
// topic; the floor catches thin topics. Counted over text blocks only —
// list items and code never count.
const minGoRuntimeProseWords = 2500

func registeredGoRuntime(t *testing.T) Topic {
	t.Helper()
	registry, err := NewProductionRegistry()
	if err != nil {
		t.Fatalf("production registry must boot: %v", err)
	}
	topic, ok := registry.TopicByID(goRuntimeID)
	if !ok {
		t.Fatalf("topic %q must be registered in the production registry", goRuntimeID)
	}
	return topic
}

func TestGoRuntimeIsRegisteredInFoundationsOrderTwo(t *testing.T) {
	topic := registeredGoRuntime(t)
	if topic.Group != "foundations" || topic.Order != 2 {
		t.Errorf("topic group/order = %q/%d, want foundations/2", topic.Group, topic.Order)
	}
	registry, err := NewProductionRegistry()
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	for _, summary := range registry.Summaries() {
		if summary.ID == goRuntimeID && summary.SectionCount != len(goRuntimeSectionIDs) {
			t.Errorf("sectionCount = %d, want %d", summary.SectionCount, len(goRuntimeSectionIDs))
		}
	}
}

func TestGoRuntimeSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(goRuntimeSectionIDs))
	for _, section := range registeredGoRuntime(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(goRuntimeSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, goRuntimeSectionIDs)
	}
}

func TestGoRuntimeBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range registeredGoRuntime(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestGoRuntimeDemoBlockSitsInlineInChannelsSection(t *testing.T) {
	topic := registeredGoRuntime(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "channels-and-select" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("channels-and-select section missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "runtime:concurrency" {
		t.Errorf("demo blocks = %+v, want exactly runtime:concurrency", demos)
	}
	// Inline means beside the teaching: the section carries prose, code,
	// and the demo together.
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("channels-and-select section lacks a %q block; kinds = %v", want, kinds)
		}
	}
}

func TestGoRuntimeClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := registeredGoRuntime(t).Sections
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

func TestGoRuntimeProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range registeredGoRuntime(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minGoRuntimeProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minGoRuntimeProseWords)
	}
	t.Logf("go-runtime prose word count: %d", words)
}

func TestGoRuntimeMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source
	// language with an empty source path — fails fast here too, at the
	// topic layer, instead of only in the file-reading ratchet.
	for _, section := range registeredGoRuntime(t).Sections {
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
