// Package content is the typed teaching model the React client renders:
// topic → sections → blocks, plus the registry that validates it at startup
// and the snippet matcher that keeps every quoted snippet honest.
//
// The server owns 100% of the teaching as data. Wire fields are camelCase
// (struct tags) and mirror the fastapi guide's content API verbatim — the
// client's DTO layer (client/src/data/content/content-dto.ts) is the
// binding contract: optional fields are OMITTED when unset (never null),
// and blocks discriminate on the "kind" string.
package content

import (
	"fmt"
	"strings"
)

// ContentAPIVersion is served on both content endpoints so a client can
// detect a wire-shape bump and migrate deliberately. Bump only with a
// deliberate contract change.
const ContentAPIVersion = 1

// Block kind tags — the wire discriminator each concrete block carries.
const (
	KindText = "text"
	KindCode = "code"
	KindDemo = "demo"
	KindList = "list"
)

// MustSourceLanguages are the languages whose snippets are real code or
// config a repo file can own: every code block in one of them MUST carry a
// repo-root-relative source and pass the snippet matcher (RATCHET = 0).
// Derived from the family rule (fastapi: py, ts, tsx, js, toml, yaml, yml,
// sql, json, proto, html, css) adapted for the Go guide: py → go, plus the
// Go-specific mod (go.mod) and xml.
//
// Deliberately exempt (no source file exists to quote): bash, sh, shell,
// dockerfile, text — terminal tours and output transcripts. Never relabel
// a language to dodge the check.
var MustSourceLanguages = map[string]bool{
	"go":    true,
	"mod":   true,
	"yaml":  true,
	"sql":   true,
	"proto": true,
	"ts":    true,
	"tsx":   true,
	"json":  true,
	"toml":  true,
	"xml":   true,
}

// List item count bounds (block-selection standard): enough items to be a
// real enumeration, few enough to stay one scannable unit.
const (
	ListItemsMin = 3
	ListItemsMax = 8
)

// Block is one renderable unit inside a section. The unexported blockKind
// method seals the hierarchy: only the block types in this package can
// implement Block, and the registry can verify each wire "kind" tag matches
// its concrete type.
type Block interface {
	blockKind() string
}

// TextBlock is prose. May contain \n\n paragraph breaks; never blank.
type TextBlock struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Text string `json:"text"`
}

// CodeBlock is a code snippet, quoted from a real repo file when its
// language is must-source. Language is a free string (the registry and the
// matcher know which are must-source); source is repo-root-relative or
// empty for exempt languages; snippet is the displayed code itself.
type CodeBlock struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Language string `json:"language"`
	Caption  string `json:"caption,omitempty"`
	Source   string `json:"source,omitempty"`
	Snippet  string `json:"snippet"`
}

// DemoBlock names a client-side live demo bound to real server endpoints.
// The block id doubles as the demo id ("<area>:<name>", e.g.
// "runtime:async") — the client demo registry's lookup key, validated by
// the registry.
type DemoBlock struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// ListBlock is a flat enumeration of parallel items (block-selection
// standard): 3–8 unique items, each written as "Lead term — explanation.";
// ordered is true only when sequence matters. No nesting — a sub-list means
// the content wants sections or reworded items instead.
type ListBlock struct {
	Kind    string   `json:"kind"`
	ID      string   `json:"id"`
	Ordered bool     `json:"ordered"`
	Items   []string `json:"items"`
}

func (TextBlock) blockKind() string { return KindText }
func (CodeBlock) blockKind() string { return KindCode }
func (DemoBlock) blockKind() string { return KindDemo }
func (ListBlock) blockKind() string { return KindList }

// Section is one teaching unit inside a topic: prose + code + its inline
// demo. Lead, when set, is the one-sentence framing under the title.
type Section struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Lead   string  `json:"lead,omitempty"`
	Blocks []Block `json:"blocks"`
}

// Topic is a whole guide page: ordered sections, the unit the detail route
// serves.
type Topic struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Subtitle string    `json:"subtitle,omitempty"`
	Group    string    `json:"group"`
	Order    int       `json:"order"`
	Sections []Section `json:"sections"`
}

// TopicGroup is a nav group topics belong to; order drives nav display
// order.
type TopicGroup struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Order int    `json:"order"`
}

// TopicSummary is a topic without its sections payload — what the list
// route serves.
type TopicSummary struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Subtitle     string `json:"subtitle,omitempty"`
	Group        string `json:"group"`
	Order        int    `json:"order"`
	SectionCount int    `json:"sectionCount"`
}

// ContentListResponse is the wire shape of GET /api/content.
type ContentListResponse struct {
	Version int            `json:"version"`
	Groups  []TopicGroup   `json:"groups"`
	Topics  []TopicSummary `json:"topics"`
}

// ContentDetail is the detail route's response model: a full topic plus the
// API version. Family contract: BOTH content endpoints serve version so a
// client can detect a ContentAPIVersion bump on any content response.
type ContentDetail struct {
	Version int `json:"version"`
	Topic
}

func requireNonBlank(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be blank", field)
	}
	return nil
}

// NewTextBlock builds a prose block; the text must have real content.
func NewTextBlock(id, text string) (TextBlock, error) {
	if err := requireNonBlank("text block id", id); err != nil {
		return TextBlock{}, err
	}
	if err := requireNonBlank("text block text", text); err != nil {
		return TextBlock{}, err
	}
	return TextBlock{Kind: KindText, ID: id, Text: text}, nil
}

// NewCodeBlock builds a code block. Caption and source may be empty (the
// registry enforces that must-source languages carry a source); the
// language and the snippet must be non-blank.
func NewCodeBlock(id, language, caption, source, snippet string) (CodeBlock, error) {
	if err := requireNonBlank("code block id", id); err != nil {
		return CodeBlock{}, err
	}
	if err := requireNonBlank("code block language", language); err != nil {
		return CodeBlock{}, err
	}
	if err := requireNonBlank("code block snippet", snippet); err != nil {
		return CodeBlock{}, err
	}
	return CodeBlock{
		Kind:     KindCode,
		ID:       id,
		Language: language,
		Caption:  caption,
		Source:   source,
		Snippet:  snippet,
	}, nil
}

// NewDemoBlock names a live demo; the title is what the demo chrome shows.
func NewDemoBlock(id, title, description string) (DemoBlock, error) {
	if err := requireNonBlank("demo block id", id); err != nil {
		return DemoBlock{}, err
	}
	if err := requireNonBlank("demo block title", title); err != nil {
		return DemoBlock{}, err
	}
	return DemoBlock{Kind: KindDemo, ID: id, Title: title, Description: description}, nil
}

// NewListBlock builds an enumeration: 3–8 unique, non-blank items. Whether
// items follow the "Lead term — explanation." grammar is an authoring rule,
// not an engine invariant.
func NewListBlock(id string, ordered bool, items []string) (ListBlock, error) {
	if err := requireNonBlank("list block id", id); err != nil {
		return ListBlock{}, err
	}
	if len(items) < ListItemsMin || len(items) > ListItemsMax {
		return ListBlock{}, fmt.Errorf(
			"list block %q has %d items; the standard allows %d–%d",
			id, len(items), ListItemsMin, ListItemsMax,
		)
	}
	seen := make(map[string]bool, len(items))
	copied := make([]string, len(items))
	for i, item := range items {
		if strings.TrimSpace(item) == "" {
			return ListBlock{}, fmt.Errorf("list block %q item %d is blank", id, i+1)
		}
		if seen[item] {
			return ListBlock{}, fmt.Errorf("list block %q repeats item %q", id, item)
		}
		seen[item] = true
		copied[i] = item
	}
	return ListBlock{Kind: KindList, ID: id, Ordered: ordered, Items: copied}, nil
}

// NewSection builds one teaching unit: at least one block, no nil blocks,
// and no duplicate block ids within the section.
func NewSection(id, title, lead string, blocks []Block) (Section, error) {
	if err := requireNonBlank("section id", id); err != nil {
		return Section{}, err
	}
	if err := requireNonBlank("section title", title); err != nil {
		return Section{}, err
	}
	if len(blocks) == 0 {
		return Section{}, fmt.Errorf("section %q needs at least one block", id)
	}
	seen := make(map[string]bool, len(blocks))
	for _, block := range blocks {
		if block == nil {
			return Section{}, fmt.Errorf("section %q contains a nil block", id)
		}
		blockID := blockIDOf(block)
		if seen[blockID] {
			return Section{}, fmt.Errorf("duplicate block id %q in section %q", blockID, id)
		}
		seen[blockID] = true
	}
	return Section{ID: id, Title: title, Lead: lead, Blocks: blocks}, nil
}

// NewTopic builds a whole guide page: at least one section, and no
// duplicate section ids.
func NewTopic(id, title, subtitle, group string, order int, sections []Section) (Topic, error) {
	if err := requireNonBlank("topic id", id); err != nil {
		return Topic{}, err
	}
	if err := requireNonBlank("topic title", title); err != nil {
		return Topic{}, err
	}
	if err := requireNonBlank("topic group", group); err != nil {
		return Topic{}, err
	}
	if len(sections) == 0 {
		return Topic{}, fmt.Errorf("topic %q needs at least one section", id)
	}
	seen := make(map[string]bool, len(sections))
	for _, section := range sections {
		if seen[section.ID] {
			return Topic{}, fmt.Errorf("duplicate section id %q in topic %q", section.ID, id)
		}
		seen[section.ID] = true
	}
	return Topic{
		ID:       id,
		Title:    title,
		Subtitle: subtitle,
		Group:    group,
		Order:    order,
		Sections: sections,
	}, nil
}

// NewTopicGroup builds a nav group.
func NewTopicGroup(id, title string, order int) (TopicGroup, error) {
	if err := requireNonBlank("group id", id); err != nil {
		return TopicGroup{}, err
	}
	if err := requireNonBlank("group title", title); err != nil {
		return TopicGroup{}, err
	}
	return TopicGroup{ID: id, Title: title, Order: order}, nil
}

// blockIDOf returns the id of any concrete block (the registry's error
// messages use it without a type switch).
func blockIDOf(block Block) string {
	switch b := block.(type) {
	case TextBlock:
		return b.ID
	case CodeBlock:
		return b.ID
	case DemoBlock:
		return b.ID
	case ListBlock:
		return b.ID
	default:
		return ""
	}
}
