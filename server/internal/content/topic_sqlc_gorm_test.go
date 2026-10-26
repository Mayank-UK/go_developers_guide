package content

import (
	"strings"
	"testing"
)

// Topic 18 sqlc-gorm — the GO-NEW measurement topic, pinned the same way
// the exemplar pins topic 01: exact section order, unique block ids, the
// inline demo in the section that teaches the N+1 problem, the terminal
// companion last, the family prose floor, and the must-source belt (the
// snippet-sources ratchet in snippet_sources_test.go verifies every
// quoted line against the real file; this belt catches the authoring
// mistake of a must-source block with no source path at all).
//
// The pins run against the BUILDER through NewRegistry — production
// registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic), so this file asserts Order 18 and
// group "data" on the built topic itself rather than on the production
// set.

const sqlcGormID = "sqlc-gorm"

var sqlcGormSectionIDs = []string{
	"sqlc-sql-first",
	"gorm-what-an-orm-does",
	"the-n-plus-one-problem",
	"lazy-vs-eager-loading",
	"transactions-across-repositories",
	"testcontainers-go",
	"choosing-honestly",
	"terminal-companion",
}

// minSQLCGormProseWords is the family depth mandate: ~3k words per topic,
// floor 2500, counted over text blocks only — list items and code never
// count.
const minSQLCGormProseWords = 2500

// builtSQLcGorm builds the topic directly; the builder must assemble
// every block without a single constructor error.
func builtSQLcGorm(t *testing.T) Topic {
	t.Helper()
	topic, err := sqlcGormTopic()
	if err != nil {
		t.Fatalf("sqlc-gorm builder must build cleanly: %v", err)
	}
	return topic
}

// registeredSQLcGorm validates the builder's output through the
// registry's own rules (the real production gate) on a one-topic registry
// carrying the topic's curriculum group.
func registeredSQLcGorm(t *testing.T) Topic {
	t.Helper()
	topic := builtSQLcGorm(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "data", Title: "Data", Order: 5}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestSQLCGormIsDataOrderEighteen(t *testing.T) {
	topic := registeredSQLcGorm(t)
	if topic.Group != "data" || topic.Order != 18 {
		t.Errorf("topic group/order = %q/%d, want data/18", topic.Group, topic.Order)
	}
	if topic.ID != sqlcGormID {
		t.Errorf("topic id = %q, want %q", topic.ID, sqlcGormID)
	}
}

func TestSQLCGormSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(sqlcGormSectionIDs))
	for _, section := range builtSQLcGorm(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(sqlcGormSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, sqlcGormSectionIDs)
	}
}

func TestSQLCGormBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtSQLcGorm(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestSQLCGormDemoSitsInlineInTheNPlusOneSection(t *testing.T) {
	topic := builtSQLcGorm(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "the-n-plus-one-problem" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("the-n-plus-one-problem section missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "db:orm-probe" {
		t.Errorf("demo blocks = %+v, want exactly db:orm-probe", demos)
	}
	// Inline means beside the teaching: the section carries prose, code,
	// and the demo together.
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("the-n-plus-one-problem section lacks a %q block; kinds = %v", want, kinds)
		}
	}
}

func TestSQLCGormClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtSQLcGorm(t).Sections
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

func TestSQLCGormProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtSQLcGorm(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minSQLCGormProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minSQLCGormProseWords)
	}
	t.Logf("sqlc-gorm prose word count: %d", words)
}

func TestSQLCGormMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source language
	// with an empty source path — fails fast here too, at the topic layer,
	// instead of only in the file-reading ratchet.
	for _, section := range builtSQLcGorm(t).Sections {
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

func TestSQLCGormSnippetSourcesResolve(t *testing.T) {
	// The full snippet ratchet (RATCHET = 0), run against this topic's own
	// registry until production registration lands: every must-source block
	// must quote a real repo file, in order, after normalization.
	topic := builtSQLcGorm(t)
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "data", Title: "Data", Order: 5}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}
