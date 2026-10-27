package content

import (
	"strings"
	"testing"
)

// Topic 25 security — pinned the same way every per-topic test pins its
// topic: exact section order, unique block ids, BOTH demos inline in
// the sections that teach them (prose + code + demo co-present), the
// bash terminal companion last, the family prose floor, the must-source
// belt, and the full snippet ratchet on a one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 25 and group
// "state-auth" on the built topic itself.

const securityID = "security"

var securitySectionIDs = []string{
	"threat-model",
	"security-headers",
	"cors",
	"rate-limiting",
	"injection-family",
	"csrf-ssrf",
	"secrets",
	"owasp-api-top-10",
	"pii-gdpr",
	"terminal-companion",
}

// minSecurityProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only.
const minSecurityProseWords = 2500

func builtSecurityTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := securityTopic()
	if err != nil {
		t.Fatalf("security builder must build cleanly: %v", err)
	}
	return topic
}

func registeredSecurityTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtSecurityTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "state-auth", Title: "State & auth", Order: 6}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestSecurityIsStateAuthOrderTwentyFive(t *testing.T) {
	topic := registeredSecurityTopic(t)
	if topic.Group != "state-auth" || topic.Order != 25 {
		t.Errorf("topic group/order = %q/%d, want state-auth/25", topic.Group, topic.Order)
	}
	if topic.ID != securityID {
		t.Errorf("topic id = %q, want %q", topic.ID, securityID)
	}
}

func TestSecuritySectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(securitySectionIDs))
	for _, section := range builtSecurityTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(securitySectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, securitySectionIDs)
	}
}

func TestSecurityBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtSecurityTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestSecurityDemosSitInlineInTheirTeachingSections pins the family
// anatomy: each of the TWO demos sits inside the section that teaches
// it, with prose and sourced code co-present — never queued at the
// topic's end.
func TestSecurityDemosSitInlineInTheirTeachingSections(t *testing.T) {
	topic := builtSecurityTopic(t)
	wants := []struct {
		sectionID string
		demoID    string
	}{
		{"security-headers", "security:lab"},
		{"pii-gdpr", "security:pii"},
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
	if total != 2 {
		t.Errorf("topic carries %d demos, want exactly 2", total)
	}
}

func TestSecurityClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtSecurityTopic(t).Sections
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

func TestSecurityProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtSecurityTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minSecurityProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minSecurityProseWords)
	}
	t.Logf("security prose word count: %d", words)
}

func TestSecurityMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtSecurityTopic(t).Sections {
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

func TestSecuritySnippetSourcesResolve(t *testing.T) {
	topic := builtSecurityTopic(t)
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
