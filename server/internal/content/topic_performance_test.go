package content

import (
	"strings"
	"testing"
)

// Topic 38 performance — pinned the same way every per-topic test pins
// its topic: exact section order, unique block ids, the demo inline in
// the section that teaches it (prose + code + demo co-present), the
// bash terminal companion last, the family prose floor, the
// must-source belt, and the full snippet ratchet on a one-topic
// registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 38 and group
// "ship" on the built topic itself.

const performanceID = "performance"

var performanceSectionIDs = []string{
	"measure-first",
	"allocations-escape",
	"gc-tuning",
	"connection-pooling",
	"compression-bench",
	"pre-production-checklist",
	"terminal-companion",
}

// minPerformanceProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only.
const minPerformanceProseWords = 2500

func builtPerformanceTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := performanceTopic()
	if err != nil {
		t.Fatalf("performance builder must build cleanly: %v", err)
	}
	return topic
}

func registeredPerformanceTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtPerformanceTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "ship", Title: "Ship", Order: 9}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestPerformanceIsShipOrderThirtyEight(t *testing.T) {
	topic := registeredPerformanceTopic(t)
	if topic.Group != "ship" || topic.Order != 38 {
		t.Errorf("topic group/order = %q/%d, want ship/38", topic.Group, topic.Order)
	}
	if topic.ID != performanceID {
		t.Errorf("topic id = %q, want %q", topic.ID, performanceID)
	}
}

func TestPerformanceSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(performanceSectionIDs))
	for _, section := range builtPerformanceTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(performanceSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, performanceSectionIDs)
	}
}

func TestPerformanceBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtPerformanceTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestPerformanceDemoSitsInlineInItsTeachingSection pins the family
// anatomy: the curriculum row pins perf:bench to the compression
// benchmark section, co-present with prose and sourced code.
func TestPerformanceDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtPerformanceTopic(t)
	wants := map[string]string{
		"compression-bench": "perf:bench",
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

func TestPerformanceClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtPerformanceTopic(t).Sections
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

// TestPerformanceTerminalCompanionDrivesTheMeasurementSurface pins the
// companion's promise: the tour runs the real benchmark command with
// real budgets, walks the benchstat A/B ritual, reads the compiler's
// escape verdicts, and curls the demo's boot-cached run.
func TestPerformanceTerminalCompanionDrivesTheMeasurementSurface(t *testing.T) {
	sections := builtPerformanceTopic(t).Sections
	last := sections[len(sections)-1]
	joined := ""
	for _, block := range last.Blocks {
		if code, ok := block.(CodeBlock); ok {
			joined += code.Snippet + "\n"
		}
	}
	for _, want := range []string{
		"/api/demo/perf/bench",
		"go test -bench",
		"-count=10",
		"benchstat",
		"gcflags=-m",
		"./internal/domain/demos/",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("terminal companion must exercise %q against the measurement surface", want)
		}
	}
}

func TestPerformanceProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtPerformanceTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minPerformanceProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minPerformanceProseWords)
	}
	t.Logf("performance prose word count: %d", words)
}

func TestPerformanceMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtPerformanceTopic(t).Sections {
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

func TestPerformanceSnippetSourcesResolve(t *testing.T) {
	topic := builtPerformanceTopic(t)
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "ship", Title: "Ship", Order: 9}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}

// TestPerformanceQuotesTheRealInstruments pins the topic's thesis: the
// subject of the performance topic is THIS repo's real measurement
// machinery — the benchmark functions the reader runs, the payload
// builder and its gzip path, the boot-cached handler and its bounded
// process-group kill, the suite's own AllocsPerRun pin, the wire
// shaping and storage copying the escape verdicts name, and the pool
// and client the process actually boots with.
func TestPerformanceQuotesTheRealInstruments(t *testing.T) {
	sources := make(map[string]bool)
	for _, section := range builtPerformanceTopic(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && code.Source != "" {
				sources[code.Source] = true
			}
		}
	}
	for _, want := range []string{
		"server/internal/domain/demos/perf_benchmarks_test.go",
		"server/internal/domain/demos/perf_bench_test.go",
		"server/internal/domain/demos/perf_bench.go",
		"server/internal/presentation/demo_perf.go",
		"server/internal/presentation/posts.go",
		"server/internal/presentation/content.go",
		"server/internal/data/posts/inmemory.go",
		"server/internal/data/db/db.go",
		"server/internal/data/redis/playground.go",
	} {
		if !sources[want] {
			t.Errorf("topic must quote its real instrument %s", want)
		}
	}
}

// TestPerformanceInventsNoNumbers pins the no-invented-numbers rule on
// the teaching PROSE: text blocks must not carry benchmark-shaped
// figures (a digit before ns/op, B/op, or allocs/op), because every
// served number is parsed from a real run with its config. Naming the
// output COLUMNS is vocabulary; quoting a value is invention.
func TestPerformanceInventsNoNumbers(t *testing.T) {
	for _, section := range builtPerformanceTopic(t).Sections {
		for _, block := range section.Blocks {
			text, ok := block.(TextBlock)
			if !ok {
				continue
			}
			for _, unit := range []string{"ns/op", "B/op", "allocs/op"} {
				from := 0
				for {
					at := strings.Index(text.Text[from:], unit)
					if at < 0 {
						break
					}
					at += from
					if at > 0 && text.Text[at-1] >= '0' && text.Text[at-1] <= '9' {
						t.Errorf("text block %q carries a measured figure beside %q — numbers live in the demo, parsed",
							text.ID, unit)
					}
					from = at + len(unit)
				}
			}
		}
	}
}
