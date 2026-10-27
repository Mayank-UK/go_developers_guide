package content

import (
	"strings"
	"testing"
)

// Topic 32 architecture — pinned the same way every per-topic test pins
// its topic: exact section order, unique block ids, the ONE demo inline
// in the section that teaches the request-flow tracer (prose + code +
// demo co-present), the bash terminal companion last, the family prose
// floor, the must-source belt, and the full snippet ratchet on a
// one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 32 and group
// "platform" on the built topic itself.

const architectureID = "architecture"

var architectureSectionIDs = []string{
	"why-architecture",
	"layer-map",
	"dependency-rule",
	"thin-handlers",
	"composition-root",
	"errors-across-layers",
	"clients-mirror",
	"request-flow-tracer",
	"terminal-companion",
}

// minArchitectureProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only.
const minArchitectureProseWords = 2500

func builtArchitectureTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := architectureTopic()
	if err != nil {
		t.Fatalf("architecture builder must build cleanly: %v", err)
	}
	return topic
}

func registeredArchitectureTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtArchitectureTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "platform", Title: "Platform", Order: 8}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestArchitectureIsPlatformOrderThirtyTwo(t *testing.T) {
	topic := registeredArchitectureTopic(t)
	if topic.Group != "platform" || topic.Order != 32 {
		t.Errorf("topic group/order = %q/%d, want platform/32", topic.Group, topic.Order)
	}
	if topic.ID != architectureID {
		t.Errorf("topic id = %q, want %q", topic.ID, architectureID)
	}
}

func TestArchitectureSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(architectureSectionIDs))
	for _, section := range builtArchitectureTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(architectureSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, architectureSectionIDs)
	}
}

func TestArchitectureBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtArchitectureTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestArchitectureDemoSitsInlineInItsTeachingSection pins the family
// anatomy: the arch:tracer demo lives in the section that teaches the
// request-flow tracer, co-present with prose and sourced code — never
// queued at the end.
func TestArchitectureDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtArchitectureTopic(t)

	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "request-flow-tracer" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatalf("section request-flow-tracer missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "arch:tracer" {
		t.Errorf("request-flow-tracer demo blocks = %+v, want exactly arch:tracer", demos)
	}
	for _, kind := range []string{KindText, KindCode, KindDemo} {
		if !kinds[kind] {
			t.Errorf("section request-flow-tracer lacks a %q block; kinds = %v", kind, kinds)
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

func TestArchitectureClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtArchitectureTopic(t).Sections
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

func TestArchitectureProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtArchitectureTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minArchitectureProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minArchitectureProseWords)
	}
	t.Logf("architecture prose word count: %d", words)
}

func TestArchitectureMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtArchitectureTopic(t).Sections {
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

func TestArchitectureSnippetSourcesResolve(t *testing.T) {
	topic := builtArchitectureTopic(t)
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
