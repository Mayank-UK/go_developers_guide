package content

import (
	"strings"
	"testing"
)

// Topic 24 authorization — pinned the same way every per-topic test pins
// its topic: exact section order, unique block ids, the ONE demo inline in
// the section that teaches it (prose + code + demo co-present), the bash
// terminal companion last, the family prose floor, the must-source belt,
// and the full snippet ratchet on a one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 24 and group
// "state-auth" on the built topic itself.

const authorizationID = "authorization"

var authorizationSectionIDs = []string{
	"what-authorization-is",
	"rbac-roles-permissions",
	"middleware-enforcement",
	"scopes",
	"ownership",
	"deny-by-default",
	"in-the-wild",
	"terminal-companion",
}

// minAuthorizationProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only.
const minAuthorizationProseWords = 2500

func builtAuthorizationTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := authorizationTopic()
	if err != nil {
		t.Fatalf("authorization builder must build cleanly: %v", err)
	}
	return topic
}

func registeredAuthorizationTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtAuthorizationTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "state-auth", Title: "State & auth", Order: 6}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestAuthorizationIsStateAuthOrderTwentyFour(t *testing.T) {
	topic := registeredAuthorizationTopic(t)
	if topic.Group != "state-auth" || topic.Order != 24 {
		t.Errorf("topic group/order = %q/%d, want state-auth/24", topic.Group, topic.Order)
	}
	if topic.ID != authorizationID {
		t.Errorf("topic id = %q, want %q", topic.ID, authorizationID)
	}
}

func TestAuthorizationSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(authorizationSectionIDs))
	for _, section := range builtAuthorizationTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(authorizationSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, authorizationSectionIDs)
	}
}

func TestAuthorizationBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtAuthorizationTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestAuthorizationDemoSitsInlineInItsTeachingSection pins the family
// anatomy: the ONE demo sits inside the section that teaches it, with
// prose and sourced code co-present — never queued at the topic's end.
func TestAuthorizationDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtAuthorizationTopic(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "rbac-roles-permissions" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("section rbac-roles-permissions missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "authz:matrix" {
		t.Errorf("demo blocks = %+v, want exactly authz:matrix", demos)
	}
	for _, kind := range []string{KindText, KindCode, KindDemo} {
		if !kinds[kind] {
			t.Errorf("section lacks a %q block; kinds = %v", kind, kinds)
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

func TestAuthorizationClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtAuthorizationTopic(t).Sections
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

func TestAuthorizationProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtAuthorizationTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minAuthorizationProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minAuthorizationProseWords)
	}
	t.Logf("authorization prose word count: %d", words)
}

func TestAuthorizationMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtAuthorizationTopic(t).Sections {
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

func TestAuthorizationSnippetSourcesResolve(t *testing.T) {
	topic := builtAuthorizationTopic(t)
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
