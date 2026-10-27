package content

import (
	"strings"
	"testing"
)

// Topic 26 files-media — pinned the same way every per-topic test pins
// its topic: exact section order, unique block ids, the ONE demo inline
// in the section that teaches it (prose + code + demo co-present), the
// bash terminal companion last, the family prose floor, the must-source
// belt, and the full snippet ratchet on a one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 26 and group
// "media-realtime" on the built topic itself.

const filesMediaID = "files-media"

var filesMediaSectionIDs = []string{
	"uploads-are-streams",
	"multipart-streaming",
	"downloads-and-disposition",
	"range-requests",
	"backpressure",
	"image-processing",
	"path-traversal",
	"terminal-companion",
}

// minFilesMediaProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only.
const minFilesMediaProseWords = 2500

func builtFilesMediaTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := filesMediaTopic()
	if err != nil {
		t.Fatalf("files-media builder must build cleanly: %v", err)
	}
	return topic
}

func registeredFilesMediaTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtFilesMediaTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "media-realtime", Title: "Media & realtime", Order: 7}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestFilesMediaIsMediaRealtimeOrderTwentySix(t *testing.T) {
	topic := registeredFilesMediaTopic(t)
	if topic.Group != "media-realtime" || topic.Order != 26 {
		t.Errorf("topic group/order = %q/%d, want media-realtime/26", topic.Group, topic.Order)
	}
	if topic.ID != filesMediaID {
		t.Errorf("topic id = %q, want %q", topic.ID, filesMediaID)
	}
}

func TestFilesMediaSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(filesMediaSectionIDs))
	for _, section := range builtFilesMediaTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(filesMediaSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, filesMediaSectionIDs)
	}
}

func TestFilesMediaBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtFilesMediaTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestFilesMediaDemoSitsInlineInItsTeachingSection pins the family
// anatomy: the ONE demo sits inside the section that teaches it, with
// prose and sourced code co-present — never queued at the topic's end.
func TestFilesMediaDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtFilesMediaTopic(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "multipart-streaming" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("section multipart-streaming missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "files:workbench" {
		t.Errorf("demo blocks = %+v, want exactly files:workbench", demos)
	}
	for _, kind := range []string{KindText, KindCode, KindDemo} {
		if !kinds[kind] {
			t.Errorf("section multipart-streaming lacks a %q block; kinds = %v", kind, kinds)
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

func TestFilesMediaClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtFilesMediaTopic(t).Sections
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

func TestFilesMediaProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtFilesMediaTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minFilesMediaProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minFilesMediaProseWords)
	}
	t.Logf("files-media prose word count: %d", words)
}

func TestFilesMediaMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtFilesMediaTopic(t).Sections {
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

func TestFilesMediaSnippetSourcesResolve(t *testing.T) {
	topic := builtFilesMediaTopic(t)
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
