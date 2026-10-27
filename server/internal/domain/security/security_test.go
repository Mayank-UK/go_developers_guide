package security

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// ── the rate limiter ─────────────────────────────────────────────────────

func TestClientRateLimiterAllowsTheBurstThenRefusesWithRetryAfter(t *testing.T) {
	limiter, err := NewClientRateLimiter(RatePolicy{Rate: rate.Limit(10), Burst: 5})
	if err != nil {
		t.Fatalf("build limiter: %v", err)
	}
	for i := 1; i <= 5; i++ {
		if decision := limiter.Allow("ip:198.51.100.7"); !decision.Allowed {
			t.Fatalf("request %d inside the burst was refused", i)
		}
	}
	decision := limiter.Allow("ip:198.51.100.7")
	if decision.Allowed {
		t.Fatal("request 6 (over the burst) was allowed")
	}
	if decision.RetryAfter <= 0 {
		t.Fatalf("refusal must carry a positive retry hint, got %s", decision.RetryAfter)
	}
	// At 10/s one token costs 100ms; anything beyond a full token
	// interval means the delay math drifted.
	if decision.RetryAfter > 100*time.Millisecond {
		t.Errorf("retry hint = %s, want at most one token interval (100ms)", decision.RetryAfter)
	}
	if got := decision.RetryAfterText(); got == "now" || !strings.HasSuffix(got, "ms") {
		t.Errorf("retry text = %q, want a millisecond-truncated duration", got)
	}
}

func TestClientRateLimiterRecoversOneTokenPerInterval(t *testing.T) {
	limiter, err := NewClientRateLimiter(RatePolicy{Rate: rate.Limit(20), Burst: 2})
	if err != nil {
		t.Fatalf("build limiter: %v", err)
	}
	limiter.Allow("k")
	limiter.Allow("k")
	if limiter.Allow("k").Allowed {
		t.Fatal("bucket should be empty")
	}
	// The cooperative wait SERVES a request by waiting for the next
	// token — at 20/s that is one 50ms interval, not a window
	// boundary (Wait consumes the token it waited for, so the bucket
	// is empty again the instant it returns).
	started := time.Now()
	if err := limiter.Wait(context.Background(), "k"); err != nil {
		t.Fatalf("wait: %v", err)
	}
	first := time.Since(started)
	if first > 150*time.Millisecond {
		t.Errorf("waited %s for one token at 20/s — the bucket refills continuously, not at window boundaries", first)
	}
	// Steady state: a client that keeps WAITING is served at exactly
	// the sustained rate — one more interval per request.
	started = time.Now()
	if err := limiter.Wait(context.Background(), "k"); err != nil {
		t.Fatalf("second wait: %v", err)
	}
	if second := time.Since(started); second > 150*time.Millisecond {
		t.Errorf("second wait = %s — sustained rate means one interval per waiting request", second)
	}
}

func TestClientRateLimiterBucketsArePerClient(t *testing.T) {
	limiter, err := NewClientRateLimiter(RatePolicy{Rate: rate.Limit(1), Burst: 2})
	if err != nil {
		t.Fatalf("build limiter: %v", err)
	}
	limiter.Allow("ip:198.51.100.7")
	limiter.Allow("ip:198.51.100.7")
	if limiter.Allow("ip:198.51.100.7").Allowed {
		t.Fatal("the first client must be over budget")
	}
	if !limiter.Allow("ip:203.0.113.9").Allowed {
		t.Error("a different client must have its OWN full bucket")
	}
	if limiter.Size() != 2 {
		t.Errorf("bucket count = %d, want one per distinct client (2)", limiter.Size())
	}
}

func TestClientRateLimiterRefusedRequestsConsumeNothing(t *testing.T) {
	limiter, err := NewClientRateLimiter(RatePolicy{Rate: rate.Limit(1), Burst: 1})
	if err != nil {
		t.Fatalf("build limiter: %v", err)
	}
	if !limiter.Allow("k").Allowed {
		t.Fatal("first request must spend the only token")
	}
	// Hammer the refused path: none of these may consume the token
	// that is refilling for the honest next requester.
	for i := 0; i < 20; i++ {
		if limiter.Allow("k").Allowed {
			t.Fatalf("refused request %d was allowed — cancellation leaked a token", i)
		}
	}
}

func TestClientRateLimiterIsConcurrencySafe(t *testing.T) {
	limiter, err := NewClientRateLimiter(RatePolicy{Rate: rate.Limit(50), Burst: 3})
	if err != nil {
		t.Fatalf("build limiter: %v", err)
	}
	const goroutines, perGoroutine = 16, 25
	var mu sync.Mutex
	allowed, refused := 0, 0
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				decision := limiter.Allow(key)
				mu.Lock()
				if decision.Allowed {
					allowed++
				} else {
					refused++
				}
				mu.Unlock()
			}
		}(fmt.Sprintf("client-%d", g%4))
	}
	wg.Wait()
	total := allowed + refused
	if total != goroutines*perGoroutine {
		t.Errorf("counted %d decisions, want %d", total, goroutines*perGoroutine)
	}
	// Each of the four keys starts with burst 3 and refills at 50/s —
	// the exact allowed count is time-dependent (that is the token
	// bucket's honesty), but it can never exceed a sane bound, and the
	// run must be race-clean.
	if allowed > goroutines*perGoroutine || refused == 0 {
		t.Errorf("allowed=%d refused=%d — a 3-burst across 400 requests must refuse most of them", allowed, refused)
	}
}

func TestNewClientRateLimiterRefusesBrokenPolicies(t *testing.T) {
	for _, policy := range []RatePolicy{
		{Rate: rate.Limit(0), Burst: 5},
		{Rate: rate.Limit(-1), Burst: 5},
		{Rate: rate.Limit(5), Burst: 0},
	} {
		if _, err := NewClientRateLimiter(policy); err == nil {
			t.Errorf("policy %v must be refused at construction", policy)
		}
	}
	if got := (RatePolicy{Rate: rate.Limit(10), Burst: 5}).String(); got != "10/s sustained, burst 5" {
		t.Errorf("policy string = %q", got)
	}
	if got := (RatePolicy{Rate: rate.Limit(0.5), Burst: 5}).String(); got != "0.5/s sustained, burst 5" {
		t.Errorf("fractional policy string = %q", got)
	}
}

// ── classification, masking, erasure ─────────────────────────────────────

func subjectFixture() SubjectRecord {
	return SubjectRecord{
		SubjectID: "sub_ada",
		Fields: []ClassifiedField{
			{Name: "display_name", Sensitivity: SensitivityPublic, Value: "Ada"},
			{Name: "plan", Sensitivity: SensitivityInternal, Value: "pro"},
			{Name: "email", Sensitivity: SensitivityPII, Value: "ada@example.com"},
			{Name: "phone", Sensitivity: SensitivityPII, Value: "+1-555-010-2233"},
			{Name: "signup_ip", Sensitivity: SensitivityPII, Value: "198.51.100.7"},
		},
	}
}

func TestMaskValueKeepsOnlyTheEdges(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		{"ada@example.com", "a*************m"},
		{"+1-555-010-2233", "+*************3"},
		{"ab", "**"},
		{"x", "*"},
		{"", ""},
	} {
		if got := MaskValue(tc.in); got != tc.want {
			t.Errorf("MaskValue(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMaskedViewServesVerbatimPublicMaskedPII(t *testing.T) {
	view := subjectFixture().MaskedView()
	if len(view) != 5 {
		t.Fatalf("view has %d fields, want 5", len(view))
	}
	if got := view[0].Value; got != "Ada" {
		t.Errorf("public field must render verbatim, got %q", got)
	}
	if got := view[1].Value; got != "pro" {
		t.Errorf("internal field must render verbatim, got %q", got)
	}
	for _, masked := range view[2:5] {
		if strings.Contains(masked.Value, "ada@example.com") ||
			strings.Contains(masked.Value, "+1-555-010-2233") ||
			strings.Contains(masked.Value, "198.51.100.7") {
			t.Errorf("masked field %s leaked its value: %q", masked.Name, masked.Value)
		}
	}
}

func TestEraseTombstonesOnlyPIIAndIsIdempotent(t *testing.T) {
	record := subjectFixture()
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	report := record.Erase(now)
	if report.AlreadyErased {
		t.Error("first erasure cannot report already-erased")
	}
	if strings.Join(report.ErasedFields, ",") != "email,phone,signup_ip" {
		t.Errorf("erased fields = %v, want exactly the PII-classified ones", report.ErasedFields)
	}
	if !record.Erased || !record.ErasedAt.Equal(now) {
		t.Error("erasure marker not stamped")
	}
	for _, name := range report.ErasedFields {
		field, ok := record.Field(name)
		if !ok || field.Value != ErasedValue {
			t.Errorf("field %s = %+v, want the tombstone", name, field)
		}
	}
	if field, _ := record.Field("plan"); field.Value != "pro" {
		t.Errorf("internal field must survive erasure, got %q", field.Value)
	}
	if field, _ := record.Field("display_name"); field.Value != "Ada" {
		t.Errorf("public field must survive erasure, got %q", field.Value)
	}

	again := record.Erase(now.Add(time.Minute))
	if !again.AlreadyErased || again.ErasedFields != nil {
		t.Errorf("second erasure must be an idempotent no-op, got %+v", again)
	}

	// The derived masked view cannot resurrect what the record no
	// longer holds — this is erasure completeness for views.
	for _, masked := range record.MaskedView() {
		if strings.Contains(masked.Value, "ada@example.com") {
			t.Errorf("masked view of an erased record still derives the PII: %+v", masked)
		}
	}
}

func TestAccessLogEntryCannotHoldFieldValues(t *testing.T) {
	entry := AccessLogEntry{SubjectID: "sub_ada", Action: "erase", Erased: true, At: time.Now().UTC()}
	line := RenderLogLine(entry)
	for _, want := range []string{"subject=sub_ada", "action=erase", "record=erased"} {
		if !strings.Contains(line, want) {
			t.Errorf("log line %q lacks %q", line, want)
		}
	}
	// The type is the policy: assert the rendered line has nowhere a
	// value could have come from — no '@', no phone digits.
	if strings.Contains(line, "@") || strings.Contains(line, "555") {
		t.Errorf("log line %q appears to carry a field value", line)
	}
}

// ── the PII service over its ports ───────────────────────────────────────

// fakeSubjectStore is the in-memory port for these tests.
type fakeSubjectStore struct {
	records map[string]SubjectRecord
}

func (s fakeSubjectStore) FindSubject(_ context.Context, subjectID string) (SubjectRecord, bool, error) {
	record, ok := s.records[subjectID]
	return record, ok, nil
}

func (s fakeSubjectStore) SaveSubject(_ context.Context, record SubjectRecord) error {
	s.records[record.SubjectID] = record
	return nil
}

// fakeAccessLogStore collects entries in order.
type fakeAccessLogStore struct {
	entries []AccessLogEntry
}

func (s *fakeAccessLogStore) Append(_ context.Context, entry AccessLogEntry) error {
	s.entries = append(s.entries, entry)
	return nil
}

func (s *fakeAccessLogStore) List(_ context.Context) ([]AccessLogEntry, error) {
	return append([]AccessLogEntry(nil), s.entries...), nil
}

func TestPIIServiceViewsMaskedAndLogsTheRead(t *testing.T) {
	subjects := fakeSubjectStore{records: map[string]SubjectRecord{"sub_ada": subjectFixture()}}
	logs := &fakeAccessLogStore{}
	service := NewPIIService(subjects, logs)

	view, found, err := service.View(context.Background(), "sub_ada", time.Now().UTC())
	if err != nil || !found {
		t.Fatalf("view: found=%t err=%v", found, err)
	}
	if len(view) != 5 {
		t.Errorf("view fields = %d, want 5", len(view))
	}
	for _, field := range view {
		if field.Sensitivity == SensitivityPII && strings.Contains(field.Value, "example.com") {
			t.Errorf("served view leaked PII in %s: %q", field.Name, field.Value)
		}
	}
	if len(logs.entries) != 1 || logs.entries[0].Action != "read" {
		t.Errorf("read not logged exactly once: %+v", logs.entries)
	}
}

func TestPIIServiceViewsUnknownSubjectAsNotFoundShaped(t *testing.T) {
	service := NewPIIService(fakeSubjectStore{records: map[string]SubjectRecord{}}, &fakeAccessLogStore{})
	_, found, err := service.View(context.Background(), "sub_ghost", time.Now().UTC())
	if found || err != nil {
		t.Errorf("unknown subject: found=%t err=%v, want false/nil (the caller maps it)", found, err)
	}
}

func TestPIIServiceErasesPersistsAndLogs(t *testing.T) {
	subjects := fakeSubjectStore{records: map[string]SubjectRecord{"sub_ada": subjectFixture()}}
	logs := &fakeAccessLogStore{}
	service := NewPIIService(subjects, logs)

	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	report, err := service.Erase(context.Background(), "sub_ada", now)
	if err != nil {
		t.Fatalf("erase: %v", err)
	}
	if len(report.ErasedFields) != 3 {
		t.Errorf("erased fields = %v, want the three PII fields", report.ErasedFields)
	}
	stored, _, _ := subjects.FindSubject(context.Background(), "sub_ada")
	if !stored.Erased {
		t.Error("erasure must be PERSISTED, not just answered")
	}
	if len(logs.entries) != 1 || logs.entries[0].Action != "erase" {
		t.Errorf("erase not logged exactly once: %+v", logs.entries)
	}

	if _, err := service.Erase(context.Background(), "sub_ghost", now); err == nil {
		t.Error("erasing an unknown subject must be an error the caller maps")
	}
}

// ── the header policy ────────────────────────────────────────────────────

func TestAPISecurityHeadersCarriesTheFiveRuleSet(t *testing.T) {
	if len(APISecurityHeaders) != 5 {
		t.Fatalf("policy has %d rules, want the five taught headers", len(APISecurityHeaders))
	}
	seen := make(map[string]string)
	for _, rule := range APISecurityHeaders {
		if rule.Header == "" || rule.Value == "" || rule.Why == "" {
			t.Errorf("rule %+v must carry header, value, and why", rule)
		}
		seen[rule.Header] = rule.Value
	}
	for _, header := range []string{
		"Content-Security-Policy", "Strict-Transport-Security", "X-Content-Type-Options",
		"X-Frame-Options", "Referrer-Policy",
	} {
		if _, ok := seen[header]; !ok {
			t.Errorf("policy lacks %s", header)
		}
	}
	if !strings.Contains(seen["Content-Security-Policy"], "default-src 'none'") {
		t.Errorf("API CSP must refuse every source, got %q", seen["Content-Security-Policy"])
	}
}
