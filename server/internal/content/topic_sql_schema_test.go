package content

import (
	"strings"
	"testing"
)

// Topic 16 sql-schema — pinned the same way every per-topic test pins its
// topic: exact section order, unique block ids, EACH demo inline in the
// section that teaches it (the explorer with indexes, the uuid lab with
// identifiers — both with prose + code + demo co-present), the bash
// terminal companion last, the family prose floor, and the must-source
// belt (the ratchet in snippet_sources_test.go verifies every quoted line
// against the real file; this belt catches the authoring mistake of a
// must-source block with no source path at all).
//
// The pins run against the BUILDER through NewRegistry — production
// registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic), so this file asserts Order 16 and group
// "data" on the built topic itself.

const sqlSchemaID = "sql-schema"

var sqlSchemaSectionIDs = []string{
	"relational-thinking",
	"types-and-constraints",
	"keys-and-relationships",
	"normalization-to-denormalization",
	"indexes",
	"migration-history",
	"identifiers",
	"audit-logging",
	"terminal-companion",
}

// minSQLSchemaProseWords is the family depth mandate: ~3k words per topic,
// floor 2500, counted over text blocks only — list items and code never
// count.
const minSQLSchemaProseWords = 2500

// builtSQLSchema builds the topic directly; the builder must assemble
// every block without a single constructor error.
func builtSQLSchema(t *testing.T) Topic {
	t.Helper()
	topic, err := sqlSchemaTopic()
	if err != nil {
		t.Fatalf("sql-schema builder must build cleanly: %v", err)
	}
	return topic
}

// registeredSQLSchema validates the builder's output through the
// registry's own rules (the real production gate) on a one-topic registry
// carrying the topic's curriculum group.
func registeredSQLSchema(t *testing.T) Topic {
	t.Helper()
	topic := builtSQLSchema(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "data", Title: "Data", Order: 5}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestSQLSchemaIsDataOrderSixteen(t *testing.T) {
	topic := registeredSQLSchema(t)
	if topic.Group != "data" || topic.Order != 16 {
		t.Errorf("topic group/order = %q/%d, want data/16", topic.Group, topic.Order)
	}
	if topic.ID != sqlSchemaID {
		t.Errorf("topic id = %q, want %q", topic.ID, sqlSchemaID)
	}
}

func TestSQLSchemaSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(sqlSchemaSectionIDs))
	for _, section := range builtSQLSchema(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(sqlSchemaSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, sqlSchemaSectionIDs)
	}
}

func TestSQLSchemaBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtSQLSchema(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

// inlineDemoInSection pins one demo block: exactly the wanted id, sitting
// in a section that also carries prose and code (inline means beside the
// teaching, never queued at the page end).
func inlineDemoInSection(t *testing.T, topic Topic, sectionID, demoID string) {
	t.Helper()
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == sectionID {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatalf("%s section missing", sectionID)
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
		t.Errorf("%s demo blocks = %+v, want exactly %s", sectionID, demos, demoID)
	}
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("%s section lacks a %q block; kinds = %v", sectionID, want, kinds)
		}
	}
}

func TestSQLSchemaDemosSitInlineInTheirTeachingSections(t *testing.T) {
	topic := builtSQLSchema(t)
	inlineDemoInSection(t, topic, "indexes", "db:schema-explorer")
	inlineDemoInSection(t, topic, "identifiers", "db:uuid")
}

func TestSQLSchemaClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtSQLSchema(t).Sections
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

func TestSQLSchemaProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtSQLSchema(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minSQLSchemaProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minSQLSchemaProseWords)
	}
	t.Logf("sql-schema prose word count: %d", words)
}

func TestSQLSchemaMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source language
	// with an empty source path — fails fast here too, at the topic layer,
	// instead of only in the file-reading ratchet.
	for _, section := range builtSQLSchema(t).Sections {
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

func TestSQLSchemaSnippetSourcesResolve(t *testing.T) {
	// The full snippet ratchet (RATCHET = 0), run against this topic's own
	// registry: every must-source block must quote a real repo file, in
	// order, after normalization — including the SQL migrations, the goose
	// runner, the config, the main.go boot wiring, and the UUIDv7
	// generator.
	topic := builtSQLSchema(t)
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

func TestSQLSchemaQuotesTheMigrationSQLAndItsGoCounterparts(t *testing.T) {
	// The topic's contract with the data phase: its must-source blocks
	// cover BOTH migrations, the embed, the runner, the composition-root
	// wiring, the config, and the uuid generator — the files topic 17
	// builds on.
	wantSources := []string{
		"server/internal/data/db/migrations/00001_init.sql",
		"server/internal/data/db/migrations/00002_audit_log.sql",
		"server/internal/data/db/embed.go",
		"server/internal/data/db/db.go",
		"server/cmd/guide/main.go",
		"server/internal/core/config.go",
		"server/internal/domain/demos/uuid_lab.go",
		"server/go.mod",
	}
	quoted := make(map[string]bool)
	for _, section := range builtSQLSchema(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && MustSourceLanguages[strings.ToLower(code.Language)] {
				quoted[code.Source] = true
			}
		}
	}
	for _, source := range wantSources {
		if !quoted[source] {
			t.Errorf("topic never quotes %s — the data phase's load-bearing file", source)
		}
	}
}
