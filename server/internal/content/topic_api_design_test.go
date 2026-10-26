package content

import (
	"strings"
	"testing"
)

// Topic 11 api-design — pinned the same way the exemplar pins topic 01:
// exact section order, unique block ids, the inline demo in the section
// that teaches the whole pipeline, the terminal companion last, the family
// prose floor, and the must-source belt (the snippet-sources ratchet in
// snippet_sources_test.go verifies every quoted line against the real
// file; this belt catches the authoring mistake of a must-source block
// with no source path at all).
//
// The pins run against the BUILDER through NewRegistry — production
// registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic), so this file asserts Order 11 and group
// "apis" on the built topic itself rather than on the production set.

const apiDesignID = "api-design"

var apiDesignSectionIDs = []string{
	"what-rest-is",
	"designing-resources",
	"methods-and-status",
	"urls-pagination-filtering",
	"error-responses",
	"versioning",
	"the-whole-pipeline",
	"terminal-companion",
}

// minAPIDesignProseWords is the family depth mandate: ~3k words per topic,
// floor 2500, counted over text blocks only — list items and code never
// count.
const minAPIDesignProseWords = 2500

// builtAPIDesign builds the topic directly; the builder must assemble
// every block without a single constructor error.
func builtAPIDesign(t *testing.T) Topic {
	t.Helper()
	topic, err := apiDesignTopic()
	if err != nil {
		t.Fatalf("api-design builder must build cleanly: %v", err)
	}
	return topic
}

// registeredAPIDesign validates the builder's output through the
// registry's own rules (the real production gate) on a one-topic registry
// carrying the topic's curriculum group.
func registeredAPIDesign(t *testing.T) Topic {
	t.Helper()
	topic := builtAPIDesign(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "apis", Title: "APIs", Order: 4}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestAPIDesignIsAPIsOrderEleven(t *testing.T) {
	topic := registeredAPIDesign(t)
	if topic.Group != "apis" || topic.Order != 11 {
		t.Errorf("topic group/order = %q/%d, want apis/11", topic.Group, topic.Order)
	}
	if topic.ID != apiDesignID {
		t.Errorf("topic id = %q, want %q", topic.ID, apiDesignID)
	}
}

func TestAPIDesignSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(apiDesignSectionIDs))
	for _, section := range builtAPIDesign(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(apiDesignSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, apiDesignSectionIDs)
	}
}

func TestAPIDesignBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtAPIDesign(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestAPIDesignDemoSitsInlineInTheWholePipelineSection(t *testing.T) {
	topic := builtAPIDesign(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "the-whole-pipeline" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("the-whole-pipeline section missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "api:posts-workbench" {
		t.Errorf("demo blocks = %+v, want exactly api:posts-workbench", demos)
	}
	// Inline means beside the teaching: the section carries prose, code,
	// and the demo together.
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("the-whole-pipeline section lacks a %q block; kinds = %v", want, kinds)
		}
	}
}

func TestAPIDesignClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtAPIDesign(t).Sections
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
		t.Errorf("terminal companion needs its CRUD curl tour, got %d bash blocks", codeBlocks)
	}
}

func TestAPIDesignProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtAPIDesign(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minAPIDesignProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minAPIDesignProseWords)
	}
	t.Logf("api-design prose word count: %d", words)
}

func TestAPIDesignMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source language
	// with an empty source path — fails fast here too, at the topic layer,
	// instead of only in the file-reading ratchet.
	for _, section := range builtAPIDesign(t).Sections {
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

func TestAPIDesignSnippetSourcesResolve(t *testing.T) {
	// The full snippet ratchet (RATCHET = 0), run against this topic's own
	// registry until production registration lands: every must-source block
	// must quote a real repo file, in order, after normalization.
	topic := builtAPIDesign(t)
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

func TestAPIDesignIsRegisteredInProduction(t *testing.T) {
	// Production wiring: the registry serves the topic, in curriculum
	// order 11, inside the apis group — the registration the client's
	// topic screen resolves through.
	registry, err := NewProductionRegistry()
	if err != nil {
		t.Fatalf("production registry must boot: %v", err)
	}
	topic, ok := registry.TopicByID(apiDesignID)
	if !ok {
		t.Fatal("api-design must be registered in NewProductionRegistry")
	}
	if topic.Order != 11 || topic.Group != "apis" {
		t.Errorf("registered api-design = %q/%d, want apis/11", topic.Group, topic.Order)
	}
}
