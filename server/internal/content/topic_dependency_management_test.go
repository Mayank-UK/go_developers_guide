package content

import (
	"strings"
	"testing"
)

// Topic 31 dependency-management — pinned the same way every per-topic
// test pins its topic: exact section order, unique block ids, the ONE
// demo inline in the section that teaches govulncheck (prose + code +
// demo co-present), the bash terminal companion last, the family prose
// floor, the must-source belt, and the full snippet ratchet on a
// one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 31 and group
// "platform" on the built topic itself.

const dependencyManagementID = "dependency-management"

var dependencyManagementSectionIDs = []string{
	"what-a-dependency-is",
	"go-mod-and-mvs",
	"tidy-why-graph",
	"govulncheck",
	"upgrades",
	"caching-in-docker",
	"choosing-dependencies",
	"terminal-companion",
}

// minDependencyManagementProseWords is the family depth mandate: ~3k
// words per topic, floor 2500, counted over text blocks only.
const minDependencyManagementProseWords = 2500

func builtDependencyManagementTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := dependencyManagementTopic()
	if err != nil {
		t.Fatalf("dependency-management builder must build cleanly: %v", err)
	}
	return topic
}

func registeredDependencyManagementTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtDependencyManagementTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "platform", Title: "Platform", Order: 8}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestDependencyManagementIsPlatformOrderThirtyOne(t *testing.T) {
	topic := registeredDependencyManagementTopic(t)
	if topic.Group != "platform" || topic.Order != 31 {
		t.Errorf("topic group/order = %q/%d, want platform/31", topic.Group, topic.Order)
	}
	if topic.ID != dependencyManagementID {
		t.Errorf("topic id = %q, want %q", topic.ID, dependencyManagementID)
	}
}

func TestDependencyManagementSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(dependencyManagementSectionIDs))
	for _, section := range builtDependencyManagementTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(dependencyManagementSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, dependencyManagementSectionIDs)
	}
}

func TestDependencyManagementBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtDependencyManagementTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestDependencyManagementDemoSitsInlineInItsTeachingSection pins the
// family anatomy: the deps:manifest demo lives in the section that
// teaches govulncheck and the live manifest, co-present with prose and
// sourced code — never queued at the end.
func TestDependencyManagementDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtDependencyManagementTopic(t)

	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "govulncheck" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatalf("section govulncheck missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "deps:manifest" {
		t.Errorf("govulncheck demo blocks = %+v, want exactly deps:manifest", demos)
	}
	for _, kind := range []string{KindText, KindCode, KindDemo} {
		if !kinds[kind] {
			t.Errorf("section govulncheck lacks a %q block; kinds = %v", kind, kinds)
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

func TestDependencyManagementClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtDependencyManagementTopic(t).Sections
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

func TestDependencyManagementProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtDependencyManagementTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minDependencyManagementProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minDependencyManagementProseWords)
	}
	t.Logf("dependency-management prose word count: %d", words)
}

func TestDependencyManagementMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtDependencyManagementTopic(t).Sections {
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

func TestDependencyManagementSnippetSourcesResolve(t *testing.T) {
	topic := builtDependencyManagementTopic(t)
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
