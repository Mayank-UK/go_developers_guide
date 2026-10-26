package content

import (
	"strings"
	"testing"
)

// Topic 20 redis — pinned the same way every per-topic test pins its
// topic: exact section order, unique block ids, the demo inline in the
// section that teaches the structures (prose + code + demo co-present),
// the bash terminal companion last, the family prose floor, the
// must-source belt, and the full snippet ratchet on a one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 20 and group
// "data" on the built topic itself.

const redisID = "redis"

var redisSectionIDs = []string{
	"what-redis-is",
	"optional-service-discipline",
	"go-redis-structures",
	"ttl-and-expiry",
	"atomic-operations",
	"pubsub",
	"when-a-second-store",
	"terminal-companion",
}

// minRedisProseWords is the family depth mandate: ~3k words per topic,
// floor 2500, counted over text blocks only — list items and code never
// count.
const minRedisProseWords = 2500

func builtRedisTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := redisTopic()
	if err != nil {
		t.Fatalf("redis builder must build cleanly: %v", err)
	}
	return topic
}

func registeredRedisTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtRedisTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "data", Title: "Data", Order: 5}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestRedisIsDataOrderTwenty(t *testing.T) {
	topic := registeredRedisTopic(t)
	if topic.Group != "data" || topic.Order != 20 {
		t.Errorf("topic group/order = %q/%d, want data/20", topic.Group, topic.Order)
	}
	if topic.ID != redisID {
		t.Errorf("topic id = %q, want %q", topic.ID, redisID)
	}
}

func TestRedisSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(redisSectionIDs))
	for _, section := range builtRedisTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(redisSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, redisSectionIDs)
	}
}

func TestRedisBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtRedisTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestRedisDemoSitsInlineInTheStructuresSection(t *testing.T) {
	topic := builtRedisTopic(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "go-redis-structures" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("go-redis-structures section missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "redis:playground" {
		t.Errorf("demo blocks = %+v, want exactly redis:playground", demos)
	}
	// Inline means beside the teaching: the section carries prose, code,
	// and the demo together.
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("go-redis-structures section lacks a %q block; kinds = %v", want, kinds)
		}
	}
}

func TestRedisClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtRedisTopic(t).Sections
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
		t.Error("terminal companion needs its curl and redis-cli tour")
	}
}

func TestRedisProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtRedisTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minRedisProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minRedisProseWords)
	}
	t.Logf("redis prose word count: %d", words)
}

func TestRedisMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source language
	// with an empty source path — fails fast here too, at the topic layer,
	// instead of only in the file-reading ratchet.
	for _, section := range builtRedisTopic(t).Sections {
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

func TestRedisSnippetSourcesResolve(t *testing.T) {
	// The full snippet ratchet (RATCHET = 0), run against this topic's own
	// registry until production registration lands: every must-source block
	// must quote a real repo file, in order, after normalization.
	topic := builtRedisTopic(t)
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "data", Title: "Data", Order: 5}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}
