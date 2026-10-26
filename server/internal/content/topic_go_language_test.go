package content

import (
	"strings"
	"testing"
)

// Topic 01 go-language is the EXEMPLAR topic: these tests pin its shape so
// later topics copy a contract, not a guess — exact section order, unique
// block ids, the inline demo in the section that teaches it, the terminal
// companion last, the family prose floor, and (belt to the snippet-sources
// ratchet in snippet_sources_test.go, which verifies every quoted line
// against the real file) no must-source block without a source path.

const goLanguageID = "go-language"

var goLanguageSectionIDs = []string{
	"why-go",
	"go-basics",
	"go-data-and-strings",
	"structs-methods",
	"interfaces",
	"pointers-vs-values",
	"errors-as-values",
	"packages",
	"generics",
	"runtime-types",
	"terminal-companion",
}

// minGoLanguageProseWords is the family depth mandate: ~3k words per
// topic; the floor catches thin topics. Counted over text blocks only —
// list items and code never count.
const minGoLanguageProseWords = 2500

func registeredGoLanguage(t *testing.T) Topic {
	t.Helper()
	registry, err := NewProductionRegistry()
	if err != nil {
		t.Fatalf("production registry must boot: %v", err)
	}
	topic, ok := registry.TopicByID(goLanguageID)
	if !ok {
		t.Fatalf("topic %q must be registered in the production registry", goLanguageID)
	}
	return topic
}

func TestGoLanguageIsRegisteredInFoundationsOrderOne(t *testing.T) {
	topic := registeredGoLanguage(t)
	if topic.Group != "foundations" || topic.Order != 1 {
		t.Errorf("topic group/order = %q/%d, want foundations/1", topic.Group, topic.Order)
	}
	registry, err := NewProductionRegistry()
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	for _, summary := range registry.Summaries() {
		if summary.ID == goLanguageID && summary.SectionCount != len(goLanguageSectionIDs) {
			t.Errorf("sectionCount = %d, want %d", summary.SectionCount, len(goLanguageSectionIDs))
		}
	}
}

func TestGoLanguageSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(goLanguageSectionIDs))
	for _, section := range registeredGoLanguage(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(goLanguageSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, goLanguageSectionIDs)
	}
}

func TestGoLanguageBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range registeredGoLanguage(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestGoLanguageDemoBlockSitsInlineInRuntimeTypesSection(t *testing.T) {
	topic := registeredGoLanguage(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "runtime-types" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("runtime-types section missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "go:runtime-types" {
		t.Errorf("demo blocks = %+v, want exactly go:runtime-types", demos)
	}
	// Inline means beside the teaching: the section carries prose, code,
	// and the demo together.
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("runtime-types section lacks a %q block; kinds = %v", want, kinds)
		}
	}
}

func blockKindTag(block Block) string {
	switch block.(type) {
	case TextBlock:
		return KindText
	case CodeBlock:
		return KindCode
	case DemoBlock:
		return KindDemo
	case ListBlock:
		return KindList
	default:
		return "unknown"
	}
}

func TestGoLanguageClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := registeredGoLanguage(t).Sections
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

func TestGoLanguageProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range registeredGoLanguage(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minGoLanguageProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minGoLanguageProseWords)
	}
	t.Logf("go-language prose word count: %d", words)
}

func TestGoLanguageMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source
	// language with an empty source path — fails fast here too, at the
	// topic layer, instead of only in the file-reading ratchet.
	for _, section := range registeredGoLanguage(t).Sections {
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
