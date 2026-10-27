package content

import (
	"strings"
	"testing"
)

// Topic 35 debugging — pinned the same way every per-topic test pins its
// topic: exact section order, unique block ids, the demo inline in the
// section that teaches it (prose + code + demo co-present), the bash
// terminal companion last, the family prose floor, the must-source
// belt, and the full snippet ratchet on a one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 35 and group
// "platform" on the built topic itself.

const debuggingID = "debugging"

var debuggingSectionIDs = []string{
	"mindset",
	"print-discipline",
	"delve",
	"pprof-profiles",
	"execution-tracer",
	"production-debugging",
	"process-nets",
	"terminal-companion",
}

// minDebuggingProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only.
const minDebuggingProseWords = 2500

func builtDebuggingTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := debuggingTopic()
	if err != nil {
		t.Fatalf("debugging builder must build cleanly: %v", err)
	}
	return topic
}

func registeredDebuggingTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtDebuggingTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "platform", Title: "Platform", Order: 8}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestDebuggingIsPlatformOrderThirtyFive(t *testing.T) {
	topic := registeredDebuggingTopic(t)
	if topic.Group != "platform" || topic.Order != 35 {
		t.Errorf("topic group/order = %q/%d, want platform/35", topic.Group, topic.Order)
	}
	if topic.ID != debuggingID {
		t.Errorf("topic id = %q, want %q", topic.ID, debuggingID)
	}
}

func TestDebuggingSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(debuggingSectionIDs))
	for _, section := range builtDebuggingTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(debuggingSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, debuggingSectionIDs)
	}
}

func TestDebuggingBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtDebuggingTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestDebuggingDemoSitsInlineInItsTeachingSection pins the family
// anatomy: the curriculum row pins debug:stall to the pprof profiles
// section, co-present with prose and sourced code.
func TestDebuggingDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtDebuggingTopic(t)
	wants := map[string]string{
		"pprof-profiles": "debug:stall",
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

func TestDebuggingClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtDebuggingTopic(t).Sections
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

// TestDebuggingTerminalCompanionRunsARealPprofRead pins the companion's
// promise: the tour includes a real profile capture and a
// `go tool pprof -top` invocation against the running stack.
func TestDebuggingTerminalCompanionRunsARealPprofRead(t *testing.T) {
	sections := builtDebuggingTopic(t).Sections
	last := sections[len(sections)-1]
	joined := ""
	for _, block := range last.Blocks {
		if code, ok := block.(CodeBlock); ok {
			joined += code.Snippet + "\n"
		}
	}
	for _, want := range []string{
		"/internal/debug/pprof/profile?seconds=",
		"go tool pprof -top",
		"/internal/debug/pprof/goroutine",
		"/internal/debug/pprof/heap",
		"/internal/debug/pprof/trace",
		"/api/ready",
		"/api/demo/debug/stall",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("terminal companion must exercise %q against the live stack", want)
		}
	}
}

func TestDebuggingProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtDebuggingTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minDebuggingProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minDebuggingProseWords)
	}
	t.Logf("debugging prose word count: %d", words)
}

func TestDebuggingMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtDebuggingTopic(t).Sections {
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

func TestDebuggingSnippetSourcesResolve(t *testing.T) {
	topic := builtDebuggingTopic(t)
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

// TestDebuggingQuotesTheRealInstruments pins the topic's thesis: the
// subject of the debugging topic is THIS repo's real machinery — the
// topic 33 pprof mount, the demo's own wedge/capture/parser, the
// production middleware, the composition root, and the honest handler
// — so a drift away from that subject fails here.
func TestDebuggingQuotesTheRealInstruments(t *testing.T) {
	sources := make(map[string]bool)
	for _, section := range builtDebuggingTopic(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && code.Source != "" {
				sources[code.Source] = true
			}
		}
	}
	for _, want := range []string{
		"server/internal/presentation/pprof.go",
		"server/internal/domain/demos/debug_stall.go",
		"server/internal/presentation/demo_debug.go",
		"server/internal/presentation/middleware.go",
		"server/cmd/guide/main.go",
	} {
		if !sources[want] {
			t.Errorf("topic must quote its real instrument %s", want)
		}
	}
}
