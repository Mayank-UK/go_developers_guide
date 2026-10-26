package content

import (
	"strings"
	"testing"
)

// Topic 09 request-response — pinned the same way the exemplar pins its
// topics: exact section order, unique block ids, BOTH demos inline in
// the sections that teach them (prose + code + demo co-present), the
// bash terminal companion last, the family prose floor, and the
// must-source belt (the snippet-sources ratchet in
// snippet_sources_test.go verifies every quoted line against the real
// file; this belt catches the authoring mistake of a must-source block
// with no source path at all).
//
// The pins run against the BUILDER through NewRegistry — production
// registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic), so this file asserts Order 9 and
// group "request-handling" on the built topic itself rather than on the
// production set.

const requestResponseID = "request-response"

var requestResponseSectionIDs = []string{
	"json-decoder-streaming-bodies",
	"writing-responses",
	"headers-and-cookies",
	"flushing-and-sse",
	"content-negotiation",
	"redirects",
	"terminal-companion",
}

// minRequestResponseProseWords is the family depth mandate: ~3k words
// per topic, floor 2500, counted over text blocks only — list items and
// code never count.
const minRequestResponseProseWords = 2500

// builtRequestResponse builds the topic directly; the builder must
// assemble every block without a single constructor error.
func builtRequestResponse(t *testing.T) Topic {
	t.Helper()
	topic, err := requestResponseTopic()
	if err != nil {
		t.Fatalf("request-response builder must build cleanly: %v", err)
	}
	return topic
}

// registeredRequestResponse validates the builder's output through the
// registry's own rules (the real production gate) on a one-topic
// registry carrying the topic's curriculum group.
func registeredRequestResponse(t *testing.T) Topic {
	t.Helper()
	topic := builtRequestResponse(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "request-handling", Title: "Request handling", Order: 3}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestRequestResponseIsRequestHandlingOrderNine(t *testing.T) {
	topic := registeredRequestResponse(t)
	if topic.Group != "request-handling" || topic.Order != 9 {
		t.Errorf("topic group/order = %q/%d, want request-handling/9", topic.Group, topic.Order)
	}
	if topic.ID != requestResponseID {
		t.Errorf("topic id = %q, want %q", topic.ID, requestResponseID)
	}
}

func TestRequestResponseSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(requestResponseSectionIDs))
	for _, section := range builtRequestResponse(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(requestResponseSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, requestResponseSectionIDs)
	}
}

func TestRequestResponseBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtRequestResponse(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestRequestResponseDemosSitInlineInTheirTeachingSections pins BOTH
// demos: reqres:inspector in the decoder section, reqres:methods in the
// flushing/SSE section — each beside the prose and code that teach it.
func TestRequestResponseDemosSitInlineInTheirTeachingSections(t *testing.T) {
	topic := builtRequestResponse(t)
	wants := map[string]string{
		"json-decoder-streaming-bodies": "reqres:inspector",
		"flushing-and-sse":              "reqres:methods",
	}
	for sectionID, demoID := range wants {
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
		var demos []DemoBlock
		kinds := make(map[string]bool)
		for _, block := range section.Blocks {
			if demo, ok := block.(DemoBlock); ok {
				demos = append(demos, demo)
			}
			kinds[blockKindTag(block)] = true
		}
		if len(demos) != 1 || demos[0].ID != demoID {
			t.Errorf("section %s demo blocks = %+v, want exactly %s", sectionID, demos, demoID)
		}
		// Inline means beside the teaching: the section carries prose,
		// code, and the demo together.
		for _, want := range []string{KindText, KindCode, KindDemo} {
			if !kinds[want] {
				t.Errorf("section %s lacks a %q block; kinds = %v", sectionID, want, kinds)
			}
		}
	}
	// And nowhere else: exactly two demo blocks in the whole topic.
	total := 0
	for _, section := range topic.Sections {
		for _, block := range section.Blocks {
			if _, ok := block.(DemoBlock); ok {
				total++
			}
		}
	}
	if total != 2 {
		t.Errorf("demo blocks = %d, want exactly the topic's two", total)
	}
}

func TestRequestResponseClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtRequestResponse(t).Sections
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

// TestRequestResponseTerminalTourCoversBothDemosAndBoundsStreams pins
// the companion's shape: the tour walks both demo endpoints and teaches
// the --max-time stream discipline the SSE section asks for.
func TestRequestResponseTerminalTourCoversBothDemosAndBoundsStreams(t *testing.T) {
	last := builtRequestResponse(t).Sections[len(builtRequestResponse(t).Sections)-1]
	var tour strings.Builder
	for _, block := range last.Blocks {
		if code, ok := block.(CodeBlock); ok {
			tour.WriteString(code.Snippet)
			tour.WriteString("\n")
		}
	}
	whole := tour.String()
	for _, want := range []string{
		"/api/content/request-response",
		"/api/demo/reqres/inspector",
		"/api/demo/reqres/methods",
		"--max-time",
	} {
		if !strings.Contains(whole, want) {
			t.Errorf("terminal tour missing %q", want)
		}
	}
}

func TestRequestResponseProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtRequestResponse(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minRequestResponseProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minRequestResponseProseWords)
	}
	t.Logf("request-response prose word count: %d", words)
}

func TestRequestResponseMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source language
	// with an empty source path — fails fast here too, at the topic layer,
	// instead of only in the file-reading ratchet.
	for _, section := range builtRequestResponse(t).Sections {
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

func TestRequestResponseSnippetSourcesResolve(t *testing.T) {
	// The full snippet ratchet (RATCHET = 0), run against this topic's own
	// registry until production registration lands: every must-source block
	// must quote a real repo file, in order, after normalization.
	topic := builtRequestResponse(t)
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "request-handling", Title: "Request handling", Order: 3}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}
