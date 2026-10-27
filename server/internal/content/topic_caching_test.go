package content

import (
	"strings"
	"testing"
)

// Topic 21 caching — pinned the same way every per-topic test pins its
// topic: exact section order, unique block ids, the demo inline in the
// section that teaches cache-aside (prose + code + demo co-present), the
// bash terminal companion last, the family prose floor, the must-source
// belt, and the full snippet ratchet on a one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 21 and group
// "state-auth" on the built topic itself.

const cachingID = "caching"

var cachingSectionIDs = []string{
	"two-worlds",
	"http-caching",
	"cache-aside",
	"invalidation",
	"stampedes",
	"cdn-hashed-assets",
	"what-not-to-cache",
	"terminal-companion",
}

// minCachingProseWords is the family depth mandate: ~3k words per topic,
// floor 2500, counted over text blocks only — list items and code never
// count.
const minCachingProseWords = 2500

func builtCachingTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := cachingTopic()
	if err != nil {
		t.Fatalf("caching builder must build cleanly: %v", err)
	}
	return topic
}

func registeredCachingTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtCachingTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "state-auth", Title: "State & auth", Order: 6}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestCachingIsStateAuthOrderTwentyOne(t *testing.T) {
	topic := registeredCachingTopic(t)
	if topic.Group != "state-auth" || topic.Order != 21 {
		t.Errorf("topic group/order = %q/%d, want state-auth/21", topic.Group, topic.Order)
	}
	if topic.ID != cachingID {
		t.Errorf("topic id = %q, want %q", topic.ID, cachingID)
	}
}

func TestCachingSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(cachingSectionIDs))
	for _, section := range builtCachingTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(cachingSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, cachingSectionIDs)
	}
}

func TestCachingBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtCachingTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestCachingDemoSitsInlineInTheCacheAsideSection(t *testing.T) {
	topic := builtCachingTopic(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "cache-aside" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("cache-aside section missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "caching:lab" {
		t.Errorf("demo blocks = %+v, want exactly caching:lab", demos)
	}
	// Inline means beside the teaching: the section carries prose, code,
	// and the demo together.
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("cache-aside section lacks a %q block; kinds = %v", want, kinds)
		}
	}
}

func TestCachingClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtCachingTopic(t).Sections
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

func TestCachingProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtCachingTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minCachingProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minCachingProseWords)
	}
	t.Logf("caching prose word count: %d", words)
}

func TestCachingMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source language
	// with an empty source path — fails fast here too, at the topic layer,
	// instead of only in the file-reading ratchet.
	for _, section := range builtCachingTopic(t).Sections {
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

func TestCachingSnippetSourcesResolve(t *testing.T) {
	// The full snippet ratchet (RATCHET = 0), run against this topic's own
	// registry until production registration lands: every must-source block
	// must quote a real repo file, in order, after normalization.
	topic := builtCachingTopic(t)
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "state-auth", Title: "State & auth", Order: 6}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}
