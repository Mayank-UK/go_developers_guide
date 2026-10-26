package content

import (
	"strings"
	"testing"
)

// Topic 03 http-web pattern pins, mirroring the go-language exemplar's
// contract tests. Production registration is the master's wave-close step,
// so these tests validate the BUILDER's result through NewRegistry (with
// this topic alone under the foundations group) — the same validations
// NewProductionRegistry will run the moment the topic is listed there:
// exact section order, unique block ids, the inline demo in the section
// that teaches it (prose + code + demo co-present), the bash terminal
// companion last, the family prose floor, and the must-source belt — which
// here runs the REAL snippet matcher (violations, the ratchet's own
// walker) so a mis-quoted block fails in this file before it can ever
// fail the ratchet.

const httpWebID = "http-web"

var httpWebSectionIDs = []string{
	"request-journey",
	"methods-status-headers",
	"idempotency-safety",
	"content-negotiation",
	"cookies-sessions-same-origin",
	"cors",
	"tls-https",
	"http-versions",
	"terminal-companion",
}

// minHTTPWebProseWords is the family depth mandate: ~3k words per topic;
// the floor catches thin topics. Counted over text blocks only — list
// items and code never count.
const minHTTPWebProseWords = 2500

// registeredHTTPWeb builds the topic and validates it through NewRegistry
// — every registry invariant (group membership, global order uniqueness,
// block rules, demo id pattern) runs before the tests assert on it.
func registeredHTTPWeb(t *testing.T) Topic {
	t.Helper()
	topic, err := httpWebTopic()
	if err != nil {
		t.Fatalf("httpWebTopic must build: %v", err)
	}
	registry, err := NewRegistry([]Topic{topic}, []TopicGroup{{ID: "foundations", Title: "Foundations", Order: 1}})
	if err != nil {
		t.Fatalf("NewRegistry over the built topic must validate: %v", err)
	}
	loaded, ok := registry.TopicByID(httpWebID)
	if !ok {
		t.Fatalf("topic %q must be resolvable in its validation registry", httpWebID)
	}
	return loaded
}

func TestHTTPWebPinsFoundationsOrderThree(t *testing.T) {
	topic := registeredHTTPWeb(t)
	if topic.Group != "foundations" || topic.Order != 3 {
		t.Errorf("topic group/order = %q/%d, want foundations/3", topic.Group, topic.Order)
	}
}

func TestHTTPWebSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(httpWebSectionIDs))
	for _, section := range registeredHTTPWeb(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(httpWebSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, httpWebSectionIDs)
	}
}

func TestHTTPWebBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range registeredHTTPWeb(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestHTTPWebDemoBlockSitsInlineInMethodsStatusHeadersSection(t *testing.T) {
	topic := registeredHTTPWeb(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "methods-status-headers" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("methods-status-headers section missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "http:headers" {
		t.Errorf("demo blocks = %+v, want exactly http:headers", demos)
	}
	// Inline means beside the teaching: the section carries prose, code,
	// and the demo together.
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("methods-status-headers section lacks a %q block; kinds = %v", want, kinds)
		}
	}
}

func TestHTTPWebClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := registeredHTTPWeb(t).Sections
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

func TestHTTPWebProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range registeredHTTPWeb(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minHTTPWebProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minHTTPWebProseWords)
	}
	t.Logf("http-web prose word count: %d", words)
}

func TestHTTPWebMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source
	// language with an empty source path — fails fast here too, at the
	// topic layer, instead of only in the file-reading ratchet.
	for _, section := range registeredHTTPWeb(t).Sections {
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

func TestHTTPWebSnippetsMatchTheirSources(t *testing.T) {
	// The belt with teeth: run the ratchet's own walker over this topic's
	// registry — every must-source block must resolve under the repo root
	// and match its real file (RATCHET = 0), so a mis-quoted snippet fails
	// HERE, at authoring time, not at the master's wave-close registration.
	topic, err := httpWebTopic()
	if err != nil {
		t.Fatalf("httpWebTopic: %v", err)
	}
	registry, err := NewRegistry([]Topic{topic}, []TopicGroup{{ID: "foundations", Title: "Foundations", Order: 1}})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("snippet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}
