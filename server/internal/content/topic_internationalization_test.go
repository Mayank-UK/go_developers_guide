package content

import (
	"strings"
	"testing"
)

// Topic 27 internationalization — pinned the same way every per-topic
// test pins its topic: exact section order, unique block ids, the ONE
// demo inline in the section that teaches it (prose + code + demo
// co-present), the bash terminal companion last, the family prose
// floor, the must-source belt, and the full snippet ratchet on a
// one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 27 and group
// "media-realtime" on the built topic itself.

const internationalizationID = "internationalization"

var internationalizationSectionIDs = []string{
	"global-by-default",
	"accept-language-negotiation",
	"message-catalogs",
	"formatting-numbers-dates",
	"timezones-vs-locales",
	"translation-workflow",
	"terminal-companion",
}

// minInternationalizationProseWords is the family depth mandate: ~3k
// words per topic, floor 2500, counted over text blocks only.
const minInternationalizationProseWords = 2500

func builtInternationalizationTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := internationalizationTopic()
	if err != nil {
		t.Fatalf("internationalization builder must build cleanly: %v", err)
	}
	return topic
}

func registeredInternationalizationTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtInternationalizationTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "media-realtime", Title: "Media & realtime", Order: 7}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestInternationalizationIsMediaRealtimeOrderTwentySeven(t *testing.T) {
	topic := registeredInternationalizationTopic(t)
	if topic.Group != "media-realtime" || topic.Order != 27 {
		t.Errorf("topic group/order = %q/%d, want media-realtime/27", topic.Group, topic.Order)
	}
	if topic.ID != internationalizationID {
		t.Errorf("topic id = %q, want %q", topic.ID, internationalizationID)
	}
}

func TestInternationalizationSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(internationalizationSectionIDs))
	for _, section := range builtInternationalizationTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(internationalizationSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, internationalizationSectionIDs)
	}
}

func TestInternationalizationBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtInternationalizationTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestInternationalizationDemoSitsInlineInItsTeachingSection pins the
// family anatomy: the ONE demo sits inside the section that teaches it,
// with prose and sourced code co-present — never queued at the topic's
// end.
func TestInternationalizationDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtInternationalizationTopic(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "accept-language-negotiation" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("section accept-language-negotiation missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "i18n:negotiator" {
		t.Errorf("demo blocks = %+v, want exactly i18n:negotiator", demos)
	}
	for _, kind := range []string{KindText, KindCode, KindDemo} {
		if !kinds[kind] {
			t.Errorf("section accept-language-negotiation lacks a %q block; kinds = %v", kind, kinds)
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

func TestInternationalizationClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtInternationalizationTopic(t).Sections
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

func TestInternationalizationProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtInternationalizationTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minInternationalizationProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minInternationalizationProseWords)
	}
	t.Logf("internationalization prose word count: %d", words)
}

func TestInternationalizationMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtInternationalizationTopic(t).Sections {
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

func TestInternationalizationSnippetSourcesResolve(t *testing.T) {
	topic := builtInternationalizationTopic(t)
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "media-realtime", Title: "Media & realtime", Order: 7}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}
