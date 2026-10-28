package content

import (
	"strings"
	"testing"
)

// Fixture builders (the fastapi guide's tests/content_builders.py shape):
// each returns a VALID object or fails the test — invalid content is built
// explicitly where a rule is under test.

func buildGroup(t *testing.T, id string, order int) TopicGroup {
	t.Helper()
	title := strings.ToUpper(id[:1]) + id[1:]
	group, err := NewTopicGroup(id, title, order)
	if err != nil {
		t.Fatalf("buildGroup(%q): %v", id, err)
	}
	return group
}

func buildText(t *testing.T, id string) TextBlock {
	t.Helper()
	block, err := NewTextBlock(id, "One paragraph of connecting prose.")
	if err != nil {
		t.Fatalf("buildText(%q): %v", id, err)
	}
	return block
}

// buildCode returns a bash block — deliberately exempt from sourcing — so
// registry fixtures need no real files.
func buildCode(t *testing.T, id string) CodeBlock {
	t.Helper()
	block, err := NewCodeBlock(id, "bash", "", "", "curl localhost:8100/api/health")
	if err != nil {
		t.Fatalf("buildCode(%q): %v", id, err)
	}
	return block
}

func buildDemo(t *testing.T, id string) DemoBlock {
	t.Helper()
	block, err := NewDemoBlock(id, "Live demo", "")
	if err != nil {
		t.Fatalf("buildDemo(%q): %v", id, err)
	}
	return block
}

func buildList(t *testing.T, id string) ListBlock {
	t.Helper()
	block, err := NewListBlock(id, false, []string{
		"Lead one — explanation.",
		"Lead two — explanation.",
		"Lead three — explanation.",
	})
	if err != nil {
		t.Fatalf("buildList(%q): %v", id, err)
	}
	return block
}

func buildSection(t *testing.T, id string, blocks ...Block) Section {
	t.Helper()
	if len(blocks) == 0 {
		blocks = []Block{buildText(t, id+"-text"), buildCode(t, id+"-code")}
	}
	section, err := NewSection(id, "Section "+id, "", blocks)
	if err != nil {
		t.Fatalf("buildSection(%q): %v", id, err)
	}
	return section
}

func buildTopic(t *testing.T, id string, order int, sections ...Section) Topic {
	t.Helper()
	if len(sections) == 0 {
		sections = []Section{buildSection(t, id+"-s1")}
	}
	topic, err := NewTopic(id, "Topic "+id, "", "foundations", order, sections)
	if err != nil {
		t.Fatalf("buildTopic(%q): %v", id, err)
	}
	return topic
}

// buildRegistry assembles a registry with the standard foundations group
// plus any extras, or fails with the registry error.
func buildRegistry(t *testing.T, topics []Topic, groups []TopicGroup) Registry {
	t.Helper()
	registry, err := NewRegistry(topics, groups)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return registry
}

func TestSingleBlockSectionsAreLegalAuthoring(t *testing.T) {
	// The ≥2-blocks-incl-text rule is per TOPIC, not per section — a
	// narrative one-block section (or a terminal-companion section) next
	// to a prose-bearing section is exactly the family's authoring shape.
	_, err := NewRegistry(
		[]Topic{buildTopic(t, "t1", 1,
			buildSection(t, "narrative", buildText(t, "x1")),
			buildSection(t, "terminal", buildCode(t, "c1")),
		)},
		[]TopicGroup{buildGroup(t, "foundations", 1)},
	)
	if err != nil {
		t.Fatalf("single-block sections must be valid when the topic carries ≥2 blocks incl text: %v", err)
	}
}

func TestEmptyRegistryIsValid(t *testing.T) {
	registry, err := NewRegistry(nil, nil)
	if err != nil {
		t.Fatalf("empty registry must be valid (fresh state before Phase 3): %v", err)
	}
	if got := registry.Groups(); got == nil || len(got) != 0 {
		t.Errorf("empty registry groups = %v, want non-nil empty slice", got)
	}
	if got := registry.Summaries(); got == nil || len(got) != 0 {
		t.Errorf("empty registry summaries = %v, want non-nil empty slice", got)
	}
	if _, ok := registry.TopicByID("anything"); ok {
		t.Error("empty registry must not resolve any topic id")
	}
}

func TestRegistryServesSortedByOrder(t *testing.T) {
	registry := buildRegistry(t,
		[]Topic{
			buildTopic(t, "second", 2),
			buildTopic(t, "first", 1),
		},
		[]TopicGroup{buildGroup(t, "apis", 4), buildGroup(t, "foundations", 1)},
	)
	var topicIDs []string
	for _, summary := range registry.Summaries() {
		topicIDs = append(topicIDs, summary.ID)
	}
	if strings.Join(topicIDs, ",") != "first,second" {
		t.Errorf("topics served in %v, want order-sorted [first second]", topicIDs)
	}
	var groupIDs []string
	for _, group := range registry.Groups() {
		groupIDs = append(groupIDs, group.ID)
	}
	if strings.Join(groupIDs, ",") != "foundations,apis" {
		t.Errorf("groups served in %v, want order-sorted [foundations apis]", groupIDs)
	}
}

func TestSummariesCarrySectionCounts(t *testing.T) {
	registry := buildRegistry(t,
		[]Topic{buildTopic(t, "t1", 1, buildSection(t, "a"), buildSection(t, "b"), buildSection(t, "c"))},
		[]TopicGroup{buildGroup(t, "foundations", 1)},
	)
	summaries := registry.Summaries()
	if len(summaries) != 1 || summaries[0].SectionCount != 3 {
		t.Fatalf("summaries = %+v, want one topic with sectionCount 3", summaries)
	}
	if summaries[0].Group != "foundations" || summaries[0].Order != 1 {
		t.Errorf("summary group/order drifted: %+v", summaries[0])
	}
}

func TestTopicByID(t *testing.T) {
	registry := buildRegistry(t,
		[]Topic{buildTopic(t, "routing", 7)},
		[]TopicGroup{buildGroup(t, "foundations", 1)},
	)
	topic, ok := registry.TopicByID("routing")
	if !ok || topic.ID != "routing" || len(topic.Sections) != 1 {
		t.Fatalf("TopicByID(routing) = %+v, %v", topic, ok)
	}
	if _, ok := registry.TopicByID("nope"); ok {
		t.Error("TopicByID(nope) must not resolve")
	}
}

func TestRegistryValidationRules(t *testing.T) {
	oneGroup := []TopicGroup{buildGroup(t, "foundations", 1)}
	cases := []struct {
		name   string
		topics []Topic
		groups []TopicGroup
		wantIn string
	}{
		{
			name:   "duplicate topic id",
			topics: []Topic{buildTopic(t, "dup", 1), buildTopic(t, "dup", 2)},
			groups: oneGroup,
			wantIn: `duplicate topic id: "dup"`,
		},
		{
			name:   "unknown group",
			topics: []Topic{buildTopic(t, "t1", 1)},
			groups: []TopicGroup{buildGroup(t, "apis", 1)},
			wantIn: "references group",
		},
		{
			name: "duplicate topic order",
			topics: []Topic{
				buildTopic(t, "a", 5),
				buildTopic(t, "b", 5),
			},
			groups: oneGroup,
			wantIn: "duplicate topic order: 5",
		},
		{
			// Built as a raw literal: the topic constructor itself rejects
			// duplicate section ids, so this proves the registry re-checks
			// what a struct literal can smuggle past it.
			name: "duplicate section id within topic",
			topics: []Topic{{
				ID: "t1", Title: "Topic", Group: "foundations", Order: 1,
				Sections: []Section{buildSection(t, "dup"), buildSection(t, "dup")},
			}},
			groups: oneGroup,
			wantIn: `duplicate section id: "dup"`,
		},
		{
			// Family rule is per TOPIC: a topic whose whole body is one
			// text block has too little structure to teach.
			name: "topic with fewer than two blocks",
			topics: []Topic{buildTopic(t, "t1", 1,
				buildSection(t, "s1", buildText(t, "x1")))},
			groups: oneGroup,
			wantIn: "needs at least 2 blocks across its sections",
		},
		{
			// Also per TOPIC: structure without any prose is not teaching.
			name: "topic without a text block",
			topics: []Topic{buildTopic(t, "t1", 1,
				buildSection(t, "s1", buildCode(t, "c1"), buildList(t, "l1")))},
			groups: oneGroup,
			wantIn: "topic without text block",
		},
		{
			// A section with zero blocks can only arrive via a struct
			// literal (the constructor rejects it); the registry re-gates.
			name: "empty section via struct literal",
			topics: []Topic{buildTopic(t, "t1", 1,
				buildSection(t, "s1"),
				Section{ID: "s2", Title: "Empty", Blocks: nil})},
			groups: oneGroup,
			wantIn: `empty section: "s2"`,
		},
		{
			name: "duplicate block ids across sections of one topic",
			topics: []Topic{buildTopic(t, "t1", 1,
				buildSection(t, "s1", buildText(t, "x1"), buildCode(t, "c1")),
				buildSection(t, "s2", buildText(t, "x2"), buildCode(t, "c1")))},
			groups: oneGroup,
			wantIn: `duplicate block id: "c1"`,
		},
		{
			name: "empty text block via struct literal",
			topics: []Topic{buildTopic(t, "t1", 1, rawSection(t,
				TextBlock{Kind: KindText, ID: "x1", Text: "  "},
				buildCode(t, "c1")))},
			groups: oneGroup,
			wantIn: `empty text block: "x1"`,
		},
		{
			name: "mismatched kind tag via struct literal",
			topics: []Topic{buildTopic(t, "t1", 1, rawSection(t,
				TextBlock{Kind: KindList, ID: "x1", Text: "prose"},
				buildCode(t, "c1")))},
			groups: oneGroup,
			wantIn: `carries kind "list"`,
		},
		{
			name: "must-source code block without source",
			topics: []Topic{buildTopic(t, "t1", 1, buildSection(t, "s1",
				buildText(t, "x1"),
				CodeBlock{Kind: KindCode, ID: "c1", Language: "go", Snippet: "func f() {}"}))},
			groups: oneGroup,
			wantIn: "requires a non-empty source path",
		},
		{
			name: "demo id outside area:name grammar",
			topics: []Topic{buildTopic(t, "t1", 1, buildSection(t, "s1",
				buildText(t, "x1"), buildDemo(t, "just-a-name")))},
			groups: oneGroup,
			wantIn: "invalid demo id",
		},
		{
			name: "list block under three items via struct literal",
			topics: []Topic{buildTopic(t, "t1", 1, buildSection(t, "s1",
				buildText(t, "x1"),
				ListBlock{Kind: KindList, ID: "l1", Items: []string{"a — A.", "b — B."}}))},
			groups: oneGroup,
			wantIn: "list block size",
		},
		{
			name:   "duplicate group id",
			topics: nil,
			groups: []TopicGroup{buildGroup(t, "apis", 1), buildGroup(t, "apis", 2)},
			wantIn: `duplicate group id: "apis"`,
		},
		{
			name:   "duplicate group order",
			topics: nil,
			groups: []TopicGroup{buildGroup(t, "apis", 3), buildGroup(t, "data", 3)},
			wantIn: "duplicate group order: 3",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewRegistry(tc.topics, tc.groups)
			if err == nil {
				t.Fatalf("expected a registry error containing %q, got none", tc.wantIn)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error %q does not name the rule (%q)", err, tc.wantIn)
			}
		})
	}
}

// rawSection builds a section from raw blocks — for cases that
// deliberately bypass the constructors to prove the registry still gates
// struct literals.
func rawSection(t *testing.T, blocks ...Block) Section {
	t.Helper()
	section, err := NewSection("raw", "Raw", "", blocks)
	if err != nil {
		t.Fatalf("rawSection: %v", err)
	}
	return section
}

func TestProductionRegistryIsValid(t *testing.T) {
	registry, err := NewProductionRegistry()
	if err != nil {
		t.Fatalf("production registry must boot: %v", err)
	}
	wantGroups := []string{
		"foundations", "framework", "request-handling", "apis", "data",
		"state-auth", "media-realtime", "platform", "ship", "capstones",
	}
	var gotGroups []string
	for _, group := range registry.Groups() {
		gotGroups = append(gotGroups, group.ID)
	}
	if strings.Join(gotGroups, ",") != strings.Join(wantGroups, ",") {
		t.Errorf("production groups = %v, want %v (ordered)", gotGroups, wantGroups)
	}
	if count := len(registry.Summaries()); count != 38 {
		t.Errorf(
			"production registry has %d topics, want 38 (go-language, go-runtime, http-web, go-http-fundamentals, interfaces-and-composition, goroutines-and-channels, routing, middleware, request-response, validation, api-design, api-documentation, graphql, webhooks, grpc, sql-schema, databases-sql, sqlc-gorm, mongodb, redis, caching, messaging, authentication, authorization, security, files-media, internationalization, realtime, background-jobs, error-handling, dependency-management, architecture, observability, testing, debugging, environments, deployment, performance)",
			count,
		)
	}
}
