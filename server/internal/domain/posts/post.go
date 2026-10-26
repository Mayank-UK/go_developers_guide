// Package posts is the guide's real resource domain: the Post entity, the
// PostsRepository port (the seam storage adapters implement), and the use
// cases the posts HTTP handlers delegate to. Later topics reuse this exact
// substrate — authorization guards /api/posts/{id} (topic 24), webhooks
// fire off its writes (topic 14), and the docs topics document its routes —
// which is why the layering here is the reference example of the family
// architecture: presentation → domain ← data.
package posts

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Field-length rules for a post. Lengths count runes (human characters),
// not bytes, so an accented title is not silently half its apparent length.
const (
	maxTitleRunes = 120
	maxBodyRunes  = 5000
	maxTags       = 8
	maxTagRunes   = 24
)

// Post is the resource entity: everything a post IS, independent of any
// storage or wire shape. Tags are normalized on write (trimmed, lowercased)
// so filtering can be exact-match on read. Timestamps are UTC instants set
// by the repository when a post is stored — the same contract a database's
// defaults would honor.
type Post struct {
	ID        string
	Title     string
	Body      string
	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Draft is a post without identity or timestamps: the input shape the
// create and replace use cases validate, normalize, and hand to the
// repository. The repository stamps ID, CreatedAt, and UpdatedAt — on
// replace, CreatedAt is preserved and only UpdatedAt moves.
type Draft struct {
	Title string
	Body  string
	Tags  []string
}

// Issue is one validation finding, structured for the error envelope's
// details: which field, which rule, what the server actually got. A client
// renders these per field without parsing prose — the same discipline the
// validation topic's arena teaches.
type Issue struct {
	Field string `json:"field"`
	Rule  string `json:"rule"`
	Got   string `json:"got"`
}

// NormalizeDraft trims the title and body and normalizes every tag
// (trimmed, lowercased, blanks dropped) — the write-side canonicalization
// that makes read-side exact-match filtering honest. Normalization happens
// before validation so rules judge canonical text, never "go " vs "go".
func (d Draft) NormalizeDraft() Draft {
	tags := make([]string, 0, len(d.Tags))
	for _, tag := range d.Tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized != "" {
			tags = append(tags, normalized)
		}
	}
	return Draft{
		Title: strings.TrimSpace(d.Title),
		Body:  strings.TrimSpace(d.Body),
		Tags:  tags,
	}
}

// ValidateDraft reports every rule the draft breaks against the normalized
// form. It collects all issues instead of stopping at the first: one 422
// naming every broken field beats a caller discovering them one round trip
// at a time. An empty result means the draft is storable.
func (d Draft) ValidateDraft() []Issue {
	var issues []Issue
	switch {
	case d.Title == "":
		issues = append(issues, Issue{
			Field: "title",
			Rule:  "required — present-but-empty is data, and data can be wrong",
			Got:   strconv.Quote(d.Title),
		})
	case utf8.RuneCountInString(d.Title) > maxTitleRunes:
		issues = append(issues, Issue{
			Field: "title",
			Rule:  fmt.Sprintf("at most %d runes, trimmed", maxTitleRunes),
			Got:   fmt.Sprintf("%d runes", utf8.RuneCountInString(d.Title)),
		})
	}
	switch {
	case d.Body == "":
		issues = append(issues, Issue{
			Field: "body",
			Rule:  "required — present-but-empty is data, and data can be wrong",
			Got:   strconv.Quote(d.Body),
		})
	case utf8.RuneCountInString(d.Body) > maxBodyRunes:
		issues = append(issues, Issue{
			Field: "body",
			Rule:  fmt.Sprintf("at most %d runes, trimmed", maxBodyRunes),
			Got:   fmt.Sprintf("%d runes", utf8.RuneCountInString(d.Body)),
		})
	}
	if len(d.Tags) > maxTags {
		issues = append(issues, Issue{
			Field: "tags",
			Rule:  fmt.Sprintf("at most %d tags after normalization (blank entries drop out)", maxTags),
			Got:   fmt.Sprintf("%d tags", len(d.Tags)),
		})
	}
	for _, tag := range d.Tags {
		if utf8.RuneCountInString(tag) > maxTagRunes {
			issues = append(issues, Issue{
				Field: "tags",
				Rule:  fmt.Sprintf("each tag at most %d runes after trimming", maxTagRunes),
				Got:   strconv.Quote(tag),
			})
			break
		}
	}
	return issues
}
