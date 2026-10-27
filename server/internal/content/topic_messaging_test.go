package content

import (
	"strings"
	"testing"
)

// Topic 22 messaging — pinned the same way every per-topic test pins its
// topic: exact section order, unique block ids, both demos inline in the
// sections that teach them (prose + code + demo co-present), the bash
// terminal companion last, the family prose floor, the must-source belt,
// and the full snippet ratchet on a one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 22 and group
// "state-auth" on the built topic itself.

const messagingID = "messaging"

var messagingSectionIDs = []string{
	"why-messaging",
	"logs-vs-queues",
	"consumer-groups-and-ordering",
	"delivery-guarantees",
	"pending-and-autoclaim",
	"transactional-outbox",
	"idempotent-consumers",
	"kafka-rabbitmq",
	"terminal-companion",
}

// minMessagingProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only — list items and
// code never count.
const minMessagingProseWords = 2500

func builtMessagingTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := messagingTopic()
	if err != nil {
		t.Fatalf("messaging builder must build cleanly: %v", err)
	}
	return topic
}

func registeredMessagingTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtMessagingTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "state-auth", Title: "State & auth", Order: 6}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestMessagingIsStateAuthOrderTwentyTwo(t *testing.T) {
	topic := registeredMessagingTopic(t)
	if topic.Group != "state-auth" || topic.Order != 22 {
		t.Errorf("topic group/order = %q/%d, want state-auth/22", topic.Group, topic.Order)
	}
	if topic.ID != messagingID {
		t.Errorf("topic id = %q, want %q", topic.ID, messagingID)
	}
}

func TestMessagingSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(messagingSectionIDs))
	for _, section := range builtMessagingTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(messagingSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, messagingSectionIDs)
	}
}

func TestMessagingBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtMessagingTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestMessagingDemosSitInlineInTheirTeachingSections pins the family
// anatomy: each demo sits inside the section that teaches it, with prose
// and sourced code co-present — never queued at the topic's end.
func TestMessagingDemosSitInlineInTheirTeachingSections(t *testing.T) {
	topic := builtMessagingTopic(t)
	wants := []struct {
		sectionID string
		demoID    string
	}{
		{"consumer-groups-and-ordering", "msg:stream"},
		{"transactional-outbox", "msg:outbox"},
	}
	for _, want := range wants {
		var section *Section
		for i := range topic.Sections {
			if topic.Sections[i].ID == want.sectionID {
				section = &topic.Sections[i]
				break
			}
		}
		if section == nil {
			t.Fatalf("section %s missing", want.sectionID)
		}
		var demos []DemoBlock
		kinds := make(map[string]bool)
		for _, block := range section.Blocks {
			if demo, ok := block.(DemoBlock); ok {
				demos = append(demos, demo)
			}
			kinds[blockKindTag(block)] = true
		}
		if len(demos) != 1 || demos[0].ID != want.demoID {
			t.Errorf("section %s demo blocks = %+v, want exactly %s", want.sectionID, demos, want.demoID)
		}
		// Inline means beside the teaching: the section carries prose,
		// code, and the demo together.
		for _, kind := range []string{KindText, KindCode, KindDemo} {
			if !kinds[kind] {
				t.Errorf("section %s lacks a %q block; kinds = %v", want.sectionID, kind, kinds)
			}
		}
	}
	// And exactly two demos in the whole topic.
	total := 0
	for _, section := range topic.Sections {
		for _, block := range section.Blocks {
			if _, ok := block.(DemoBlock); ok {
				total++
			}
		}
	}
	if total != 2 {
		t.Errorf("topic carries %d demos, want exactly 2", total)
	}
}

func TestMessagingClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtMessagingTopic(t).Sections
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
		t.Error("terminal companion needs its redis-cli and curl tour")
	}
}

func TestMessagingProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtMessagingTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minMessagingProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minMessagingProseWords)
	}
	t.Logf("messaging prose word count: %d", words)
}

func TestMessagingMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source language
	// with an empty source path — fails fast here too, at the topic layer,
	// instead of only in the file-reading ratchet.
	for _, section := range builtMessagingTopic(t).Sections {
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

func TestMessagingSnippetSourcesResolve(t *testing.T) {
	// The full snippet ratchet (RATCHET = 0), run against this topic's own
	// registry until production registration lands: every must-source block
	// must quote a real repo file, in order, after normalization.
	topic := builtMessagingTopic(t)
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
