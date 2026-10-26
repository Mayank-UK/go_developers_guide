package events

import (
	"context"

	"guide/internal/core"
)

// The use cases: one verb each, thin over the Store port, carrying every
// rule the verb owns — the same shape the posts domain teaches. The
// mongodb topic quotes RecordEvent as its "validation in the application
// layer" proof: the store is not consulted until the draft has passed
// judgment, so an invalid event never reaches the driver.

// RecordEventUseCase records one event: normalize, validate (the whole
// issue list, one typed 422), and only then hand the store a canonical
// event.
type RecordEventUseCase struct {
	store Store
}

// NewRecordEventUseCase wires the storage port.
func NewRecordEventUseCase(store Store) *RecordEventUseCase {
	return &RecordEventUseCase{store: store}
}

// Execute judges the draft, then upserts through the port. The upsert is
// idempotent, so re-recording an identical event is a no-op with a stable
// ID — exactly what re-runs and retries want.
func (uc *RecordEventUseCase) Execute(ctx context.Context, draft Draft) (Event, error) {
	normalized, err := validNormalizedDraft(draft)
	if err != nil {
		return Event{}, err
	}
	return uc.store.Upsert(ctx, storeDraft(normalized))
}

// RecentEventsUseCase lists the log for a normalized query.
type RecentEventsUseCase struct {
	store Store
}

// NewRecentEventsUseCase wires the storage port.
func NewRecentEventsUseCase(store Store) *RecentEventsUseCase {
	return &RecentEventsUseCase{store: store}
}

// Execute reads through the port, newest first; the result is never nil.
// An unreachable store surfaces as the typed service_unavailable the
// handler maps once.
func (uc *RecentEventsUseCase) Execute(ctx context.Context, query Query) ([]Event, error) {
	listed, err := uc.store.Recent(ctx, query)
	if err != nil {
		return nil, err
	}
	if listed == nil {
		return []Event{}, nil
	}
	return listed, nil
}

// validNormalizedDraft is the shared write-side gate: normalize, then
// collect every issue into one typed 422. It returns the canonical draft
// when clean.
func validNormalizedDraft(draft Draft) (Draft, error) {
	normalized := draft.NormalizeDraft()
	if issues := normalized.ValidateDraft(); len(issues) > 0 {
		return Draft{}, core.ValidationFailed("Invalid event.", issues)
	}
	return normalized, nil
}

// storeDraft carries a validated draft into the stored form the port
// accepts: the same fields, ID left for the store to assign.
func storeDraft(draft Draft) Event {
	return Event{
		Key:        draft.Key,
		Type:       draft.Type,
		Severity:   draft.Severity,
		Source:     draft.Source,
		Message:    draft.Message,
		Tags:       draft.Tags,
		OccurredAt: draft.OccurredAt,
	}
}
