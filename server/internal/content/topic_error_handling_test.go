package content

import (
	"strings"
	"testing"
)

// Topic 30 error-handling — pinned the same way every per-topic test
// pins its topic: exact section order, unique block ids, the ONE demo
// inline in the section that teaches failure injection (prose + code +
// demo co-present), the bash terminal companion last, the family prose
// floor, the must-source belt, and the full snippet ratchet on a
// one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 30 and group
// "platform" on the built topic itself.

const errorHandlingID = "error-handling"

var errorHandlingSectionIDs = []string{
	"errors-are-values",
	"wrapping-and-matching",
	"sentinel-versus-typed",
	"one-taxonomy-one-envelope",
	"the-panic-policy",
	"failure-injection",
	"what-logs-on-failure",
	"process-nets",
	"terminal-companion",
}

// minErrorHandlingProseWords is the family depth mandate: ~3k words
// per topic, floor 2500, counted over text blocks only.
const minErrorHandlingProseWords = 2500

func builtErrorHandlingTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := errorHandlingTopic()
	if err != nil {
		t.Fatalf("error-handling builder must build cleanly: %v", err)
	}
	return topic
}

func registeredErrorHandlingTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtErrorHandlingTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "platform", Title: "Platform", Order: 8}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestErrorHandlingIsPlatformOrderThirty(t *testing.T) {
	topic := registeredErrorHandlingTopic(t)
	if topic.Group != "platform" || topic.Order != 30 {
		t.Errorf("topic group/order = %q/%d, want platform/30", topic.Group, topic.Order)
	}
	if topic.ID != errorHandlingID {
		t.Errorf("topic id = %q, want %q", topic.ID, errorHandlingID)
	}
}

func TestErrorHandlingSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(errorHandlingSectionIDs))
	for _, section := range builtErrorHandlingTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(errorHandlingSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, errorHandlingSectionIDs)
	}
}

func TestErrorHandlingBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtErrorHandlingTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestErrorHandlingDemoSitsInlineInItsTeachingSection pins the family
// anatomy: the errors:explorer demo lives in the section that teaches
// failure injection, co-present with prose and sourced code — never
// queued at the end.
func TestErrorHandlingDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtErrorHandlingTopic(t)

	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "failure-injection" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatalf("section failure-injection missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "errors:explorer" {
		t.Errorf("failure-injection demo blocks = %+v, want exactly errors:explorer", demos)
	}
	for _, kind := range []string{KindText, KindCode, KindDemo} {
		if !kinds[kind] {
			t.Errorf("section failure-injection lacks a %q block; kinds = %v", kind, kinds)
		}
	}
	total := 0
	for _, section := range topic.Sections {
		for _, block := range section.Blocks {
			if _, ok := block.(DemoBlock); ok {
				total++
			}
		}
	}
	if total != 1 {
		t.Errorf("topic carries %d demos, want exactly 1", total)
	}
}

func TestErrorHandlingClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtErrorHandlingTopic(t).Sections
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

func TestErrorHandlingProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtErrorHandlingTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minErrorHandlingProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minErrorHandlingProseWords)
	}
	t.Logf("error-handling prose word count: %d", words)
}

func TestErrorHandlingMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtErrorHandlingTopic(t).Sections {
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

func TestErrorHandlingSnippetSourcesResolve(t *testing.T) {
	topic := builtErrorHandlingTopic(t)
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "platform", Title: "Platform", Order: 8}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}
