package content

import (
	"regexp"
	"strings"
	"testing"
)

// Topic 40 scaling — pinned the same way every per-topic test pins its
// topic: exact section order, unique block ids, the demo inline in the
// section that teaches it (prose + code + demo co-present), the bash
// terminal companion last, the family prose floor, the must-source
// belt, the full snippet ratchet on a one-topic registry, and the
// no-invented-throughput honesty rule.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 40 and group
// "ship" on the built topic itself.

const scalingID = "scaling"

var scalingSectionIDs = []string{
	"what-scaling-means",
	"statelessness",
	"load-balancing",
	"replicas",
	"data-at-scale",
	"cdn-edge",
	"backpressure",
	"capacity",
	"terminal-companion",
}

// minScalingProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only.
const minScalingProseWords = 2500

func builtScalingTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := scalingTopic()
	if err != nil {
		t.Fatalf("scaling builder must build cleanly: %v", err)
	}
	return topic
}

func registeredScalingTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtScalingTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "ship", Title: "Ship", Order: 9}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestScalingIsShipOrderForty(t *testing.T) {
	topic := registeredScalingTopic(t)
	if topic.Group != "ship" || topic.Order != 40 {
		t.Errorf("topic group/order = %q/%d, want ship/40", topic.Group, topic.Order)
	}
	if topic.ID != scalingID {
		t.Errorf("topic id = %q, want %q", topic.ID, scalingID)
	}
}

func TestScalingSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(scalingSectionIDs))
	for _, section := range builtScalingTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(scalingSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, scalingSectionIDs)
	}
}

func TestScalingBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtScalingTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestScalingDemoSitsInlineInItsTeachingSection pins the family
// anatomy: the curriculum row pins scaling:stateless to the
// statelessness section, co-present with prose and sourced code.
func TestScalingDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtScalingTopic(t)
	wants := map[string]string{
		"statelessness": "scaling:stateless",
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

func TestScalingClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtScalingTopic(t).Sections
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

// TestScalingTerminalCompanionDrivesTheScalingSurface pins the
// companion's promise: the tour curls the statelessness round TWICE and
// diffs the bodies (the demo's determinism claim, checked live), the
// topic detail, and the honest 404 envelope.
func TestScalingTerminalCompanionDrivesTheScalingSurface(t *testing.T) {
	sections := builtScalingTopic(t).Sections
	last := sections[len(sections)-1]
	joined := ""
	for _, block := range last.Blocks {
		if code, ok := block.(CodeBlock); ok {
			joined += code.Snippet + "\n"
		}
	}
	for _, want := range []string{
		"/api/demo/scaling/stateless",
		"/api/content/scaling",
		"diff <(curl -s localhost:8100/api/demo/scaling/stateless)",
		"echo deterministic",
		"/api/content/scaling-nope",
		"/api/ready",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("terminal companion must exercise %q against the scaling surface", want)
		}
	}
}

func TestScalingProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtScalingTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minScalingProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minScalingProseWords)
	}
	t.Logf("scaling prose word count: %d", words)
}

func TestScalingMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtScalingTopic(t).Sections {
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

func TestScalingSnippetSourcesResolve(t *testing.T) {
	topic := builtScalingTopic(t)
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

// TestScalingQuotesTheRealInstruments pins the topic's thesis: the
// subject of the scaling topic is THIS repo's real machinery — the
// demo's fleet and balancer, the thin handler that serves it, the
// composition root's per-process state and graceful drain, the k8s
// Service and Deployment as balancer and replica contract, the
// schema's index doctrine, the process-wide pool, the nginx edge, the
// token-bucket limiter, the realtime ceilings, and the metrics pair
// capacity arithmetic feeds on.
func TestScalingQuotesTheRealInstruments(t *testing.T) {
	sources := make(map[string]bool)
	for _, section := range builtScalingTopic(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && code.Source != "" {
				sources[code.Source] = true
			}
		}
	}
	for _, want := range []string{
		"server/internal/domain/demos/scaling_stateless.go",
		"server/internal/presentation/demo_scaling.go",
		"server/cmd/guide/main.go",
		"deploy/k8s/service.json",
		"deploy/k8s/deployment.json",
		"server/internal/data/db/migrations/00001_init.sql",
		"server/internal/data/db/db.go",
		".docker/client/nginx.conf",
		"server/internal/domain/security/ratelimit.go",
		"server/internal/domain/realtime/hub.go",
		"server/internal/presentation/metrics.go",
	} {
		if !sources[want] {
			t.Errorf("topic must quote its real instrument %s", want)
		}
	}
}

// TestScalingInventsNoThroughput is the topic's honesty pin on the
// teaching PROSE (the performance topic's no-invented-numbers rule,
// applied to throughput): text and list blocks must not carry
// throughput-shaped figures (a digit beside rps/qps/req-per-second, or
// an N-times-faster claim). The round's real counts live in the demo
// payload, measured; naming a unit in words is vocabulary, quoting a
// value is invention.
func TestScalingInventsNoThroughput(t *testing.T) {
	throughputFigure := regexp.MustCompile(
		`[0-9]+(\.[0-9]+)?\s*(rps|qps|req/s|requests? per second)|[0-9]+(\.[0-9]+)?x faster|[0-9]+(\.[0-9]+)?×`)
	for _, section := range builtScalingTopic(t).Sections {
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
			if match := throughputFigure.FindString(body); match != "" {
				t.Errorf("block %q carries a throughput figure %q — counts live in the demo, measured",
					blockIDOf(block), match)
			}
		}
	}
}
