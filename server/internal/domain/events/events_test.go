package events

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"guide/internal/core"
)

// The domain's own rules, pinned without any store: normalization
// canonicalizes, validation collects EVERY issue, the query boundary
// judges the closed vocabulary, and — the proof the mongodb topic quotes —
// RecordEvent refuses an invalid draft BEFORE the store is consulted
// (the fake below fails the test the moment the port is touched).

// refusedStore fails the test if any port method is called — the zero-IO
// proof for invalid drafts.
type refusedStore struct {
	t *testing.T
}

func (s refusedStore) Upsert(context.Context, Event) (Event, error) {
	s.t.Fatal("Upsert must not be called for an invalid draft — MongoDB never sees it")
	return Event{}, nil
}

func (s refusedStore) Recent(context.Context, Query) ([]Event, error) {
	s.t.Fatal("Recent must not be called for an invalid query")
	return nil, nil
}

func (s refusedStore) Count(context.Context, Query) (int64, error) {
	s.t.Fatal("Count must not be called for an invalid query")
	return 0, nil
}

func TestEventNormalizeDraftCanonicalizesEverything(t *testing.T) {
	draft := Draft{
		Key:        "  Deploy-API-0309  ",
		Type:       "Deploy",
		Severity:   "INFO",
		Source:     " ci ",
		Message:    "  rolled out  ",
		Tags:       []string{" Prod ", "prod", "", "Latency", "prod"},
		OccurredAt: time.Date(2026, 3, 9, 8, 55, 0, 0, time.UTC),
	}

	normalized := draft.NormalizeDraft()

	if normalized.Key != "Deploy-API-0309" {
		t.Errorf("key = %q, want trimmed only (case is validation's to judge)", normalized.Key)
	}
	if normalized.Type != "deploy" || normalized.Severity != "info" {
		t.Errorf("type/severity = %q/%q, want lowercased", normalized.Type, normalized.Severity)
	}
	if normalized.Source != "ci" || normalized.Message != "rolled out" {
		t.Errorf("source/message = %q/%q, want trimmed", normalized.Source, normalized.Message)
	}
	wantTags := []string{"latency", "prod"}
	if !reflect.DeepEqual(normalized.Tags, wantTags) {
		t.Errorf("tags = %v, want de-blanked, de-duplicated, sorted %v", normalized.Tags, wantTags)
	}
}

func TestEventValidateDraftCollectsEveryIssue(t *testing.T) {
	draft := Draft{
		Key:        "UPPER KEY",
		Type:       "restart",
		Severity:   "sev1",
		Source:     "",
		Message:    "",
		Tags:       []string{"a", "b", "c", "d", "e", "f", "g"},
		OccurredAt: time.Time{},
	}

	issues := draft.NormalizeDraft().ValidateDraft()
	fields := make([]string, 0, len(issues))
	for _, issue := range issues {
		fields = append(fields, issue.Field)
	}
	want := []string{"key", "type", "severity", "source", "message", "tags", "occurredAt"}
	if !reflect.DeepEqual(fields, want) {
		t.Errorf("issue fields = %v, want every broken field %v", fields, want)
	}
}

func TestEventValidateDraftAcceptsACleanDraft(t *testing.T) {
	draft := Draft{
		Key: "deploy-api-0309", Type: TypeDeploy, Severity: SeverityCritical,
		Source: "ci", Message: "rolled out", Tags: []string{"prod"},
		OccurredAt: time.Date(2026, 3, 9, 8, 55, 0, 0, time.UTC),
	}
	if issues := draft.NormalizeDraft().ValidateDraft(); len(issues) != 0 {
		t.Errorf("clean draft has issues: %+v", issues)
	}
}

func TestEventRecordRefusesInvalidDraftBeforeAnyIO(t *testing.T) {
	record := NewRecordEventUseCase(refusedStore{t: t})

	draft := Draft{
		Key: "!!!not-kebab!!!", Type: "deploy", Severity: "info",
		Source: "ci", Message: "never stored",
		Tags: nil, OccurredAt: time.Date(2026, 3, 9, 8, 55, 0, 0, time.UTC),
	}
	stored, err := record.Execute(context.Background(), draft)
	if err == nil {
		t.Fatal("invalid draft must be refused")
	}
	if !errors.Is(err, core.ErrValidationFailed) {
		t.Errorf("refusal kind = %v, want validation_failed", err)
	}
	if stored.ID != "" || stored.Key != "" {
		t.Errorf("refused record returned %+v, want the zero event", stored)
	}
}

func TestEventNormalizeQueryJudgesTheClosedVocabulary(t *testing.T) {
	if _, err := NormalizeQuery("restart", "", ""); err == nil {
		t.Error("unknown type must be refused")
	}
	if _, err := NormalizeQuery("deploy", "PROD", ""); err == nil {
		t.Error("non-kebab tag must be refused")
	}
	if _, err := NormalizeQuery("deploy", "prod", "0"); err == nil {
		t.Error("limit 0 must be refused")
	}
	query, err := NormalizeQuery("", "", "")
	if err != nil {
		t.Fatalf("empty query must pass: %v", err)
	}
	if query.Limit != DefaultQueryLimit || query.Type != "" || query.Tag != "" {
		t.Errorf("default query = %+v, want no filters at the default limit", query)
	}
	query, err = NormalizeQuery("alert", "prod", "3")
	if err != nil {
		t.Fatalf("valid query must pass: %v", err)
	}
	if query.Type != "alert" || query.Tag != "prod" || query.Limit != 3 {
		t.Errorf("query = %+v, want the sent filters", query)
	}
}

func TestEventRecentNeverServesNil(t *testing.T) {
	listed, err := NewRecentEventsUseCase(nilStore{}).Execute(context.Background(), Query{Limit: 10})
	if err != nil {
		t.Fatalf("recent over the nil-returning store: %v", err)
	}
	if listed == nil {
		t.Error("empty result must be [], never nil")
	}
	if len(listed) != 0 {
		t.Errorf("len = %d, want 0", len(listed))
	}
}

// nilStore is the smallest honest fake: nothing stored, no matches.
type nilStore struct{}

func (nilStore) Upsert(ctx context.Context, event Event) (Event, error) { return event, nil }
func (nilStore) Recent(context.Context, Query) ([]Event, error)         { return nil, nil }
func (nilStore) Count(context.Context, Query) (int64, error)            { return 0, nil }
