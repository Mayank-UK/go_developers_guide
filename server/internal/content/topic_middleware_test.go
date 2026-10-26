package content

import (
	"strings"
	"testing"
)

// Topic 08 middleware — pinned the same way the exemplar pins topic 01 and
// topic 05 pins itself: exact section order, unique block ids, the inline
// demo in the section that teaches the chain and its ordering, the bash
// terminal companion last (no source), the family prose floor, and the
// must-source belt (the snippet-sources ratchet in snippet_sources_test.go
// verifies every quoted line against the real file; this belt catches the
// authoring mistake of a must-source block with no source path at all).
//
// The pins run against the BUILDER through NewRegistry — production
// registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic; the wave's shared wiring owns it), so
// this file asserts Order 8 and group "request-handling" on the built
// topic itself rather than on the production set.

const middlewareID = "middleware"

var middlewareSectionIDs = []string{
	"the-chain-and-its-ordering",
	"request-id-and-logging",
	"panic-recovery",
	"timeouts",
	"hand-rolled-vs-chi",
	"terminal-companion",
}

// minMiddlewareProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only — list items and code
// never count.
const minMiddlewareProseWords = 2500

// builtMiddleware builds the topic directly; the builder must assemble
// every block without a single constructor error.
func builtMiddleware(t *testing.T) Topic {
	t.Helper()
	topic, err := middlewareTopic()
	if err != nil {
		t.Fatalf("middleware builder must build cleanly: %v", err)
	}
	return topic
}

// registeredMiddleware validates the builder's output through the
// registry's own rules (the real production gate) on a one-topic registry
// carrying the topic's curriculum group.
func registeredMiddleware(t *testing.T) Topic {
	t.Helper()
	topic := builtMiddleware(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "request-handling", Title: "Request handling", Order: 3}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestMiddlewareIsRequestHandlingOrderEight(t *testing.T) {
	topic := registeredMiddleware(t)
	if topic.Group != "request-handling" || topic.Order != 8 {
		t.Errorf("topic group/order = %q/%d, want request-handling/8", topic.Group, topic.Order)
	}
	if topic.ID != middlewareID {
		t.Errorf("topic id = %q, want %q", topic.ID, middlewareID)
	}
}

func TestMiddlewareSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(middlewareSectionIDs))
	for _, section := range builtMiddleware(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(middlewareSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, middlewareSectionIDs)
	}
}

func TestMiddlewareBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtMiddleware(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestMiddlewareDemoSitsInlineInTheChainSection(t *testing.T) {
	topic := builtMiddleware(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "the-chain-and-its-ordering" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("the-chain-and-its-ordering section missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "middleware:pipeline" {
		t.Errorf("demo blocks = %+v, want exactly middleware:pipeline", demos)
	}
	// Inline means beside the teaching: the section carries prose, code,
	// and the demo together.
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("the-chain-and-its-ordering section lacks a %q block; kinds = %v", want, kinds)
		}
	}
}

func TestMiddlewareClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtMiddleware(t).Sections
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

func TestMiddlewareProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtMiddleware(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minMiddlewareProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minMiddlewareProseWords)
	}
	t.Logf("middleware prose word count: %d", words)
}

func TestMiddlewareMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source language
	// with an empty source path — fails fast here too, at the topic layer,
	// instead of only in the file-reading ratchet.
	for _, section := range builtMiddleware(t).Sections {
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

func TestMiddlewareSnippetSourcesResolve(t *testing.T) {
	// The full snippet ratchet (RATCHET = 0), run against this topic's own
	// registry until production registration lands: every must-source block
	// must quote a real repo file, in order, after normalization.
	topic := builtMiddleware(t)
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
