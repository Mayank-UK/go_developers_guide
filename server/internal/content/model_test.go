package content

import (
	"encoding/json"
	"strings"
	"testing"
)

// Wire goldens pin the exact JSON the client's DTO layer parses (its
// fromJson is the binding contract): camelCase keys, kind discriminator
// first, optional fields ABSENT when unset — never null, never "".

func marshal(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

func TestTextBlockWireGolden(t *testing.T) {
	block, err := NewTextBlock("t1", "One paragraph of prose.")
	if err != nil {
		t.Fatalf("NewTextBlock: %v", err)
	}
	got := marshal(t, block)
	want := `{"kind":"text","id":"t1","text":"One paragraph of prose."}`
	if got != want {
		t.Errorf("text block wire =\n got %s\nwant %s", got, want)
	}
}

func TestCodeBlockWireGolden(t *testing.T) {
	full, err := NewCodeBlock("c1", "go", "The matcher", "server/internal/content/snippet.go", "func MatchSnippet() {}")
	if err != nil {
		t.Fatalf("NewCodeBlock: %v", err)
	}
	got := marshal(t, full)
	want := `{"kind":"code","id":"c1","language":"go","caption":"The matcher",` +
		`"source":"server/internal/content/snippet.go","snippet":"func MatchSnippet() {}"}`
	if got != want {
		t.Errorf("code block wire =\n got %s\nwant %s", got, want)
	}

	// An exempt-language block (terminal tour) carries no caption/source —
	// both keys are absent, not null.
	bare, err := NewCodeBlock("c2", "bash", "", "", "curl localhost:8100/api/content")
	if err != nil {
		t.Fatalf("NewCodeBlock: %v", err)
	}
	got = marshal(t, bare)
	want = `{"kind":"code","id":"c2","language":"bash","snippet":"curl localhost:8100/api/content"}`
	if got != want {
		t.Errorf("bare code block wire =\n got %s\nwant %s", got, want)
	}
}

func TestDemoBlockWireGolden(t *testing.T) {
	full, err := NewDemoBlock("runtime:async", "Async explorer", "Watch scheduling live.")
	if err != nil {
		t.Fatalf("NewDemoBlock: %v", err)
	}
	got := marshal(t, full)
	want := `{"kind":"demo","id":"runtime:async","title":"Async explorer","description":"Watch scheduling live."}`
	if got != want {
		t.Errorf("demo block wire =\n got %s\nwant %s", got, want)
	}

	bare, err := NewDemoBlock("runtime:async", "Async explorer", "")
	if err != nil {
		t.Fatalf("NewDemoBlock: %v", err)
	}
	got = marshal(t, bare)
	want = `{"kind":"demo","id":"runtime:async","title":"Async explorer"}`
	if got != want {
		t.Errorf("bare demo block wire =\n got %s\nwant %s", got, want)
	}
}

func TestListBlockWireGolden(t *testing.T) {
	block, err := NewListBlock("l1", false, []string{
		"Buffered channels — block when full.",
		"Select — wait on many operations.",
		"Context — carry cancellation.",
	})
	if err != nil {
		t.Fatalf("NewListBlock: %v", err)
	}
	got := marshal(t, block)
	want := `{"kind":"list","id":"l1","ordered":false,"items":[` +
		`"Buffered channels — block when full.",` +
		`"Select — wait on many operations.",` +
		`"Context — carry cancellation."]}`
	if got != want {
		t.Errorf("list block wire =\n got %s\nwant %s", got, want)
	}
}

func TestSectionAndTopicWireGolden(t *testing.T) {
	text, _ := NewTextBlock("t1", "Prose first.")
	code, _ := NewCodeBlock("c1", "bash", "", "", "curl localhost:8100/api/health")
	section, err := NewSection("s1", "Requests", "", []Block{text, code})
	if err != nil {
		t.Fatalf("NewSection: %v", err)
	}
	got := marshal(t, section)
	want := `{"id":"s1","title":"Requests","blocks":[` +
		`{"kind":"text","id":"t1","text":"Prose first."},` +
		`{"kind":"code","id":"c1","language":"bash","snippet":"curl localhost:8100/api/health"}]}`
	if got != want {
		t.Errorf("section wire =\n got %s\nwant %s", got, want)
	}

	withLead, err := NewSection("s2", "Leads", "One framing sentence.", []Block{text})
	if err != nil {
		t.Fatalf("NewSection: %v", err)
	}
	if got := marshal(t, withLead); !strings.Contains(got, `"lead":"One framing sentence."`) {
		t.Errorf("section with lead omits it: %s", got)
	}

	topic, err := NewTopic("routing", "Routing", "", "request-handling", 7, []Section{section})
	if err != nil {
		t.Fatalf("NewTopic: %v", err)
	}
	got = marshal(t, topic)
	if !strings.HasPrefix(got, `{"id":"routing","title":"Routing","group":"request-handling","order":7,"sections":[`) {
		t.Errorf("topic wire shape drifted: %s", got)
	}
	if strings.Contains(got, `"subtitle"`) {
		t.Errorf("unset subtitle must be absent from the wire: %s", got)
	}

	withSubtitle, err := NewTopic("routing", "Routing", "From pattern to handler", "request-handling", 7, []Section{section})
	if err != nil {
		t.Fatalf("NewTopic: %v", err)
	}
	if got := marshal(t, withSubtitle); !strings.Contains(got, `"subtitle":"From pattern to handler"`) {
		t.Errorf("set subtitle missing from the wire: %s", got)
	}
}

func TestSummaryAndResponseWireGolden(t *testing.T) {
	summary := TopicSummary{
		ID: "routing", Title: "Routing", Group: "request-handling", Order: 7, SectionCount: 3,
	}
	if got, want := marshal(t, summary),
		`{"id":"routing","title":"Routing","group":"request-handling","order":7,"sectionCount":3}`; got != want {
		t.Errorf("summary wire =\n got %s\nwant %s", got, want)
	}

	group, err := NewTopicGroup("apis", "APIs", 4)
	if err != nil {
		t.Fatalf("NewTopicGroup: %v", err)
	}
	detail := ContentDetail{Version: ContentAPIVersion, Topic: Topic{
		ID: "routing", Title: "Routing", Group: "request-handling", Order: 7,
	}}
	got := marshal(t, detail)
	if want := `{"version":1,"id":"routing","title":"Routing","group":"request-handling","order":7,"sections":null}`; got != want {
		t.Errorf("detail wire =\n got %s\nwant %s", got, want)
	}

	list := ContentListResponse{
		Version: ContentAPIVersion,
		Groups:  []TopicGroup{group},
		Topics:  []TopicSummary{summary},
	}
	got = marshal(t, list)
	want := `{"version":1,"groups":[{"id":"apis","title":"APIs","order":4}],` +
		`"topics":[{"id":"routing","title":"Routing","group":"request-handling","order":7,"sectionCount":3}]}`
	if got != want {
		t.Errorf("list wire =\n got %s\nwant %s", got, want)
	}
}

func TestConstructorInvariants(t *testing.T) {
	cases := []struct {
		name  string
		build func() error
	}{
		{"text blank id", func() error { _, err := NewTextBlock("  ", "text"); return err }},
		{"text blank text", func() error { _, err := NewTextBlock("t1", " \n "); return err }},
		{"code blank language", func() error {
			_, err := NewCodeBlock("c1", "", "", "server/x.go", "x")
			return err
		}},
		{"code blank snippet", func() error {
			_, err := NewCodeBlock("c1", "go", "", "server/x.go", " ")
			return err
		}},
		{"demo blank title", func() error { _, err := NewDemoBlock("a:b", "", ""); return err }},
		{"list too few", func() error {
			_, err := NewListBlock("l1", false, []string{"a — A.", "b — B."})
			return err
		}},
		{"list too many", func() error {
			_, err := NewListBlock("l1", false, []string{"1 — I.", "2 — II.", "3 — III.", "4 — IV.", "5 — V.", "6 — VI.", "7 — VII.", "8 — VIII.", "9 — IX."})
			return err
		}},
		{"list blank item", func() error {
			_, err := NewListBlock("l1", false, []string{"a — A.", "  ", "c — C."})
			return err
		}},
		{"list duplicate item", func() error {
			_, err := NewListBlock("l1", false, []string{"a — A.", "a — A.", "c — C."})
			return err
		}},
		{"section no blocks", func() error { _, err := NewSection("s1", "T", "", nil); return err }},
		{"section nil block", func() error {
			_, err := NewSection("s1", "T", "", []Block{nil})
			return err
		}},
		{"section duplicate block ids", func() error {
			text, _ := NewTextBlock("t1", "Prose.")
			_, err := NewSection("s1", "T", "", []Block{text, text})
			return err
		}},
		{"topic no sections", func() error { _, err := NewTopic("t", "T", "", "g", 1, nil); return err }},
		{"topic duplicate section ids", func() error {
			sec, _ := NewSection("s1", "T", "", []Block{mustText("t1", "P.")})
			_, err := NewTopic("t", "T", "", "g", 1, []Section{sec, sec})
			return err
		}},
		{"group blank id", func() error { _, err := NewTopicGroup("", "T", 1); return err }},
		{"group blank title", func() error { _, err := NewTopicGroup("g", " ", 1); return err }},
	}
	for _, tc := range cases {
		if err := tc.build(); err == nil {
			t.Errorf("%s: expected an error, got none", tc.name)
		}
	}
}

func mustText(id, text string) TextBlock {
	block, err := NewTextBlock(id, text)
	if err != nil {
		panic(err)
	}
	return block
}
