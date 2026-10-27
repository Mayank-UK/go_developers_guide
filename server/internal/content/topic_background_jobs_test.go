package content

import (
	"strings"
	"testing"
)

// Topic 29 background-jobs — pinned the same way every per-topic test
// pins its topic: exact section order, unique block ids, the ONE demo
// inline in the section that teaches retries (prose + code + demo
// co-present), the bash terminal companion last, the family prose
// floor, the must-source belt, and the full snippet ratchet on a
// one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 29 and group
// "media-realtime" on the built topic itself.

const backgroundJobsID = "background-jobs"

var backgroundJobsSectionIDs = []string{
	"why-background",
	"goroutine-pools",
	"asynq-worker-queues",
	"retries-and-backoff",
	"dlq-and-replay",
	"scheduling-with-cron",
	"email-as-a-job",
	"observability-numbers",
	"terminal-companion",
}

// minBackgroundJobsProseWords is the family depth mandate: ~3k words
// per topic, floor 2500, counted over text blocks only.
const minBackgroundJobsProseWords = 2500

func builtBackgroundJobsTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := backgroundJobsTopic()
	if err != nil {
		t.Fatalf("background-jobs builder must build cleanly: %v", err)
	}
	return topic
}

func registeredBackgroundJobsTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtBackgroundJobsTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "media-realtime", Title: "Media & realtime", Order: 7}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestBackgroundJobsIsMediaRealtimeOrderTwentyNine(t *testing.T) {
	topic := registeredBackgroundJobsTopic(t)
	if topic.Group != "media-realtime" || topic.Order != 29 {
		t.Errorf("topic group/order = %q/%d, want media-realtime/29", topic.Group, topic.Order)
	}
	if topic.ID != backgroundJobsID {
		t.Errorf("topic id = %q, want %q", topic.ID, backgroundJobsID)
	}
}

func TestBackgroundJobsSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(backgroundJobsSectionIDs))
	for _, section := range builtBackgroundJobsTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(backgroundJobsSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, backgroundJobsSectionIDs)
	}
}

func TestBackgroundJobsBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtBackgroundJobsTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestBackgroundJobsDemoSitsInlineInItsTeachingSection pins the family
// anatomy: the jobs:console demo lives in the section that teaches
// retries and backoff, co-present with prose and sourced code — never
// queued at the end.
func TestBackgroundJobsDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtBackgroundJobsTopic(t)

	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "retries-and-backoff" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatalf("section retries-and-backoff missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "jobs:console" {
		t.Errorf("retries-and-backoff demo blocks = %+v, want exactly jobs:console", demos)
	}
	for _, kind := range []string{KindText, KindCode, KindDemo} {
		if !kinds[kind] {
			t.Errorf("section retries-and-backoff lacks a %q block; kinds = %v", kind, kinds)
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

func TestBackgroundJobsClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtBackgroundJobsTopic(t).Sections
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

func TestBackgroundJobsProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtBackgroundJobsTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minBackgroundJobsProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minBackgroundJobsProseWords)
	}
	t.Logf("background-jobs prose word count: %d", words)
}

func TestBackgroundJobsMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtBackgroundJobsTopic(t).Sections {
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

func TestBackgroundJobsSnippetSourcesResolve(t *testing.T) {
	topic := builtBackgroundJobsTopic(t)
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
