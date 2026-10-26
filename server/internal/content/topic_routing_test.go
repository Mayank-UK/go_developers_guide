package content

import (
	"strings"
	"testing"
)

// Topic 07 routing — pinned the same way the exemplar pins topic 01: exact
// section order, unique block ids, the inline demo in the section that
// teaches matching, the terminal companion last, the family prose floor,
// and the must-source belt (the snippet-sources ratchet in
// snippet_sources_test.go verifies every quoted line against the real
// file; this belt catches the authoring mistake of a must-source block
// with no source path at all).
//
// The pins run against the BUILDER through NewRegistry — production
// registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic), so this file asserts Order 7 and group
// "request-handling" on the built topic itself rather than on the
// production set.

const routingTopicID = "routing"

var routingSectionIDs = []string{
	"route-table",
	"wildcards",
	"query-parsing",
	"matching",
	"mounting",
	"routing-at-scale",
	"terminal-companion",
}

// minRoutingProseWords is the family depth mandate: ~3k words per topic,
// floor 2500, counted over text blocks only — list items and code never
// count.
const minRoutingProseWords = 2500

// builtRouting builds the topic directly; the builder must assemble every
// block without a single constructor error.
func builtRouting(t *testing.T) Topic {
	t.Helper()
	topic, err := routingTopic()
	if err != nil {
		t.Fatalf("routing builder must build cleanly: %v", err)
	}
	return topic
}

// registeredRouting validates the builder's output through the registry's
// own rules (the real production gate) on a one-topic registry carrying
// the topic's curriculum group.
func registeredRouting(t *testing.T) Topic {
	t.Helper()
	topic := builtRouting(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "request-handling", Title: "Request handling", Order: 3}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestRoutingIsRequestHandlingOrderSeven(t *testing.T) {
	topic := registeredRouting(t)
	if topic.Group != "request-handling" || topic.Order != 7 {
		t.Errorf("topic group/order = %q/%d, want request-handling/7", topic.Group, topic.Order)
	}
	if topic.ID != routingTopicID {
		t.Errorf("topic id = %q, want %q", topic.ID, routingTopicID)
	}
}

func TestRoutingSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(routingSectionIDs))
	for _, section := range builtRouting(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(routingSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, routingSectionIDs)
	}
}

func TestRoutingBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtRouting(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestRoutingDemoSitsInlineInMatchingSection(t *testing.T) {
	topic := builtRouting(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "matching" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("matching section missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "routing:explorer" {
		t.Errorf("demo blocks = %+v, want exactly routing:explorer", demos)
	}
	// Inline means beside the teaching: the section carries prose, code,
	// and the demo together.
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("matching section lacks a %q block; kinds = %v", want, kinds)
		}
	}
}

func TestRoutingClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtRouting(t).Sections
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

func TestRoutingProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtRouting(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minRoutingProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minRoutingProseWords)
	}
	t.Logf("routing prose word count: %d", words)
}

func TestRoutingMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source language
	// with an empty source path — fails fast here too, at the topic layer,
	// instead of only in the file-reading ratchet.
	for _, section := range builtRouting(t).Sections {
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

func TestRoutingSnippetSourcesResolve(t *testing.T) {
	// The full snippet ratchet (RATCHET = 0), run against this topic's own
	// registry until production registration lands: every must-source block
	// must quote a real repo file, in order, after normalization.
	topic := builtRouting(t)
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "request-handling", Title: "Request handling", Order: 3}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}
