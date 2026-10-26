package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Topic 06 goroutines-and-channels pattern pins. The topic is NOT yet in
// NewProductionRegistry (wave wiring is master-owned), so every pin runs
// against a registry built from the BUILDER itself — the same validations
// the production registry will apply at wiring time, plus a quote belt:
// this topic quotes heavily from its own in-flight demo builders, so the
// must-source blocks are matched against the real files HERE, catching
// quote drift before the production ratchet ever sees the topic.

const goroutinesChannelsID = "goroutines-and-channels"

var goroutinesChannelsSectionIDs = []string{
	"concurrency-model",
	"goroutines-and-waitgroup",
	"channels",
	"select-and-timeouts",
	"context-cancellation",
	"errgroup",
	"race-detector",
	"terminal-companion",
}

// minGoroutinesChannelsProseWords is the family depth mandate: ~3k words
// per topic; the floor catches thin topics. Counted over text blocks only.
const minGoroutinesChannelsProseWords = 2500

func registeredGoroutinesChannels(t *testing.T) Topic {
	t.Helper()
	topic, err := goroutinesChannelsTopic()
	if err != nil {
		t.Fatalf("builder must assemble cleanly: %v", err)
	}
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "framework", Title: "Framework", Order: 2}},
	)
	if err != nil {
		t.Fatalf("topic must pass every registry validation: %v", err)
	}
	loaded, ok := registry.TopicByID(goroutinesChannelsID)
	if !ok {
		t.Fatalf("topic %q must register under its own id", goroutinesChannelsID)
	}
	return loaded
}

func TestGoroutinesChannelsIsFrameworkOrderSix(t *testing.T) {
	topic := registeredGoroutinesChannels(t)
	if topic.Group != "framework" || topic.Order != 6 {
		t.Errorf("topic group/order = %q/%d, want framework/6", topic.Group, topic.Order)
	}
	if len(topic.Sections) != len(goroutinesChannelsSectionIDs) {
		t.Errorf("section count = %d, want %d", len(topic.Sections), len(goroutinesChannelsSectionIDs))
	}
}

func TestGoroutinesChannelsSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(goroutinesChannelsSectionIDs))
	for _, section := range registeredGoroutinesChannels(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(goroutinesChannelsSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, goroutinesChannelsSectionIDs)
	}
}

func TestGoroutinesChannelsBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range registeredGoroutinesChannels(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// inlineDemoSection finds the section carrying a demo block id and checks
// the co-presence rule: prose, code, and the demo together, inline.
func inlineDemoSection(t *testing.T, topic Topic, sectionID, demoID string) Section {
	t.Helper()
	for _, section := range topic.Sections {
		for _, block := range section.Blocks {
			if demo, ok := block.(DemoBlock); ok && demo.ID == demoID {
				if section.ID != sectionID {
					t.Errorf("demo %q lives in section %q, want %q", demoID, section.ID, sectionID)
				}
				kinds := make(map[string]bool)
				for _, b := range section.Blocks {
					kinds[blockKindTag(b)] = true
				}
				for _, want := range []string{KindText, KindCode, KindDemo} {
					if !kinds[want] {
						t.Errorf("section %q lacks a %q block; kinds = %v", sectionID, want, kinds)
					}
				}
				return section
			}
		}
	}
	t.Fatalf("demo %q missing from the topic", demoID)
	return Section{}
}

func TestGoroutinesChannelsDemosAreInlineInTheirTeachingSections(t *testing.T) {
	topic := registeredGoroutinesChannels(t)
	inlineDemoSection(t, topic, "context-cancellation", "conc:context")
	inlineDemoSection(t, topic, "race-detector", "conc:race")

	// Exactly two demos, no strays.
	demos := 0
	for _, section := range topic.Sections {
		for _, block := range section.Blocks {
			if _, ok := block.(DemoBlock); ok {
				demos++
			}
		}
	}
	if demos != 2 {
		t.Errorf("demo blocks = %d, want exactly 2 (conc:context, conc:race)", demos)
	}
}

func TestGoroutinesChannelsClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := registeredGoroutinesChannels(t).Sections
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

func TestGoroutinesChannelsProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range registeredGoroutinesChannels(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minGoroutinesChannelsProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minGoroutinesChannelsProseWords)
	}
	t.Logf("goroutines-and-channels prose word count: %d", words)
}

// TestGoroutinesChannelsQuotesResolveAgainstRealFiles is the quote belt:
// every must-source block carries a source path under the repo root, the
// file exists, and the snippet matches it — the same rule the production
// ratchet will enforce the moment the topic is wired into the registry.
// The topic quotes its own demo builders, so drift fails here first.
func TestGoroutinesChannelsQuotesResolveAgainstRealFiles(t *testing.T) {
	root, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	checked := 0
	for _, section := range registeredGoroutinesChannels(t).Sections {
		for _, block := range section.Blocks {
			code, ok := block.(CodeBlock)
			if !ok || !MustSourceLanguages[strings.ToLower(code.Language)] {
				continue
			}
			if strings.TrimSpace(code.Source) == "" {
				t.Errorf("code block %q (language %q) lacks a source path", code.ID, code.Language)
				continue
			}
			path := filepath.Join(root, code.Source)
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Errorf("code block %q source not found: %s: %v", code.ID, code.Source, readErr)
				continue
			}
			if match, reason := MatchSnippet(string(raw), code.Snippet, code.Language); !match {
				t.Errorf("code block %q drifted from %s: %s", code.ID, code.Source, reason)
			}
			checked++
		}
	}
	if checked < 15 {
		t.Errorf("quote belt checked only %d must-source blocks — the topic quotes more", checked)
	}
}
