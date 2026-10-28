package content

import (
	"strings"
	"testing"
)

// Topic 43 advanced-architecture — pinned the same way every per-topic
// test pins its topic: exact section order, unique block ids, BOTH demos
// inline in the sections that teach them (prose + code + demo
// co-present, each in its own section), the bash terminal companion
// last, the family prose floor, the must-source belt, the full snippet
// ratchet on a one-topic registry, and the capstone-source rule (the
// topic quotes the REAL distributed-patterns machinery it teaches
// through, plus the earlier topics' instruments it consolidates).

const advancedArchitectureID = "advanced-architecture"

var advancedArchitectureSectionIDs = []string{
	"consistency-map",
	"ddd-vocabulary",
	"monolith-first",
	"between-services",
	"sagas",
	"cqrs",
	"event-sourcing",
	"tracing-across-boundaries",
	"terminal-companion",
}

// minAdvancedArchitectureProseWords is the family depth mandate: ~3k
// words per topic, floor 2500, counted over text blocks only.
const minAdvancedArchitectureProseWords = 2500

func builtAdvancedArchitectureTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := advancedArchitectureTopic()
	if err != nil {
		t.Fatalf("advanced-architecture builder must build cleanly: %v", err)
	}
	return topic
}

func registeredAdvancedArchitectureTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtAdvancedArchitectureTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "capstones", Title: "Capstones", Order: 10}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestAdvancedArchitectureIsCapstonesOrderFortyThree(t *testing.T) {
	topic := registeredAdvancedArchitectureTopic(t)
	if topic.Group != "capstones" || topic.Order != 43 {
		t.Errorf("topic group/order = %q/%d, want capstones/43", topic.Group, topic.Order)
	}
	if topic.ID != advancedArchitectureID {
		t.Errorf("topic id = %q, want %q", topic.ID, advancedArchitectureID)
	}
}

func TestAdvancedArchitectureSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(advancedArchitectureSectionIDs))
	for _, section := range builtAdvancedArchitectureTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(advancedArchitectureSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, advancedArchitectureSectionIDs)
	}
}

func TestAdvancedArchitectureBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtAdvancedArchitectureTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestAdvancedArchitectureDemosSitInlineInTheirTeachingSections pins
// the family anatomy for the guide's only two-demo topic: the saga
// demo lives in the sagas section and the event-store demo in the
// event-sourcing section — each co-present with prose and sourced code,
// never queued at the page end.
func TestAdvancedArchitectureDemosSitInlineInTheirTeachingSections(t *testing.T) {
	topic := builtAdvancedArchitectureTopic(t)
	wants := map[string]string{
		"sagas":          "arch:saga",
		"event-sourcing": "arch:eventstore",
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
	if total != 2 {
		t.Errorf("topic carries %d demos, want exactly 2", total)
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

func TestAdvancedArchitectureClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtAdvancedArchitectureTopic(t).Sections
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

// TestAdvancedArchitectureTerminalCompanionDrivesBothDemos pins the
// companion's promise: the tour reads BOTH demo endpoints twice and
// diffs the bodies (the determinism claim, checked live), reads the
// observed outcomes and counts through the content API's own shape,
// rides the client's dev proxy, and closes on the honest 404.
func TestAdvancedArchitectureTerminalCompanionDrivesBothDemos(t *testing.T) {
	sections := builtAdvancedArchitectureTopic(t).Sections
	last := sections[len(sections)-1]
	joined := ""
	for _, block := range last.Blocks {
		if code, ok := block.(CodeBlock); ok {
			joined += code.Snippet + "\n"
		}
	}
	for _, want := range []string{
		"/api/demo/arch/saga",
		"/api/demo/arch/eventstore",
		"/api/content/advanced-architecture",
		"diff <(curl -s localhost:8100/api/demo/arch/saga)",
		"diff <(curl -s localhost:8100/api/demo/arch/eventstore)",
		"echo deterministic",
		"localhost:5175/api/demo/arch/saga",
		"/api/content/advanced-architecture-nope",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("terminal companion must exercise %q against the topic's surfaces", want)
		}
	}
}

func TestAdvancedArchitectureProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtAdvancedArchitectureTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minAdvancedArchitectureProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minAdvancedArchitectureProseWords)
	}
	t.Logf("advanced-architecture prose word count: %d", words)
}

func TestAdvancedArchitectureMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtAdvancedArchitectureTopic(t).Sections {
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

func TestAdvancedArchitectureSnippetSourcesResolve(t *testing.T) {
	topic := builtAdvancedArchitectureTopic(t)
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "capstones", Title: "Capstones", Order: 10}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}

// TestAdvancedArchitectureQuotesTheRealMachinery pins the capstone
// thesis: the topic's exhibits are the REAL distributed-patterns
// machinery the demos serve — saga and event store, their demo
// builders, their handlers, the composition root's routes, the client
// wiring — plus the earlier topics' instruments it consolidates (the
// isolation baseline, the outbox, the posts port and use cases, the
// tracer's graft point and child context).
func TestAdvancedArchitectureQuotesTheRealMachinery(t *testing.T) {
	sources := make(map[string]bool)
	for _, section := range builtAdvancedArchitectureTopic(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && code.Source != "" {
				sources[code.Source] = true
			}
		}
	}
	for _, want := range []string{
		"server/internal/domain/patterns/saga.go",
		"server/internal/domain/patterns/eventstore.go",
		"server/internal/domain/demos/arch_saga.go",
		"server/internal/domain/demos/arch_eventstore.go",
		"server/internal/presentation/demo_arch_saga.go",
		"server/internal/presentation/demo_arch_eventstore.go",
		"server/cmd/guide/main.go",
		"client/src/ui/demos/demo-registry.tsx",
		"client/src/core/container.ts",
		"server/internal/domain/messaging/outbox.go",
		"server/internal/domain/posts/repository.go",
		"server/internal/domain/posts/usecases.go",
		"server/internal/data/db/sql_demos.go",
		"server/internal/domain/obs/spans.go",
		"server/internal/domain/obs/tracecontext.go",
	} {
		if !sources[want] {
			t.Errorf("topic must quote its real instrument %s", want)
		}
	}
}

// TestAdvancedArchitectureCrossReferencesTheCurriculum pins the
// capstone contract: the topic consolidates the guide rather than
// introducing new machinery, and its authored prose names the topics it
// leans on.
func TestAdvancedArchitectureCrossReferencesTheCurriculum(t *testing.T) {
	joined := make([]string, 0)
	for _, section := range builtAdvancedArchitectureTopic(t).Sections {
		if section.Lead != "" {
			joined = append(joined, section.Lead)
		}
		for _, block := range section.Blocks {
			switch b := block.(type) {
			case TextBlock:
				joined = append(joined, b.Text)
			case ListBlock:
				joined = append(joined, strings.Join(b.Items, "\n"))
			}
		}
	}
	prose := strings.ToLower(strings.Join(joined, "\n"))
	for _, want := range []string{
		"topic 17", // databases-sql (the isolation baseline)
		"topic 22", // messaging (the outbox, idempotent consumers)
		"topic 30", // error handling (the taxonomy, typed errors)
		"topic 32", // architecture (the layer map this topic extends)
		"topic 33", // observability (trace propagation)
		"topic 34", // testing (the machinery's own pinned tests)
		"topic 39", // reliability (bounded calls across boundaries)
		"topic 40", // scaling (read/write scaling in CQRS)
	} {
		if !strings.Contains(prose, want) {
			t.Errorf("the capstone must cross-reference %q in its prose", want)
		}
	}
}
