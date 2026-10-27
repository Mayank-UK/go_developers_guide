package content

import (
	"strings"
	"testing"
)

// Topic 33 observability — pinned the same way every per-topic test pins
// its topic: exact section order, unique block ids, the two demos inline
// in the sections that teach them (prose + code + demo co-present), the
// bash terminal companion last, the family prose floor, the must-source
// belt, and the full snippet ratchet on a one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 33 and group
// "platform" on the built topic itself.

const observabilityID = "observability"

var observabilitySectionIDs = []string{
	"three-questions",
	"slog-json",
	"request-logging",
	"levels",
	"health-readiness",
	"pprof-in-production",
	"prometheus-metrics",
	"trace-context",
	"alerting",
	"process-nets",
	"terminal-companion",
}

// minObservabilityProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only.
const minObservabilityProseWords = 2500

func builtObservabilityTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := observabilityTopic()
	if err != nil {
		t.Fatalf("observability builder must build cleanly: %v", err)
	}
	return topic
}

func registeredObservabilityTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtObservabilityTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "platform", Title: "Platform", Order: 8}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestObservabilityIsPlatformOrderThirtyThree(t *testing.T) {
	topic := registeredObservabilityTopic(t)
	if topic.Group != "platform" || topic.Order != 33 {
		t.Errorf("topic group/order = %q/%d, want platform/33", topic.Group, topic.Order)
	}
	if topic.ID != observabilityID {
		t.Errorf("topic id = %q, want %q", topic.ID, observabilityID)
	}
}

func TestObservabilitySectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(observabilitySectionIDs))
	for _, section := range builtObservabilityTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(observabilitySectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, observabilitySectionIDs)
	}
}

func TestObservabilityBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtObservabilityTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestObservabilityDemosSitInlineInTheirTeachingSections pins the family
// anatomy: obs:viewer lives in the slog JSON section, obs:trace in the
// trace-context section — each co-present with prose and sourced code,
// never queued at the end.
func TestObservabilityDemosSitInlineInTheirTeachingSections(t *testing.T) {
	topic := builtObservabilityTopic(t)
	wants := map[string]string{
		"slog-json":     "obs:viewer",
		"trace-context": "obs:trace",
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
	if total != 2 {
		t.Errorf("topic carries %d demos, want exactly 2", total)
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

func TestObservabilityClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtObservabilityTopic(t).Sections
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

func TestObservabilityProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtObservabilityTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minObservabilityProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minObservabilityProseWords)
	}
	t.Logf("observability prose word count: %d", words)
}

func TestObservabilityMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtObservabilityTopic(t).Sections {
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

func TestObservabilitySnippetSourcesResolve(t *testing.T) {
	topic := builtObservabilityTopic(t)
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
