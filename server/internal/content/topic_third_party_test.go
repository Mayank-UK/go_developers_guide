package content

import (
	"strings"
	"testing"
)

// Topic 42 third-party — pinned the same way every per-topic test pins
// its topic: exact section order, unique block ids, the demo inline in
// the section that teaches it (prose + code + demo co-present), the
// bash terminal companion last, the family prose floor, the
// must-source belt, the full snippet ratchet on a one-topic registry,
// and the capstone-source rule (the topic quotes the real instruments
// the earlier topics taught through, plus its own real hub machinery).
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 42 and
// group "capstones" on the built topic itself.

const thirdPartyID = "third-party"

var thirdPartySectionIDs = []string{
	"integrating-vs-building",
	"vendor-ports-and-adapters",
	"email-over-smtp",
	"email-as-a-job-recap",
	"stripe-payments",
	"cloud-storage",
	"keys-quotas-blast-radius",
	"terminal-companion",
}

// minThirdPartyProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only.
const minThirdPartyProseWords = 2500

func builtThirdPartyTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := thirdPartyTopic()
	if err != nil {
		t.Fatalf("third-party builder must build cleanly: %v", err)
	}
	return topic
}

func registeredThirdPartyTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtThirdPartyTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "capstones", Title: "Capstones", Order: 10}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestThirdPartyIsCapstonesOrderFortyTwo(t *testing.T) {
	topic := registeredThirdPartyTopic(t)
	if topic.Group != "capstones" || topic.Order != 42 {
		t.Errorf("topic group/order = %q/%d, want capstones/42", topic.Group, topic.Order)
	}
	if topic.ID != thirdPartyID {
		t.Errorf("topic id = %q, want %q", topic.ID, thirdPartyID)
	}
}

func TestThirdPartySectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(thirdPartySectionIDs))
	for _, section := range builtThirdPartyTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(thirdPartySectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, thirdPartySectionIDs)
	}
}

func TestThirdPartyBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtThirdPartyTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestThirdPartyDemoSitsInlineInItsTeachingSection pins the family
// anatomy: the curriculum row pins thirdparty:hub to the
// integrating-vs-building section, co-present with prose and sourced
// code.
func TestThirdPartyDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtThirdPartyTopic(t)
	wants := map[string]string{
		"integrating-vs-building": "thirdparty:hub",
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

func TestThirdPartyClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtThirdPartyTopic(t).Sections
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

// TestThirdPartyTerminalCompanionDrivesTheHubSurface pins the
// companion's promise: the tour reads the hub payload TWICE and diffs
// the bodies (the determinism claim, checked live), flips the shared
// scenario to live and back (the DI switch, checked live), reads the
// observed outcomes and counts, plants a sentinel credential and
// proves it never crosses the wire, and closes on the honest 404.
func TestThirdPartyTerminalCompanionDrivesTheHubSurface(t *testing.T) {
	sections := builtThirdPartyTopic(t).Sections
	last := sections[len(sections)-1]
	joined := ""
	for _, block := range last.Blocks {
		if code, ok := block.(CodeBlock); ok {
			joined += code.Snippet + "\n"
		}
	}
	for _, want := range []string{
		"/api/demo/thirdparty/hub",
		"/api/content/third-party",
		"diff <(curl -s localhost:8100/api/demo/thirdparty/hub)",
		"echo deterministic",
		"\"dataMode\":\"live\"",
		"\"dataMode\":\"fake\"",
		"/api/content/third-party-nope",
		"go_developers_guide_dev",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("terminal companion must exercise %q against the hub surface", want)
		}
	}
}

func TestThirdPartyProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtThirdPartyTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minThirdPartyProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minThirdPartyProseWords)
	}
	t.Logf("third-party prose word count: %d", words)
}

func TestThirdPartyMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtThirdPartyTopic(t).Sections {
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

func TestThirdPartySnippetSourcesResolve(t *testing.T) {
	topic := builtThirdPartyTopic(t)
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

// TestThirdPartyQuotesTheRealInstruments pins the capstone thesis:
// the topic's exhibits are the REAL instruments the earlier topics
// taught through — the manifest, the webhook signer, the taxonomy, the
// scenario switch, the optional-service timeouts, the canonical job
// handler, the deduplicator, the files port, the composition root,
// the secret rail, the rate policy — and its OWN hub machinery and
// handler.
func TestThirdPartyQuotesTheRealInstruments(t *testing.T) {
	sources := make(map[string]bool)
	for _, section := range builtThirdPartyTopic(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && code.Source != "" {
				sources[code.Source] = true
			}
		}
	}
	for _, want := range []string{
		"server/go.mod",
		"server/internal/domain/demos/webhook_simulator.go",
		"server/internal/core/errors.go",
		"server/internal/domain/demos/thirdparty_hub.go",
		"server/internal/presentation/demo_thirdparty.go",
		"server/internal/data/redis/playground.go",
		"server/internal/data/jobs/handlers.go",
		"server/internal/domain/messaging/streams.go",
		"server/internal/domain/files/files.go",
		"server/cmd/guide/main.go",
		"deploy/docker-compose.prod.yml",
		"server/internal/domain/security/ratelimit.go",
	} {
		if !sources[want] {
			t.Errorf("topic must quote its real instrument %s", want)
		}
	}
}

// TestThirdPartyCrossReferencesTheCurriculum pins the capstone
// contract: the topic consolidates the guide rather than introducing
// new machinery, and its authored prose names the topics it leans on.
func TestThirdPartyCrossReferencesTheCurriculum(t *testing.T) {
	joined := make([]string, 0)
	for _, section := range builtThirdPartyTopic(t).Sections {
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
		"topic 29",      // email as a job recap
		"topic 14",      // webhooks (the signature gate Stripe follows)
		"topic 26",      // files media (the Store port)
		"topic 39",      // reliability (bounded calls)
		"topic 31",      // dependency management (the manifest)
		"topic 32",      // architecture (composition root DI)
		"topic 30",      // error handling (the taxonomy)
		"topics 19, 20", // optional-service degradation
	} {
		if !strings.Contains(prose, want) {
			t.Errorf("the capstone must cross-reference %q in its prose", want)
		}
	}
}

// TestThirdPartyRecipesAreHonest pins the honesty stance: vendor SDKs
// this repo does not pin (stripe-go, aws-sdk, mail clients) appear
// only in bash recipes and prose — never as sourced code blocks
// pretending to be repo code — and every recipe says so in its own
// text.
func TestThirdPartyRecipesAreHonest(t *testing.T) {
	topic := builtThirdPartyTopic(t)
	for _, section := range topic.Sections {
		for _, block := range section.Blocks {
			code, ok := block.(CodeBlock)
			if !ok || code.Language != "bash" {
				continue
			}
			if strings.Contains(code.Snippet, "stripe-go/v") ||
				strings.Contains(code.Snippet, "aws-sdk-go-v2") {
				if !strings.Contains(code.Snippet, "RECIPE") || !strings.Contains(code.Snippet, "NOT") {
					t.Errorf("bash block %q names a vendor SDK without its RECIPE honesty header", code.ID)
				}
			}
		}
	}
}
