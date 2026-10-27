package content

import (
	"strings"
	"testing"
)

// Topic 36 environments — pinned the same way every per-topic test pins
// its topic: exact section order, unique block ids, the demo inline in
// the section that teaches it (prose + code + demo co-present), the
// bash terminal companion last, the family prose floor, the
// must-source belt, and the full snippet ratchet on a one-topic
// registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 36 and group
// "platform" on the built topic itself.

const environmentsID = "environments"

var environmentsSectionIDs = []string{
	"one-codebase",
	"twelve-factor",
	"typed-env-config",
	"env-specific-behavior",
	"secrets",
	"feature-flags",
	"terminal-companion",
}

// minEnvironmentsProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only.
const minEnvironmentsProseWords = 2500

func builtEnvironmentsTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := environmentsTopic()
	if err != nil {
		t.Fatalf("environments builder must build cleanly: %v", err)
	}
	return topic
}

func registeredEnvironmentsTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtEnvironmentsTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "platform", Title: "Platform", Order: 8}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestEnvironmentsIsPlatformOrderThirtySix(t *testing.T) {
	topic := registeredEnvironmentsTopic(t)
	if topic.Group != "platform" || topic.Order != 36 {
		t.Errorf("topic group/order = %q/%d, want platform/36", topic.Group, topic.Order)
	}
	if topic.ID != environmentsID {
		t.Errorf("topic id = %q, want %q", topic.ID, environmentsID)
	}
}

func TestEnvironmentsSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(environmentsSectionIDs))
	for _, section := range builtEnvironmentsTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(environmentsSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, environmentsSectionIDs)
	}
}

func TestEnvironmentsBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtEnvironmentsTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestEnvironmentsDemoSitsInlineInItsTeachingSection pins the family
// anatomy: the curriculum row pins env:inspector to the typed env
// config section, co-present with prose and sourced code.
func TestEnvironmentsDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtEnvironmentsTopic(t)
	wants := map[string]string{
		"typed-env-config": "env:inspector",
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

func TestEnvironmentsClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtEnvironmentsTopic(t).Sections
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

// TestEnvironmentsTerminalCompanionDrivesTheConfigSurface pins the
// companion's promise: the tour reads the live inspector, proves the
// redaction on the wire, flips a scenario flag through PATCH and
// restores it, and drives a scratch binary through GDG_PORT /
// GDG_LOG_LEVEL overrides including the fail-fast refusals.
func TestEnvironmentsTerminalCompanionDrivesTheConfigSurface(t *testing.T) {
	sections := builtEnvironmentsTopic(t).Sections
	last := sections[len(sections)-1]
	joined := ""
	for _, block := range last.Blocks {
		if code, ok := block.(CodeBlock); ok {
			joined += code.Snippet + "\n"
		}
	}
	for _, want := range []string{
		"/api/demo/env/inspector",
		"-X PATCH",
		"/api/demo/scenario",
		"GDG_PORT=not-a-port",
		"GDG_PORT=8199",
		"GDG_LOG_LEVEL=debug",
		"/api/health",
		"guide@",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("terminal companion must exercise %q against the live stack", want)
		}
	}
}

func TestEnvironmentsProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtEnvironmentsTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minEnvironmentsProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minEnvironmentsProseWords)
	}
	t.Logf("environments prose word count: %d", words)
}

func TestEnvironmentsMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtEnvironmentsTopic(t).Sections {
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

func TestEnvironmentsSnippetSourcesResolve(t *testing.T) {
	topic := builtEnvironmentsTopic(t)
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

// TestEnvironmentsQuotesTheRealInstruments pins the topic's thesis: the
// subject of the environments topic is THIS repo's real machinery —
// the config loader, the scenario store, the demo's own redaction
// policy, the thin handler, the dev-instance declarations, and the
// composition root — so a drift away from that subject fails here.
func TestEnvironmentsQuotesTheRealInstruments(t *testing.T) {
	sources := make(map[string]bool)
	for _, section := range builtEnvironmentsTopic(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && code.Source != "" {
				sources[code.Source] = true
			}
		}
	}
	for _, want := range []string{
		"server/internal/core/config.go",
		"server/internal/core/scenario.go",
		"server/internal/domain/demos/env_inspector.go",
		"server/internal/presentation/demo_env.go",
		"server/internal/presentation/health.go",
		"server/cmd/guide/main.go",
		".devcontainer/devcontainer.json",
		".docker/docker-compose.yml",
		".docker/docker-compose.prod.yml",
	} {
		if !sources[want] {
			t.Errorf("topic must quote its real instrument %s", want)
		}
	}
}
