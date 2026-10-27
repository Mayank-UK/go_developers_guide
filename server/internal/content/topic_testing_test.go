package content

import (
	"strings"
	"testing"
)

// Topic 34 testing — pinned the same way every per-topic test pins its
// topic: exact section order, unique block ids, the demo inline in the
// section that teaches it (prose + code + demo co-present), the bash
// terminal companion last, the family prose floor, the must-source
// belt, and the full snippet ratchet on a one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 34 and group
// "platform" on the built topic itself.

const testingID = "testing"

var testingSectionIDs = []string{
	"pyramid-honestly",
	"table-driven-tests",
	"httptest-handlers",
	"testcontainers",
	"awkward-bits",
	"client-testing",
	"coverage",
	"ci-contract",
	"browser-e2e",
	"terminal-companion",
}

// minTestingProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only.
const minTestingProseWords = 2500

func builtTestingTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := testingTopic()
	if err != nil {
		t.Fatalf("testing builder must build cleanly: %v", err)
	}
	return topic
}

func registeredTestingTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtTestingTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "platform", Title: "Platform", Order: 8}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestTestingIsPlatformOrderThirtyFour(t *testing.T) {
	topic := registeredTestingTopic(t)
	if topic.Group != "platform" || topic.Order != 34 {
		t.Errorf("topic group/order = %q/%d, want platform/34", topic.Group, topic.Order)
	}
	if topic.ID != testingID {
		t.Errorf("topic id = %q, want %q", topic.ID, testingID)
	}
}

func TestTestingSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(testingSectionIDs))
	for _, section := range builtTestingTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(testingSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, testingSectionIDs)
	}
}

func TestTestingBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtTestingTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestTestingDemoSitsInlineInItsTeachingSection pins the family
// anatomy: the curriculum row pins testing:runner to the
// testcontainers section, co-present with prose and sourced code.
func TestTestingDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtTestingTopic(t)
	wants := map[string]string{
		"testcontainers": "testing:runner",
	}

	total := 0
	for _, section := range topic.Sections {
		for _, block := range section.Blocks {
			if demo, ok := block.(DemoBlock); ok {
				total++
				want, teachable := wants[section.ID]
				if !teachable || demo.ID != want {
					t.Errorf("demo %q sits in section %q; want %q", demo.ID, section.ID, wants[section.ID])
				}
			}
		}
	}
	if total != 1 {
		t.Errorf("topic carries %d demos, want exactly 1", total)
	}

	for sectionID := range wants {
		var section *Section
		for i := range topic.Sections {
			if topic.Sections[i].ID == sectionID {
				section = &topic.Sections[i]
				break
			}
		}
		if section == nil {
			t.Fatalf("section %s missing", sectionID)
		}
		kinds := make(map[string]bool)
		for _, block := range section.Blocks {
			kinds[blockKindTag(block)] = true
		}
		for _, kind := range []string{KindText, KindCode, KindDemo} {
			if !kinds[kind] {
				t.Errorf("section %s lacks a %q block; kinds = %v", sectionID, kind, kinds)
			}
		}
	}
}

func TestTestingClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtTestingTopic(t).Sections
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
	if codeBlocks < 2 {
		t.Error("terminal companion needs its curl tour blocks")
	}
}

func TestTestingProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtTestingTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minTestingProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minTestingProseWords)
	}
	t.Logf("testing prose word count: %d", words)
}

func TestTestingMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtTestingTopic(t).Sections {
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

func TestTestingSnippetSourcesResolve(t *testing.T) {
	topic := builtTestingTopic(t)
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

// TestTestingQuotesTheSuitesOwnTests pins the topic's thesis: the
// subject of the testing topic is THIS repo's test code, so the quoted
// sources must include server test files, client spec files, and the
// demo's own engine — a drift away from that subject fails here.
func TestTestingQuotesTheSuitesOwnTests(t *testing.T) {
	sources := make(map[string]bool)
	for _, section := range builtTestingTopic(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && code.Source != "" {
				sources[code.Source] = true
			}
		}
	}
	for _, want := range []string{
		"server/internal/core/config_parsing_test.go",
		"server/internal/presentation/posts_test.go",
		"server/internal/data/db/testcontainers_smoke_test.go",
		"client/src/data/demo/obs-trace-dto.spec.ts",
		"client/src/ui/demos/obs/trace_demo.spec.tsx",
		"server/internal/domain/demos/testing_runner.go",
	} {
		if !sources[want] {
			t.Errorf("topic must quote its own suite's %s", want)
		}
	}
}
