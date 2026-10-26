// Package events is the guide's document-store domain area: a small event
// log — deploys, alerts, scaling decisions, backups — whose shape is the
// topic 19 teaching case for "when a document model fits". A MongoDB
// document has no schema the database will enforce, so THIS package is
// where an event's rules live: normalization and validation happen in the
// application layer, before any driver is consulted. The storage port is
// repository.go; the MongoDB adapter behind it lives in
// internal/data/mongodb.
package events

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Closed vocabularies — the words the boundary accepts. A database would
// enforce these with CHECK constraints or enums; a document store cannot,
// so the closed set lives here and every write is judged against it
// before the store is consulted.
const (
	TypeDeploy  = "deploy"  // a build rolled out somewhere
	TypeAlert   = "alert"   // a monitor noticed something
	TypeScaling = "scaling" // replicas moved
	TypeBackup  = "backup"  // a snapshot landed
)

const (
	SeverityInfo     = "info"     // routine, worth keeping
	SeverityWarning  = "warning"  // a human should look soon
	SeverityCritical = "critical" // a human should look now
)

// keyPattern is the business key's grammar: lowercase kebab-case, the same
// discipline the guide's resource ids keep.
var keyPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Field bounds — the numbers validation judges against.
const (
	MaxKeyLength     = 64
	MaxSourceLength  = 32
	MaxMessageLength = 200
	MaxTags          = 5
)

// Event is one logged occurrence, as the domain sees it: the stored form
// the adapter returns (ID is the store-assigned _id, hex-encoded) and the
// unit every read path hands back. OccurredAt is the event's OWN instant —
// when the thing happened — deliberately distinct from when it was
// written.
type Event struct {
	ID         string
	Key        string
	Type       string
	Severity   string
	Source     string
	Message    string
	Tags       []string
	OccurredAt time.Time
}

// Draft is the write-side shape: everything an event carries except the
// store-assigned ID. Build one from client input, normalize, validate —
// the same three-beat boundary discipline the posts resource teaches.
type Draft struct {
	Key        string
	Type       string
	Severity   string
	Source     string
	Message    string
	Tags       []string
	OccurredAt time.Time
}

// Issue names one broken field the way the guide's error envelope serves
// it: which field, the rule it broke, what it got. Mirrors the posts
// domain's Issue so validation details render identically on the wire.
type Issue struct {
	Field string `json:"field"`
	Rule  string `json:"rule"`
	Got   string `json:"got"`
}

// NormalizeDraft canonicalizes the draft before rules judge it: strings
// trimmed, type/severity/tags lowercased, tags de-blanked, de-duplicated,
// and sorted — the write-side normalization that makes exact-match reads
// and the store's unique key honest. The zero time and empty key are NOT
// repaired here; those are validation's to reject.
func (d Draft) NormalizeDraft() Draft {
	normalized := Draft{
		Key:        strings.TrimSpace(d.Key),
		Type:       strings.ToLower(strings.TrimSpace(d.Type)),
		Severity:   strings.ToLower(strings.TrimSpace(d.Severity)),
		Source:     strings.TrimSpace(d.Source),
		Message:    strings.TrimSpace(d.Message),
		OccurredAt: d.OccurredAt,
	}
	seen := make(map[string]bool, len(d.Tags))
	for _, tag := range d.Tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		normalized.Tags = append(normalized.Tags, tag)
	}
	sort.Strings(normalized.Tags)
	return normalized
}

// ValidateDraft judges a NORMALIZED draft and collects every issue — one
// pass, all problems, so the caller answers with the complete list the
// way the guide's 422 envelope teaches. Empty means clean.
func (d Draft) ValidateDraft() []Issue {
	var issues []Issue
	add := func(field, rule, got string) {
		issues = append(issues, Issue{Field: field, Rule: rule, Got: got})
	}

	if d.Key == "" {
		add("key", "required — the stable business key upserts address", "(empty)")
	} else if len(d.Key) > MaxKeyLength {
		add("key", fmt.Sprintf("at most %d characters", MaxKeyLength), fmt.Sprintf("%d characters", len(d.Key)))
	} else if !keyPattern.MatchString(d.Key) {
		add("key", "lowercase kebab-case (letters, digits, single dashes)", d.Key)
	}

	switch d.Type {
	case TypeDeploy, TypeAlert, TypeScaling, TypeBackup:
	default:
		add("type", fmt.Sprintf("one of %q, %q, %q, or %q", TypeDeploy, TypeAlert, TypeScaling, TypeBackup), renderGot(d.Type))
	}

	switch d.Severity {
	case SeverityInfo, SeverityWarning, SeverityCritical:
	default:
		add("severity", fmt.Sprintf("one of %q, %q, or %q", SeverityInfo, SeverityWarning, SeverityCritical), renderGot(d.Severity))
	}

	if d.Source == "" {
		add("source", "required — what system emitted the event", "(empty)")
	} else if len(d.Source) > MaxSourceLength {
		add("source", fmt.Sprintf("at most %d characters", MaxSourceLength), fmt.Sprintf("%d characters", len(d.Source)))
	}

	if d.Message == "" {
		add("message", "required — one line saying what happened", "(empty)")
	} else if len(d.Message) > MaxMessageLength {
		add("message", fmt.Sprintf("at most %d characters", MaxMessageLength), fmt.Sprintf("%d characters", len(d.Message)))
	}

	if len(d.Tags) > MaxTags {
		add("tags", fmt.Sprintf("at most %d, after normalization drops blanks and duplicates", MaxTags), fmt.Sprintf("%d tags", len(d.Tags)))
	}

	if d.OccurredAt.IsZero() {
		add("occurredAt", "required — when the event happened (RFC 3339)", "(missing)")
	}

	return issues
}

// renderGot renders a possibly-empty vocabulary miss readably — an empty
// miss reads "(empty)", never a silent blank.
func renderGot(value string) string {
	if value == "" {
		return "(empty)"
	}
	return value
}
