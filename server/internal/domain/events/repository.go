package events

import (
	"context"
	"fmt"
	"strconv"

	"guide/internal/core"
)

// The storage port — the seam this domain is consumed through and the
// only type internal/data/mongodb knows about from here. Handlers and use
// cases depend on this interface, never on the driver; the composition
// root wires the MongoDB adapter (or a fake) behind it. Port methods
// return the app's typed errors: an unreachable store is
// core.ErrServiceUnavailable, so one mapping at the handler produces the
// honest 503 envelope for every adapter — the optional-service stance.
// There is deliberately NO validation on this side of the seam: drafts
// are judged in the use case, and the store trusts what it is handed.

// Query bounds: the defaults and the ceiling the read path enforces.
const (
	DefaultQueryLimit = 10
	MaxQueryLimit     = 50
)

// Query is a normalized, validated read: an exact type match from the
// closed vocabulary ("" means no filter), an exact tag match against the
// tag array ("" means no filter), and a limit already inside its bounds.
// Build one through NormalizeQuery — that is where the rules live.
type Query struct {
	Type  string
	Tag   string
	Limit int
}

// NormalizeQuery judges raw filter values the way the guide's boundaries
// always do: "" means "not sent" (no filter), a known value passes, an
// unknown value collects into one typed 422 naming the vocabulary.
func NormalizeQuery(eventType, tag, limit string) (Query, error) {
	var issues []Issue
	normalized := Query{Limit: DefaultQueryLimit}

	switch eventType {
	case "":
	case TypeDeploy, TypeAlert, TypeScaling, TypeBackup:
		normalized.Type = eventType
	default:
		issues = append(issues, Issue{
			Field: "type",
			Rule:  fmt.Sprintf("one of %q, %q, %q, or %q, or omit for all types", TypeDeploy, TypeAlert, TypeScaling, TypeBackup),
			Got:   strconv.Quote(eventType),
		})
	}

	if tag != "" && !keyPattern.MatchString(tag) {
		issues = append(issues, Issue{
			Field: "tag",
			Rule:  "lowercase kebab-case, or omit for no tag filter",
			Got:   strconv.Quote(tag),
		})
	} else {
		normalized.Tag = tag
	}

	if limit == "" {
		// DefaultQueryLimit already set.
	} else if parsed, err := strconv.Atoi(limit); err != nil || parsed < 1 || parsed > MaxQueryLimit {
		issues = append(issues, Issue{
			Field: "limit",
			Rule:  fmt.Sprintf("an integer 1–%d when sent; omit for %d", MaxQueryLimit, DefaultQueryLimit),
			Got:   strconv.Quote(limit),
		})
	} else {
		normalized.Limit = parsed
	}

	if len(issues) > 0 {
		return Query{}, core.ValidationFailed("Invalid event query.", issues)
	}
	return normalized, nil
}

// Store is the event log's storage port. Upsert is keyed by the event's
// business key: re-recording the same key replaces the same document —
// the idempotency a re-run seed and an at-least-once pipeline both need.
type Store interface {
	// Upsert writes the event keyed by Key and returns the stored form —
	// including the store-assigned ID, exactly as the database states it.
	Upsert(ctx context.Context, event Event) (Event, error)
	// Recent returns events matching the query, newest first, capped at
	// the query's limit. The result is never nil: no match renders [].
	Recent(ctx context.Context, query Query) ([]Event, error)
	// Count returns how many events match the query's filters.
	Count(ctx context.Context, query Query) (int64, error)
}
