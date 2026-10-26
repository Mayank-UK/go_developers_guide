package content

import (
	"strings"
	"testing"
)

// Topic 13 graphql — pinned the same way the exemplar pins topic 01:
// exact section order, unique block ids, the inline demo in the section
// that teaches introspection and tooling, the terminal companion last,
// the family prose floor, and the must-source belt (the snippet-sources
// ratchet in snippet_sources_test.go verifies every quoted line against
// the real file; this belt catches the authoring mistake of a must-source
// block with no source path at all).
//
// The pins run against the BUILDER through NewRegistry — production
// registration in NewProductionRegistry is a separate, deliberate step
// (registry.go lists every topic), so this file asserts Order 13 and
// group "apis" on the built topic itself rather than on the production
// set.

const graphqlTopicID = "graphql"

var graphqlSectionIDs = []string{
	"schema-first-thinking",
	"schema-and-resolvers",
	"queries-and-mutations",
	"graphql-errors",
	"n-plus-one-and-dataloaders",
	"introspection-and-tooling",
	"terminal-companion",
}

// minGraphQLProseWords is the family depth mandate: ~3k words per topic,
// floor 2500, counted over text blocks only — list items and code never
// count.
const minGraphQLProseWords = 2500

// builtGraphQL builds the topic directly; the builder must assemble
// every block without a single constructor error.
func builtGraphQL(t *testing.T) Topic {
	t.Helper()
	topic, err := graphqlTopic()
	if err != nil {
		t.Fatalf("graphql builder must build cleanly: %v", err)
	}
	return topic
}

// registeredGraphQL validates the builder's output through the
// registry's own rules (the real production gate) on a one-topic registry
// carrying the topic's curriculum group.
func registeredGraphQL(t *testing.T) Topic {
	t.Helper()
	topic := builtGraphQL(t)
	if _, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "apis", Title: "APIs", Order: 4}},
	); err != nil {
		t.Fatalf("registry must accept the builder's output: %v", err)
	}
	return topic
}

func TestGraphQLIsAPIsOrderThirteen(t *testing.T) {
	topic := registeredGraphQL(t)
	if topic.Group != "apis" || topic.Order != 13 {
		t.Errorf("topic group/order = %q/%d, want apis/13", topic.Group, topic.Order)
	}
	if topic.ID != graphqlTopicID {
		t.Errorf("topic id = %q, want %q", topic.ID, graphqlTopicID)
	}
}

func TestGraphQLSectionsAreInOrder(t *testing.T) {
	got := make([]string, 0, len(graphqlSectionIDs))
	for _, section := range builtGraphQL(t).Sections {
		got = append(got, section.ID)
	}
	if strings.Join(got, ",") != strings.Join(graphqlSectionIDs, ",") {
		t.Errorf("sections = %v, want %v", got, graphqlSectionIDs)
	}
}

func TestGraphQLBlockIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, section := range builtGraphQL(t).Sections {
		for _, block := range section.Blocks {
			id := blockIDOf(block)
			if seen[id] {
				t.Errorf("duplicate block id %q", id)
			}
			seen[id] = true
		}
	}
}

func TestGraphQLDemoSitsInlineInTheIntrospectionSection(t *testing.T) {
	topic := builtGraphQL(t)
	var section *Section
	for i := range topic.Sections {
		if topic.Sections[i].ID == "introspection-and-tooling" {
			section = &topic.Sections[i]
			break
		}
	}
	if section == nil {
		t.Fatal("introspection-and-tooling section missing")
	}
	var demos []DemoBlock
	kinds := make(map[string]bool)
	for _, block := range section.Blocks {
		if demo, ok := block.(DemoBlock); ok {
			demos = append(demos, demo)
		}
		kinds[blockKindTag(block)] = true
	}
	if len(demos) != 1 || demos[0].ID != "api:graphql-playground" {
		t.Errorf("demo blocks = %+v, want exactly api:graphql-playground", demos)
	}
	// Inline means beside the teaching: the section carries prose, code,
	// and the demo together.
	for _, want := range []string{KindText, KindCode, KindDemo} {
		if !kinds[want] {
			t.Errorf("introspection-and-tooling section lacks a %q block; kinds = %v", want, kinds)
		}
	}
}

func TestGraphQLClosesWithTheBashTerminalCompanion(t *testing.T) {
	sections := builtGraphQL(t).Sections
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
	if codeBlocks < 5 {
		t.Errorf("terminal companion needs its curl tour, got %d bash blocks", codeBlocks)
	}
}

func TestGraphQLProseMeetsTheFamilyFloor(t *testing.T) {
	words := 0
	for _, section := range builtGraphQL(t).Sections {
		for _, block := range section.Blocks {
			if text, ok := block.(TextBlock); ok {
				words += len(strings.Fields(text.Text))
			}
		}
	}
	if words < minGraphQLProseWords {
		t.Errorf("prose = %d words, want at least %d", words, minGraphQLProseWords)
	}
	t.Logf("graphql prose word count: %d", words)
}

func TestGraphQLMustSourceBlocksCarryASource(t *testing.T) {
	// Belt to the ratchet: the authoring invariant — a must-source language
	// with an empty source path — fails fast here too, at the topic layer,
	// instead of only in the file-reading ratchet.
	for _, section := range builtGraphQL(t).Sections {
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

func TestGraphQLSnippetSourcesResolve(t *testing.T) {
	// The full snippet ratchet (RATCHET = 0), run against this topic's own
	// registry until production registration lands: every must-source block
	// must quote a real repo file, in order, after normalization.
	topic := builtGraphQL(t)
	registry, err := NewRegistry(
		[]Topic{topic},
		[]TopicGroup{{ID: "apis", Title: "APIs", Order: 4}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if found := violations(t, registry); len(found) > RATCHET {
		t.Errorf("ratchet violations (max %d):\n%s", RATCHET, strings.Join(found, "\n"))
	}
}

func TestGraphQLQuotesTheGraphTreeItTeaches(t *testing.T) {
	// The topic's spine is the real graph tree: the schema, the codegen
	// contract, the generated interfaces, the resolver halves, the error
	// mapping, the execution path, the loader, and the endpoint must all
	// be quoted from the files that run in production.
	wantSources := []string{
		"server/internal/graph/schema/posts.graphqls",
		"server/internal/graph/gqlgen.yml",
		"server/internal/graph/generated/execution.go",
		"server/internal/graph/resolver/resolver.go",
		"server/internal/graph/resolver/posts.resolvers.go",
		"server/internal/graph/execute.go",
		"server/internal/presentation/demo_graphql.go",
		"server/internal/graph/errors.go",
		"server/internal/graph/loader/counting.go",
		"server/internal/graph/loader/loader.go",
	}
	quoted := make(map[string]bool)
	for _, section := range builtGraphQL(t).Sections {
		for _, block := range section.Blocks {
			if code, ok := block.(CodeBlock); ok && code.Source != "" {
				quoted[code.Source] = true
			}
		}
	}
	for _, source := range wantSources {
		if !quoted[source] {
			t.Errorf("topic must quote %s", source)
		}
	}
}
