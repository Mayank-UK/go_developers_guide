package content

import (
	"strings"
	"testing"
)

// Topic 23 authentication — pinned the same way every per-topic test
// pins its topic: exact section order, unique block ids, all SEVEN
// demos inline in the sections that teach them (prose + code + demo
// co-present), the bash terminal companion last, the family prose
// floor, the must-source belt, and the full snippet ratchet on a
// one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 23 and group
// "state-auth" on the built topic itself.

const authenticationID = "authentication"

var authenticationSectionIDs = []string{
	"what-authentication-is",
	"login-flow-and-sessions",
	"api-keys",
	"choosing-mechanism",
	"jwt",
	"oauth2-flows",
	"deciding",
	"password-hashing",
	"refresh-rotation",
	"terminal-companion",
}

// minAuthenticationProseWords is the family depth mandate: ~3k words
// per topic, floor 2500, counted over text blocks only.
const minAuthenticationProseWords = 2500

func builtAuthenticationTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := authenticationTopic()
	if err != nil {
		t.Fatalf("authentication builder must build cleanly: %v", err)
	}
	return topic
}

func registeredAuthenticationTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtAuthenticationTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "state-auth", Title: "State & auth", Order: 6}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestAuthenticationIsStateAuthOrderTwentyThree(t *testing.T) {
	topic := registeredAuthenticationTopic(t)
	if topic.Group != "state-auth" || topic.Order != 23 {
		t.Errorf("topic group/order = %q/%d, want state-auth/23", topic.Group, topic.Order)
	}
	if topic.ID != authenticationID {
		t.Errorf("topic id = %q, want %q", topic.ID, authenticationID)
	}
}

func TestAuthenticationSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(authenticationSectionIDs))
	for _, section := range builtAuthenticationTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(authenticationSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, authenticationSectionIDs)
	}
}

func TestAuthenticationBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtAuthenticationTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestAuthenticationDemosSitInlineInTheirTeachingSections pins the
// family anatomy: each of the SEVEN demos sits inside the section that
// teaches it, with prose and sourced code co-present — never queued at
// the topic's end.
func TestAuthenticationDemosSitInlineInTheirTeachingSections(t *testing.T) {
	topic := builtAuthenticationTopic(t)
	wants := []struct {
		sectionID string
		demoID    string
	}{
		{"login-flow-and-sessions", "auth:sessions"},
		{"api-keys", "auth:api-keys"},
		{"choosing-mechanism", "auth:compare"},
		{"jwt", "auth:jwt"},
		{"oauth2-flows", "auth:oauth"},
		{"password-hashing", "auth:hashing"},
		{"refresh-rotation", "auth:refresh"},
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
		for _, kind := range []string{KindText, KindCode, KindDemo} {
			if !kinds[kind] {
				t.Errorf("section %s lacks a %q block; kinds = %v", want.sectionID, kind, kinds)
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
	if total != 7 {
		t.Errorf("topic carries %d demos, want exactly 7", total)
	}
}

func TestAuthenticationClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtAuthenticationTopic(t).Sections
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

func TestAuthenticationProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtAuthenticationTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minAuthenticationProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minAuthenticationProseWords)
	}
	t.Logf("authentication prose word count: %d", words)
}

func TestAuthenticationMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtAuthenticationTopic(t).Sections {
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

func TestAuthenticationSnippetSourcesResolve(t *testing.T) {
	topic := builtAuthenticationTopic(t)
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
