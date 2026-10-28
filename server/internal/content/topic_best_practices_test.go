package content

import (
	"strings"
	"testing"
)

// Topic 41 best-practices — pinned the same way every per-topic test
// pins its topic: exact section order, unique block ids, the demo
// inline in the section that teaches it (prose + code + demo
// co-present), the bash terminal companion last, the family prose
// floor, the must-source belt, the full snippet ratchet on a one-topic
// registry, and the synthesis-source rule (the capstone quotes the
// real instruments the earlier topics taught through).
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 41 and
// group "capstones" on the built topic itself.

const bestPracticesID = "best-practices"

var bestPracticesSectionIDs = []string{
	"setup",
	"api-discipline",
	"security",
	"data",
	"reliability",
	"observability",
	"performance",
	"operations",
	"git-and-review",
	"human-practices",
	"anti-checklist",
	"terminal-companion",
}

// minBestPracticesProseWords is the family depth mandate: ~3k words
// per topic, floor 2500, counted over text blocks only.
const minBestPracticesProseWords = 2500

func builtBestPracticesTopic(t *testing.T) Topic {
	t.Helper()
	topic, err := bestPracticesTopic()
	if err != nil {
		t.Fatalf("best-practices builder must build cleanly: %v", err)
	}
	return topic
}

func registeredBestPracticesTopic(t *testing.T) Topic {
	t.Helper()
	topic := builtBestPracticesTopic(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "capstones", Title: "Capstones", Order: 10}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestBestPracticesIsCapstonesOrderFortyOne(t *testing.T) {
	topic := registeredBestPracticesTopic(t)
	if topic.Group != "capstones" || topic.Order != 41 {
		t.Errorf("topic group/order = %q/%d, want capstones/41", topic.Group, topic.Order)
	}
	if topic.ID != bestPracticesID {
		t.Errorf("topic id = %q, want %q", topic.ID, bestPracticesID)
	}
}

func TestBestPracticesSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(bestPracticesSectionIDs))
	for _, section := range builtBestPracticesTopic(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(bestPracticesSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, bestPracticesSectionIDs)
	}
}

func TestBestPracticesBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtBestPracticesTopic(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// TestBestPracticesDemoSitsInlineInItsTeachingSection pins the family
// anatomy: the curriculum row pins best:checklist to the anti-checklist
// section, co-present with prose and sourced code.
func TestBestPracticesDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtBestPracticesTopic(t)
	wants := map[string]string{
		"anti-checklist": "best:checklist",
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

func TestBestPracticesClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtBestPracticesTopic(t).Sections
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

// TestBestPracticesTerminalCompanionDrivesTheAuditSurface pins the
// companion's promise: the tour reads the audit panel TWICE and diffs
// the bodies (the boot-cache determinism claim, checked live), the
// summary line, a live row's evidence, the not-dos, this topic's own
// detail, and the honest 404 envelope.
func TestBestPracticesTerminalCompanionDrivesTheAuditSurface(t *testing.T) {
	sections := builtBestPracticesTopic(t).Sections
	last := sections[len(sections)-1]
	joined := ""
	for _, block := range last.Blocks {
		if code, ok := block.(CodeBlock); ok {
			joined += code.Snippet + "\n"
		}
	}
	for _, want := range []string{
		"/api/demo/best/checklist",
		"/api/content/best-practices",
		"diff <(curl -s localhost:8100/api/demo/best/checklist)",
		"echo deterministic",
		"/api/content/best-practices-nope",
		"go_developers_guide_dev",
		"error-envelope-on-miss",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("terminal companion must exercise %q against the audit surface", want)
		}
	}
}

func TestBestPracticesProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtBestPracticesTopic(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minBestPracticesProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minBestPracticesProseWords)
	}
	t.Logf("best-practices prose word count: %d", words)
}

func TestBestPracticesMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtBestPracticesTopic(t).Sections {
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

func TestBestPracticesSnippetSourcesResolve(t *testing.T) {
	topic := builtBestPracticesTopic(t)
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

// TestBestPracticesQuotesTheRealInstruments pins the synthesis thesis:
// the capstone's exhibits are the REAL instruments the earlier topics
// taught through — the module manifest, the thin handler, the status
// table, the decoy hash, the first migration, the drain, the logger,
// the bounded server, the compose file, the ratchet, and this topic's
// own audit machinery.
func TestBestPracticesQuotesTheRealInstruments(t *testing.T) {
	sources := make(map[string]bool)
	for _, section := range builtBestPracticesTopic(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && code.Source != "" {
				sources[code.Source] = true
			}
		}
	}
	for _, want := range []string{
		"server/go.mod",
		"server/internal/presentation/posts.go",
		"server/internal/core/errors.go",
		"server/internal/domain/auth/password.go",
		"server/internal/data/db/migrations/00001_init.sql",
		"server/cmd/guide/main.go",
		"server/internal/core/logging.go",
		".docker/docker-compose.yml",
		"server/internal/content/snippet_sources_test.go",
		"server/internal/domain/demos/best_checklist.go",
		"server/internal/presentation/demo_best.go",
	} {
		if !sources[want] {
			t.Errorf("topic must quote its real instrument %s", want)
		}
	}
}

// TestBestPracticesCrossReferencesTheCurriculum pins the synthesis
// contract: the capstone consolidates the guide rather than introducing
// new machinery, and its authored prose (leads, text, and list items)
// names the topics it consolidates.
func TestBestPracticesCrossReferencesTheCurriculum(t *testing.T) {
	joined := make([]string, 0)
	for _, section := range builtBestPracticesTopic(t).Sections {
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
		"topics 11 and 12",     // API discipline
		"topic 25",             // security
		"topics 16 through 20", // data
		"topic 39",             // reliability
		"topic 33",             // observability
		"topic 38",             // performance
		"topics 36 and 37",     // operations
	} {
		if !strings.Contains(prose, want) {
			t.Errorf("the synthesis must cross-reference %q in its prose", want)
		}
	}
}
