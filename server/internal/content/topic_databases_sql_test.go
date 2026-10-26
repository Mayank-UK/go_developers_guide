package content

import (
	"strings"
	"testing"
)

// Topic 17 databases-sql — pinned the same way every per-topic test pins
// its topic: exact section order, unique block ids, EACH of the six demos
// inline in the section that teaches it (prose + code + demo co-present),
// the bash terminal companion last, the family prose floor (the scope
// earns the upper end of it), the must-source belt, and the full snippet
// ratchet on a one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 17 and group
// "data" on the built topic itself.

const databasesSQLID = "databases-sql"

var databasesSQLSectionIDs = []string{
	"connecting-and-pools",
	"dao-and-parameterized-sql",
	"one-port-two-stores",
	"transactions",
	"builders-and-orms",
	"sql-by-hand",
	"reading-plans",
	"full-text-search",
	"isolation-and-locking",
	"audit-in-transaction",
	"terminal-companion",
}

// minDatabasesSQLProseWords is the family depth mandate: ~3k words per
// topic, floor 2500, counted over text blocks only — this topic's six
// demos and eleven sections earn the upper end.
const minDatabasesSQLProseWords = 2500

func builtDatabasesSQL(t *testing.T) Topic {
	t.Helper()
	topic, err := databasesSQLTopic()
	if err != nil {
		t.Fatalf("databases-sql builder must build cleanly: %v", err)
	}
	return topic
}

func registeredDatabasesSQL(t *testing.T) Topic {
	t.Helper()
	topic := builtDatabasesSQL(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "data", Title: "Data", Order: 5}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestDatabasesSQLIsDataOrderSeventeen(t *testing.T) {
	topic := registeredDatabasesSQL(t)
	if topic.Group != "data" || topic.Order != 17 {
		t.Errorf("topic group/order = %q/%d, want data/17", topic.Group, topic.Order)
	}
	if topic.ID != databasesSQLID {
		t.Errorf("topic id = %q, want %q", topic.ID, databasesSQLID)
	}
}

func TestDatabasesSQLSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(databasesSQLSectionIDs))
	for _, section := range builtDatabasesSQL(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(databasesSQLSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, databasesSQLSectionIDs)
	}
}

func TestDatabasesSQLBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtDatabasesSQL(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestDatabasesSQLDemosSitInlineInTheirTeachingSections(t *testing.T) {
	topic := builtDatabasesSQL(t)
	inlineDemoInSection(t, topic, "dao-and-parameterized-sql", "db:sql-safety")
	inlineDemoInSection(t, topic, "transactions", "db:migrations")
	inlineDemoInSection(t, topic, "sql-by-hand", "db:sql-lab")
	inlineDemoInSection(t, topic, "full-text-search", "db:search")
	inlineDemoInSection(t, topic, "isolation-and-locking", "db:race")
	inlineDemoInSection(t, topic, "audit-in-transaction", "db:audit")
}

func TestDatabasesSQLClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtDatabasesSQL(t).Sections
	last := sections[len(sections)-1]
	if last.ID != "terminal-companion" {
		t.Fatalf("last section = %q, want terminal-companion", last.ID)
	}
	codeBlocks := 0
	sawFlip := false
	for _, block := range last.Blocks {
		if code, ok := block.(CodeBlock); ok {
			codeBlocks++
			if code.Language != "bash" || code.Source != "" {
				t.Errorf("terminal block %q must be bash with no source", code.ID)
			}
			if strings.Contains(code.Snippet, "dataMode") {
				sawFlip = true
			}
		}
	}
	if codeBlocks == 0 {
		t.Error("terminal companion needs its curl tour")
	}
	if !sawFlip {
		t.Error("the terminal companion must walk the dataMode flip — the topic's closing proof")
	}
}

func TestDatabasesSQLProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtDatabasesSQL(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minDatabasesSQLProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minDatabasesSQLProseWords)
	}
	t.Logf("databases-sql prose word count: %d", words)
}

func TestDatabasesSQLMustSourceBlocksCarryASource(t *testing.T) {
	for _, section := range builtDatabasesSQL(t).Sections {
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

func TestDatabasesSQLSnippetSourcesResolve(t *testing.T) {
	topic := builtDatabasesSQL(t)
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

func TestDatabasesSQLQuotesTheTopic17LoadBearers(t *testing.T) {
	// The topic's contract: its must-source blocks cover the pgx
	// repository, the switching facade, the composition-root swap, the
	// demo runner, the lab and search SQL, and the FTS migration — the
	// files this topic makes load-bearing.
	wantSources := []string{
		"server/internal/data/db/db.go",
		"server/internal/data/db/sql_demos.go",
		"server/internal/data/db/migrations/00003_fts_index.sql",
		"server/internal/data/posts/pg.go",
		"server/internal/data/posts/switching.go",
		"server/cmd/guide/main.go",
		"server/internal/domain/demos/sql_lab.go",
		"server/internal/domain/demos/sql_search.go",
	}
	quoted := make(map[string]bool)
	for _, section := range builtDatabasesSQL(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && MustSourceLanguages[strings.ToLower(code.Language)] {
				quoted[code.Source] = true
			}
		}
	}
	for _, source := range wantSources {
		if !quoted[source] {
			t.Errorf("topic never quotes %s — the topic 17 load-bearing file", source)
		}
	}
}
