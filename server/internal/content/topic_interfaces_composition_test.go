package content

import (
	"strings"
	"testing"
)

// Topic 05 interfaces-and-composition — the GO-NEW topic, pinned here the
// same way the exemplar pins topic 01: exact section order, unique block
// ids, the inline demo in the section that teaches satisfaction/typing, the
// terminal companion last, the family prose floor, and the must-source belt
// (the snippet-sources ratchet in snippet_sources_test.go verifies every
// quoted line against the real file; this belt catches the authoring
// mistake of a must-source block with no source path at all).
//
// The pins run against the BUILDER through NewRegistry — production
// registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic), so this file asserts Order 5 and group
// "framework" on the built topic itself rather than on the production set.

const interfacesCompositionID = "interfaces-and-composition"

var interfacesCompositionSectionIDs = []string{
	"composition-over-inheritance",
	"small-interfaces",
	"embedding",
	"accept-interfaces-return-structs",
	"type-switches-assertions",
	"satisfaction-matrix",
	"generics-done-right",
	"terminal-companion",
}

// minInterfacesCompositionProseWords is the family depth mandate: ~3k words
// per topic, floor 2500, counted over text blocks only — list items and
// code never count.
const minInterfacesCompositionProseWords = 2500

// builtInterfacesComposition builds the topic directly; the builder must
// assemble every block without a single constructor error.
func builtInterfacesComposition(t *testing.T) Topic {
	t.Helper()
	topic, err := interfacesCompositionTopic()
	if err != nil {
		t.Fatalf("interfaces-and-composition builder must build cleanly: %v", err)
	}
	return topic
}

// registeredInterfacesComposition validates the builder's output through
// the registry's own rules (the real production gate) on a one-topic
// registry carrying the topic's curriculum group.
func registeredInterfacesComposition(t *testing.T) Topic {
	t.Helper()
	topic := builtInterfacesComposition(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "framework", Title: "Framework", Order: 2}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestInterfacesCompositionIsFrameworkOrderFive(t *testing.T) {
	topic := registeredInterfacesComposition(t)
	if topic.Group != "framework" || topic.Order != 5 {
		t.Errorf("topic group/order = %q/%d, want framework/5", topic.Group, topic.Order)
	}
	if topic.ID != interfacesCompositionID {
		t.Errorf("topic id = %q, want %q", topic.ID, interfacesCompositionID)
	}
}

func TestInterfacesCompositionSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(interfacesCompositionSectionIDs))
	for _, section := range builtInterfacesComposition(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(interfacesCompositionSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, interfacesCompositionSectionIDs)
	}
}

func TestInterfacesCompositionBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtInterfacesComposition(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestInterfacesCompositionDemoSitsInlineInSatisfactionMatrixSection(t *testing.T) {
	topic := builtInterfacesComposition(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "satisfaction-matrix" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("satisfaction-matrix section missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "iface:matrix" {
		t.Errorf("demo blocks = %+v, want exactly iface:matrix", demos)
	}
	// Inline means beside the teaching: the section carries prose, code,
	// and the demo together.
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("satisfaction-matrix section lacks a %q block; kinds = %v", want, kinds)
		}
	}
}

func TestInterfacesCompositionClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtInterfacesComposition(t).Sections
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

func TestInterfacesCompositionProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtInterfacesComposition(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minInterfacesCompositionProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minInterfacesCompositionProseWords)
	}
	t.Logf("interfaces-and-composition prose word count: %d", words)
}

func TestInterfacesCompositionMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source language
	// with an empty source path — fails fast here too, at the topic layer,
	// instead of only in the file-reading ratchet.
	for _, section := range builtInterfacesComposition(t).Sections {
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

func TestInterfacesCompositionSnippetSourcesResolve(t *testing.T) {
	// The full snippet ratchet (RATCHET = 0), run against this topic's own
	// registry until production registration lands: every must-source block
	// must quote a real repo file, in order, after normalization.
	topic := builtInterfacesComposition(t)
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "framework", Title: "Framework", Order: 2}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}
