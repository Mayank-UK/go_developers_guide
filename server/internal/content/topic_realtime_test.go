package content

import (
	"strings"
	"testing"
)

// Topic 28 realtime — pinned the same way every per-topic test pins its
// topic: exact section order, unique block ids, the TWO demos inline in
// the sections that teach them (prose + code + demo co-present), the
// bash terminal companion last, the family prose floor, the must-source
// belt, and the full snippet ratchet on a one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 28 and group
// "media-realtime" on the built topic itself.

const realtimeID = "realtime"

var realtimeSectionIDs = []string{
	"push-models",
	"sse-streaming",
	"event-stream-mechanics",
	"websockets",
	"rooms-and-ceiling",
	"reconnection-semantics",
	"terminal-companion",
}

// minRealtimeProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only.
const minRealtimeProseWords = 2500

func builtRealtimeTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := realtimeTopic()
	if err != nil {
		t.Fatalf("realtime builder must build cleanly: %v", err)
	}
	return topic
}

func registeredRealtimeTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtRealtimeTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "media-realtime", Title: "Media & realtime", Order: 7}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestRealtimeIsMediaRealtimeOrderTwentyEight(t *testing.T) {
	topic := registeredRealtimeTopic(t)
	if topic.Group != "media-realtime" || topic.Order != 28 {
		t.Errorf("topic group/order = %q/%d, want media-realtime/28", topic.Group, topic.Order)
	}
	if topic.ID != realtimeID {
		t.Errorf("topic id = %q, want %q", topic.ID, realtimeID)
	}
}

func TestRealtimeSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(realtimeSectionIDs))
	for _, section := range builtRealtimeTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(realtimeSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, realtimeSectionIDs)
	}
}

func TestRealtimeBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtRealtimeTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestRealtimeDemosSitInlineInTheirTeachingSections pins the family
// anatomy for a two-demo topic: the SSE demo lives in the section that
// teaches SSE, the WS demo in the section that teaches WebSockets, each
// co-present with prose and sourced code — never queued at the end.
func TestRealtimeDemosSitInlineInTheirTeachingSections(t *testing.T) {
	topic := builtRealtimeTopic(t)
	for _, tc := range []struct {
		sectionID string
		demoID    string
	}{
		{"sse-streaming", "realtime:ticker"},
		{"websockets", "realtime:chat"},
	} {
		var section *Section
		for i := range topic.Sections {
			if topic.Sections[i].ID == tc.sectionID {
				section = &topic.Sections[i]
				break
			}
		}
		if section == nil {
			t.Fatalf("section %s missing", tc.sectionID)
		}
		var demos []DemoBlock
		kinds := make(map[string]bool)
		for _, block := range section.Blocks {
			if demo, ok := block.(DemoBlock); ok {
				demos = append(demos, demo)
			}
			kinds[blockKindTag(block)] = true
		}
		if len(demos) != 1 || demos[0].ID != tc.demoID {
			t.Errorf("section %s demo blocks = %+v, want exactly %s", tc.sectionID, demos, tc.demoID)
		}
		for _, kind := range []string{KindText, KindCode, KindDemo} {
			if !kinds[kind] {
				t.Errorf("section %s lacks a %q block; kinds = %v", tc.sectionID, kind, kinds)
			}
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
	if total != 2 {
		t.Errorf("topic carries %d demos, want exactly 2", total)
	}
}

func TestRealtimeClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtRealtimeTopic(t).Sections
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

func TestRealtimeProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtRealtimeTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minRealtimeProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minRealtimeProseWords)
	}
	t.Logf("realtime prose word count: %d", words)
}

func TestRealtimeMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtRealtimeTopic(t).Sections {
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

func TestRealtimeSnippetSourcesResolve(t *testing.T) {
	topic := builtRealtimeTopic(t)
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
