package security

import (
	"context"
	"fmt"

	domsecurity "guide/internal/domain/security"
)

// The demo policies, stated once where the wiring and the teaching
// both read them. The story limiter is fast enough that its whole
// window (burst, wall, recovery) plays out inside one request; the
// route limiter is loose enough that the demo UI stays usable but a
// curl loop trips the real 429 within one line of shell.
var (
	// StoryRatePolicy drives the security:lab story's own limiter: a
	// fresh limiter per story run, so every request observes the same
	// deterministic window.
	StoryRatePolicy = domsecurity.RatePolicy{Rate: 10, Burst: 5}
	// RouteRatePolicy wraps the demo routes themselves in the
	// composition root — the limiter whose 429 the terminal tour trips.
	RouteRatePolicy = domsecurity.RatePolicy{Rate: 2, Burst: 5}
)

// Console is the topic's substrate, wired once at the composition
// root: the header policy the middleware applies and the route
// limiter that wraps the demo routes. The two STORIES build their
// remaining state fresh per run — the lab story mints its own
// limiter (so the window is deterministic request over request) and
// the PII story its own subject store (erasure is destructive, and
// the next request must find the fixture records intact).
type Console struct {
	Headers      []domsecurity.HeaderRule
	RouteLimiter *domsecurity.ClientRateLimiter
}

// NewConsole wires the substrate. Construction can only fail on a
// broken limiter policy, which is a compile-time-adjacent constant
// here — the error shape is kept so the composition root treats every
// console the same way.
func NewConsole() (*Console, error) {
	routeLimiter, err := domsecurity.NewClientRateLimiter(RouteRatePolicy)
	if err != nil {
		return nil, fmt.Errorf("security console route limiter: %w", err)
	}
	return &Console{
		Headers:      domsecurity.APISecurityHeaders,
		RouteLimiter: routeLimiter,
	}, nil
}

// newStoryLimiter mints the lab story's own deterministic limiter.
func (c *Console) newStoryLimiter() (*domsecurity.ClientRateLimiter, error) {
	limiter, err := domsecurity.NewClientRateLimiter(StoryRatePolicy)
	if err != nil {
		return nil, fmt.Errorf("story limiter: %w", err)
	}
	return limiter, nil
}

// seedSubjects returns the three fixture data subjects. The
// classifications are the teaching: display names are public, plan
// tiers are internal, contact details are PII — and so is the signup
// IP, the one most teams misfile.
func seedSubjects() []domsecurity.SubjectRecord {
	return []domsecurity.SubjectRecord{
		{
			SubjectID: "sub_ada",
			Fields: []domsecurity.ClassifiedField{
				{Name: "display_name", Sensitivity: domsecurity.SensitivityPublic, Value: "Ada"},
				{Name: "plan", Sensitivity: domsecurity.SensitivityInternal, Value: "pro"},
				{Name: "email", Sensitivity: domsecurity.SensitivityPII, Value: "ada@example.com"},
				{Name: "phone", Sensitivity: domsecurity.SensitivityPII, Value: "+1-555-010-2233"},
				{Name: "signup_ip", Sensitivity: domsecurity.SensitivityPII, Value: "198.51.100.7"},
			},
		},
		{
			SubjectID: "sub_linus",
			Fields: []domsecurity.ClassifiedField{
				{Name: "display_name", Sensitivity: domsecurity.SensitivityPublic, Value: "Linus"},
				{Name: "plan", Sensitivity: domsecurity.SensitivityInternal, Value: "team"},
				{Name: "email", Sensitivity: domsecurity.SensitivityPII, Value: "linus@example.com"},
			},
		},
		{
			SubjectID: "sub_grace",
			Fields: []domsecurity.ClassifiedField{
				{Name: "display_name", Sensitivity: domsecurity.SensitivityPublic, Value: "Grace"},
				{Name: "plan", Sensitivity: domsecurity.SensitivityInternal, Value: "free"},
				{Name: "email", Sensitivity: domsecurity.SensitivityPII, Value: "grace@example.com"},
				{Name: "phone", Sensitivity: domsecurity.SensitivityPII, Value: "+44-20-7946-0958"},
			},
		},
	}
}

// memorySubjectStore is the in-memory SubjectStore adapter: a map of
// records, seeded per story run. Production swaps this for the
// subjects table behind pgx — the port and the erasure routine stay.
type memorySubjectStore struct {
	records map[string]domsecurity.SubjectRecord
}

// newMemorySubjectStore seeds the store from fixture records.
func newMemorySubjectStore(seeds []domsecurity.SubjectRecord) *memorySubjectStore {
	records := make(map[string]domsecurity.SubjectRecord, len(seeds))
	for _, record := range seeds {
		records[record.SubjectID] = record
	}
	return &memorySubjectStore{records: records}
}

func (s *memorySubjectStore) FindSubject(_ context.Context, subjectID string) (domsecurity.SubjectRecord, bool, error) {
	record, ok := s.records[subjectID]
	return record, ok, nil
}

func (s *memorySubjectStore) SaveSubject(_ context.Context, record domsecurity.SubjectRecord) error {
	s.records[record.SubjectID] = record
	return nil
}

// memoryAccessLogStore is the in-memory AccessLogStore adapter.
type memoryAccessLogStore struct {
	entries []domsecurity.AccessLogEntry
}

func (s *memoryAccessLogStore) Append(_ context.Context, entry domsecurity.AccessLogEntry) error {
	s.entries = append(s.entries, entry)
	return nil
}

func (s *memoryAccessLogStore) List(_ context.Context) ([]domsecurity.AccessLogEntry, error) {
	return append([]domsecurity.AccessLogEntry(nil), s.entries...), nil
}
