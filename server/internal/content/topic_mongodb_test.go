package content

import (
	"strings"
	"testing"
)

// Topic 19 mongodb — pinned the same way every per-topic test pins its
// topic: exact section order, unique block ids, the demo inline in the
// section that teaches it (prose + code + demo co-present), the bash
// terminal companion last, the family prose floor, the must-source belt,
// and the full snippet ratchet on a one-topic registry.
//
// Registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic); this file asserts order 19 and group
// "data" on the built topic itself.

const mongodbID = "mongodb"

var mongodbSectionIDs = []string{
	"document-thinking",
	"optional-service-discipline",
	"the-go-driver",
	"validation-in-the-app",
	"queries-and-projection",
	"indexes-and-ttl",
	"layering-port-and-adapter",
	"sql-vs-mongo",
	"nosql-survey",
	"terminal-companion",
}

// minMongodbProseWords is the family depth mandate: ~3k words per topic,
// floor 2500, counted over text blocks only — list items and code never
// count.
const minMongodbProseWords = 2500

func builtMongodb(t *testing.T) Topic {
	t.Helper()
	topic, err := mongodbTopic()
	if err != nil {
		t.Fatalf("mongodb builder must build cleanly: %v", err)
	}
	return topic
}

func registeredMongodb(t *testing.T) Topic {
	t.Helper()
	topic := builtMongodb(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "data", Title: "Data", Order: 5}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestMongodbIsDataOrderNineteen(t *testing.T) {
	topic := registeredMongodb(t)
	if topic.Group != "data" || topic.Order != 19 {
		t.Errorf("topic group/order = %q/%d, want data/19", topic.Group, topic.Order)
	}
	if topic.ID != mongodbID {
		t.Errorf("topic id = %q, want %q", topic.ID, mongodbID)
	}
}

func TestMongodbSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(mongodbSectionIDs))
	for _, section := range builtMongodb(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(mongodbSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, mongodbSectionIDs)
	}
}

func TestMongodbBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtMongodb(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestMongodbDemoSitsInlineInItsTeachingSection(t *testing.T) {
	topic := builtMongodb(t)
	inlineDemoInSection(t, topic, "queries-and-projection", "db:mongo-console")
}

func TestMongodbClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtMongodb(t).Sections
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
		t.Error("terminal companion needs its curl and mongosh tour")
	}
}

func TestMongodbProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtMongodb(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minMongodbProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minMongodbProseWords)
	}
	t.Logf("mongodb prose word count: %d", words)
}

func TestMongodbMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: a must-source language with an empty source
	// path fails fast here too, at the topic layer.
	for _, section := range builtMongodb(t).Sections {
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

func TestMongodbSnippetSourcesResolve(t *testing.T) {
	// The full snippet ratchet (RATCHET = 0), run against this topic's
	// own registry: every must-source block must quote a real repo file,
	// in order, after normalization — the adapter, the domain's
	// validation and port, the console runner, the compose service, and
	// the go.mod pin.
	topic := builtMongodb(t)
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

func TestMongodbQuotesTheMongoTreeAndItsPins(t *testing.T) {
	// The topic's contract with the data phase: its must-source blocks
	// cover BOTH sides of the layering (domain port + validation, data
	// adapter + console runner), plus the driver pin and the compose
	// service the whole topic runs on.
	wantSources := []string{
		"server/internal/domain/events/event.go",
		"server/internal/domain/events/repository.go",
		"server/internal/domain/events/usecases.go",
		"server/internal/data/mongodb/client.go",
		"server/internal/data/mongodb/events.go",
		"server/internal/data/mongodb/console.go",
		"server/go.mod",
		".docker/docker-compose.yml",
	}
	quoted := make(map[string]bool)
	for _, section := range builtMongodb(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && MustSourceLanguages[strings.ToLower(code.Language)] {
				quoted[code.Source] = true
			}
		}
	}
	for _, source := range wantSources {
		if !quoted[source] {
			t.Errorf("topic never quotes %s — the mongodb topic's load-bearing file", source)
		}
	}
}
