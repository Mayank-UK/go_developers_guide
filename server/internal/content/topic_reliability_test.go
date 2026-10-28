package content

import (
	"regexp"
	"strings"
	"testing"
)

// Topic 39 reliability — pinned the same way every per-topic test pins
// its topic: exact section order, unique block ids, the demo inline in
// the section that teaches it (prose + code + demo co-present), the
// bash terminal companion last, the family prose floor, the
// must-source belt, the full snippet ratchet on a one-topic registry,
// and the no-invented-timings honesty rule.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 39 and group
// "ship" on the built topic itself.

const reliabilityID = "reliability"

var reliabilitySectionIDs = []string{
	"distributed-failure",
	"timeouts-deadlines",
	"retries-backoff",
	"circuit-breaker",
	"degradation",
	"bulkheads-shedding",
	"chaos",
	"races",
	"terminal-companion",
}

// minReliabilityProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only.
const minReliabilityProseWords = 2500

func builtReliabilityTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := reliabilityTopic()
	if err != nil {
		t.Fatalf("reliability builder must build cleanly: %v", err)
	}
	return topic
}

func registeredReliabilityTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtReliabilityTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "ship", Title: "Ship", Order: 9}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestReliabilityIsShipOrderThirtyNine(t *testing.T) {
	topic := registeredReliabilityTopic(t)
	if topic.Group != "ship" || topic.Order != 39 {
		t.Errorf("topic group/order = %q/%d, want ship/39", topic.Group, topic.Order)
	}
	if topic.ID != reliabilityID {
		t.Errorf("topic id = %q, want %q", topic.ID, reliabilityID)
	}
}

func TestReliabilitySectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(reliabilitySectionIDs))
	for _, section := range builtReliabilityTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(reliabilitySectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, reliabilitySectionIDs)
	}
}

func TestReliabilityBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtReliabilityTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestReliabilityDemoSitsInlineInItsTeachingSection pins the family
// anatomy: the curriculum row pins reliability:breaker to the circuit
// breaker section, co-present with prose and sourced code.
func TestReliabilityDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtReliabilityTopic(t)
	wants := map[string]string{
		"circuit-breaker": "reliability:breaker",
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

func TestReliabilityClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtReliabilityTopic(t).Sections
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

// TestReliabilityTerminalCompanionDrivesTheReliabilitySurface pins the
// companion's promise: the tour curls the breaker under each scenario
// mode — baseline, threshold trip, deadline conversion, recovery —
// through the REAL scenario endpoints.
func TestReliabilityTerminalCompanionDrivesTheReliabilitySurface(t *testing.T) {
	sections := builtReliabilityTopic(t).Sections
	last := sections[len(sections)-1]
	joined := ""
	for _, block := range last.Blocks {
		if code, ok := block.(CodeBlock); ok {
			joined += code.Snippet + "\n"
		}
	}
	for _, want := range []string{
		"/api/demo/reliability/breaker",
		"PATCH localhost:8100/api/demo/scenario",
		"\"injectFailure\": \"error\"",
		"\"injectFailure\": \"latency\"",
		"\"injectFailure\": \"none\"",
		"/api/content/reliability",
		"sleep 3",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("terminal companion must exercise %q against the reliability surface", want)
		}
	}
}

func TestReliabilityProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtReliabilityTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minReliabilityProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minReliabilityProseWords)
	}
	t.Logf("reliability prose word count: %d", words)
}

func TestReliabilityMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtReliabilityTopic(t).Sections {
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

func TestReliabilitySnippetSourcesResolve(t *testing.T) {
	topic := builtReliabilityTopic(t)
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

// TestReliabilityQuotesTheRealInstruments pins the topic's thesis: the
// subject of the reliability topic is THIS repo's real machinery — the
// breaker and its dependency, the handler that walks them, the boot
// and server timeouts the process actually runs, the runtime topic's
// deadline beat, the jobs worker's retry policy, the redis adapter's
// optional-service timeouts, the worker-pool bulkhead, the limiter's
// typed shed, and the scenario store's chaos enum.
func TestReliabilityQuotesTheRealInstruments(t *testing.T) {
	sources := make(map[string]bool)
	for _, section := range builtReliabilityTopic(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && code.Source != "" {
				sources[code.Source] = true
			}
		}
	}
	for _, want := range []string{
		"server/internal/domain/demos/reliability_breaker.go",
		"server/internal/presentation/demo_reliability.go",
		"server/cmd/guide/main.go",
		"server/internal/domain/demos/context_cancellation.go",
		"server/internal/data/jobs/handlers.go",
		"server/internal/data/redis/playground.go",
		"server/internal/domain/demos/concurrency.go",
		"server/internal/presentation/security_ratelimit.go",
		"server/internal/core/scenario.go",
	} {
		if !sources[want] {
			t.Errorf("topic must quote its real instrument %s", want)
		}
	}
}

// TestReliabilityInventsNoTimings is the demo's honesty pin on the
// teaching PROSE (the performance topic's no-invented-numbers rule,
// applied to time): text and list blocks must not carry measured-look
// timing figures (a digit before ms/milliseconds/µs/us/ns/sec/second).
// The topic's real durations — config constants and measured elapsed
// values — live in the quoted code and the demo payload respectively;
// naming a unit in words is vocabulary, quoting a value is invention.
func TestReliabilityInventsNoTimings(t *testing.T) {
	timingFigure := regexp.MustCompile(`[0-9]+(\.[0-9]+)?\s*(ms|millis|µs|us|ns|sec|second)`)
	for _, section := range builtReliabilityTopic(t).Sections {
		for _, block := range section.Blocks {
			var body string
			switch b := block.(type) {
			case TextBlock:
				body = b.Text
			case ListBlock:
				body = strings.Join(b.Items, "\n")
			default:
				continue
			}
			if match := timingFigure.FindString(body); match != "" {
				t.Errorf("block %q carries a timing figure %q — durations live in the demo, measured",
					blockIDOf(block), match)
			}
		}
	}
}
